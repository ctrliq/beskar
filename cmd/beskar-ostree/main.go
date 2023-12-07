package main

import (
	"flag"
	"fmt"
	"go.ciq.dev/beskar/internal/pkg/pluginsrv"
	"go.ciq.dev/beskar/internal/plugins/ostree"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/config"
	"go.ciq.dev/beskar/pkg/sighandler"
	"go.ciq.dev/beskar/pkg/version"
	"log"
	"net"
	"os"
	"syscall"
)

var configDir string

func serve(beskarOSTreeCmd *flag.FlagSet) error {
	if err := beskarOSTreeCmd.Parse(os.Args[1:]); err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM, syscall.SIGINT)

	beskarOSTreeConfig, err := config.ParseBeskarOSTreeConfig(configDir)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", beskarOSTreeConfig.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	plugin, err := ostree.New(ctx, beskarOSTreeConfig)
	if err != nil {
		return err
	}

	go func() {
		errCh <- pluginsrv.Serve(ln, plugin)
	}()

	return wait(false)
}

func main() {
	beskarOSTreeCmd := flag.NewFlagSet("beskar-ostree", flag.ExitOnError)
	beskarOSTreeCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	subCommand := ""
	if len(os.Args) > 1 {
		subCommand = os.Args[1]
	}

	switch subCommand {
	case "version":
		fmt.Println(version.Semver)
	default:
		if err := serve(beskarOSTreeCmd); err != nil {
			log.Fatal(err)
		}
	}
}
