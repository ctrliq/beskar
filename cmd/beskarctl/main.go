// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"go.ciq.dev/beskar/pkg/version"
)

func fatal(format string, a ...any) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}

func main() {
	pushCmd := flag.NewFlagSet("push", flag.ExitOnError)
	pushRepo := pushCmd.String("repo", "", "repo")
	pushRegistry := pushCmd.String("registry", "", "registry")

	pushMetadataCmd := flag.NewFlagSet("push-metadata", flag.ExitOnError)
	pushMetadataRepo := pushMetadataCmd.String("repo", "", "repo")
	pushMetadataRegistry := pushMetadataCmd.String("registry", "", "registry")
	pushMetadataType := pushMetadataCmd.String("type", "", "type")

	if len(os.Args) == 1 {
		fatal("missing subcommand")
	}

	switch os.Args[1] {
	case "version":
		fmt.Println(version.Semver)
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
	case "push-metadata":
		if err := pushMetadataCmd.Parse(os.Args[2:]); err != nil {
			fatal("while parsing command arguments: %w", err)
		}
		metadata := pushMetadataCmd.Arg(0)
		if metadata == "" {
			fatal("a metadata file must be specified")
		} else if pushMetadataRegistry == nil || *pushMetadataRegistry == "" {
			fatal("a registry must be specified")
		} else if pushMetadataRepo == nil || *pushMetadataRepo == "" {
			fatal("a repo must be specified")
		} else if pushMetadataType == nil || *pushMetadataType == "" {
			fatal("a metadata type must be specified")
		}
		if err := pushMetadata(metadata, *pushMetadataType, *pushMetadataRepo, *pushMetadataRegistry); err != nil {
			fatal("while pushing metadata: %s", err)
		}
	default:
		fatal("unknown %q subcommand", os.Args[1])
	}
}

func push(rpmPath string, repo, registry string) error {
	pusher, err := orasrpm.NewRPMPusher(rpmPath, repo, name.WithDefaultRegistry(registry))
	if err != nil {
		return fmt.Errorf("while creating RPM pusher: %w", err)
	}

	fmt.Printf("Pushing %s to %s\n", rpmPath, pusher.Reference())

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

func pushMetadata(metadataPath string, dataType, repo, registry string) error {
	pusher, err := orasrpm.NewRPMExtraMetadataPusher(metadataPath, repo, dataType, name.WithDefaultRegistry(registry))
	if err != nil {
		return fmt.Errorf("while creating RPM metadata pusher: %w", err)
	}

	fmt.Printf("Pushing %s to %s\n", metadataPath, pusher.Reference())

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
