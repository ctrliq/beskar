package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cavaliergopher/rpm"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/orasrpm"
	"go.ciq.dev/beskar/pkg/oras"
)

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

	_, err = rpmFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("while resetting RPM file descriptor cursor: %w", err)
	}

	sum := sha256.New()
	if _, err = io.Copy(sum, rpmFile); err != nil {
		return fmt.Errorf("while computing RPM sha256 checksum: %w", err)
	}
	pkgid := fmt.Sprintf("%x", sum.Sum(nil))

	rawRef := filepath.Join(registry, "yum", repo, "packages:"+pkgid)
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
	)

	fmt.Printf("Pushing %s to %s\n", rpmFile.Name(), rawRef)

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
