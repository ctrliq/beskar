// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cavaliergopher/rpm"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
)

var Version = "dev"

func fatal(format string, a ...any) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}

func main() {
	pushCmd := flag.NewFlagSet("foo", flag.ExitOnError)
	pushRepo := pushCmd.String("repo", "", "repo")
	pushRegistry := pushCmd.String("registry", "", "registry")

	if len(os.Args) == 1 {
		fatal("missing subcommand")
	}

	switch os.Args[1] {
	case "version":
		fmt.Println(Version)
	case "push":
		if err := pushCmd.Parse(os.Args[2:]); err != nil {
			fatal("while parsing command arguments: %w", err)
		}
		rpm := pushCmd.Arg(0)
		if rpm == "" {
			fatal("an RPM package must be specified")
		} else if pushRegistry == nil || *pushRegistry == "" {
			fatal("a registry must be specified")
		} else if pushRepo == nil || *pushRepo == "" {
			fatal("a repo must be specified")
		}
		if err := push(rpm, *pushRepo, *pushRegistry); err != nil {
			fatal("while pushing RPM package: %s", err)
		}
	default:
		fatal("unknown %q subcommand", os.Args[1])
	}
}

func push(rpmPath string, repo, registry string) error {
	rpmFile, err := os.Open(rpmPath)
	if err != nil {
		return fmt.Errorf("while opening %s: %w", rpmPath, err)
	}
	defer rpmFile.Close()

	pkg, err := rpm.Read(rpmFile)
	if err != nil {
		return fmt.Errorf("while reading %s metadata: %w", rpmPath, err)
	}

	archTag := pkg.Header.GetTag(1022)
	arch := ""
	if archTag == nil {
		arch = pkg.Architecture()
	} else {
		arch = archTag.String()
	}

	rpmName := fmt.Sprintf("%s-%s-%s.%s.rpm", pkg.Name(), pkg.Version(), pkg.Release(), arch)

	pkgTag := fmt.Sprintf("%x", sha256.Sum256([]byte(rpmName)))

	rawRef := filepath.Join(registry, "yum", repo, "packages:"+pkgTag)
	ref, err := name.ParseReference(rawRef)
	if err != nil {
		return fmt.Errorf("while parsing reference %s: %w", rawRef, err)
	}

	pusher := orasrpm.NewRPMPusher(
		ref,
		rpmPath,
		orasrpm.WithRPMLayerPlatform(
			&v1.Platform{
				Architecture: arch,
				OS:           "linux",
			},
		),
		orasrpm.WithRPMLayerAnnotations(map[string]string{
			imagespec.AnnotationTitle: rpmName,
		}),
	)

	fmt.Printf("Pushing %s to %s\n", rpmName, rawRef)

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
