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

var configDir string

func serve(beskarCmd *flag.FlagSet) error {
	if err := beskarCmd.Parse(os.Args[1:]); err != nil {
		return err
	}

	beskarConfig, err := config.ParseBeskarConfig(configDir)
	if err != nil {
		return err
	}

	ctx, beskarRegistry, err := beskar.New(beskarConfig)
	if err != nil {
		return err
	}

	return beskarRegistry.Serve(ctx)
}

func gc(beskarGCCmd *flag.FlagSet) error {
	var (
		removeUntagged bool
		dryRun         bool
	)

	beskarGCCmd.BoolVar(&removeUntagged, "delete-untagged", false, "delete manifests that are not currently referenced via tag")
	beskarGCCmd.BoolVar(&dryRun, "dry-run", false, "do everything except remove the blobs")

	if err := beskarGCCmd.Parse(os.Args[2:]); err != nil {
		return err
	}

	beskarConfig, err := config.ParseBeskarConfig(configDir)
	if err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM)

	go func() {
		errCh <- beskar.RunGC(ctx, beskarConfig, dryRun, removeUntagged)
	}()

	return wait()
}

func main() {
	beskarCmd := flag.NewFlagSet("beskar", flag.ExitOnError)
	beskarCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	beskarGCCmd := flag.NewFlagSet("beskar-gc", flag.ExitOnError)
	beskarGCCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	subCommand := ""
	if len(os.Args) > 1 {
		subCommand = os.Args[1]
	}

	switch subCommand {
	case "gc":
		if err := gc(beskarCmd); err != nil {
			log.Fatal(err)
		}
	default:
		if err := serve(beskarGCCmd); err != nil {
			log.Fatal(err)
		}
	}
}
