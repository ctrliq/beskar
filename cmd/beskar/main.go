package main

import (
	"flag"
	"log"
	"os"
	"syscall"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/s3-aws"
	"go.ciq.dev/beskar/internal/pkg/beskar"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/pkg/sighandler"
)

func main() {
	beskarCmd := flag.NewFlagSet("beskar", flag.ExitOnError)
	dir := beskarCmd.String("dir", "", "configuration directory")

	if err := beskarCmd.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	configDir := ""
	if dir != nil {
		configDir = *dir
	}

	errCh := make(chan error, 1)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM)

	registryConfig, err := config.ParseRegistryConfig(configDir)
	if err != nil {
		log.Fatal(err)
	}

	beskarConfig, err := config.ParseBeskarConfig(configDir)
	if err != nil {
		log.Fatal(err)
	}

	beskarRegistry, err := beskar.New(ctx, beskarConfig, registryConfig, errCh)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		errCh <- beskarRegistry.Serve()
	}()

	if err := wait(); err != nil {
		log.Fatal(err)
	}
}
