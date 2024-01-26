// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"

	"go.ciq.dev/beskar/internal/pkg/pluginsrv"
	"go.ciq.dev/beskar/internal/plugins/mirror"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/config"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrorrepository"
	"go.ciq.dev/beskar/pkg/sighandler"
	"go.ciq.dev/beskar/pkg/version"
)

var configDir string

func serve(beskarMirrorCmd *flag.FlagSet) error {
	if err := beskarMirrorCmd.Parse(os.Args[1:]); err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM, syscall.SIGINT)

	beskarMirrorConfig, err := config.ParseBeskarMirrorConfig(configDir)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", beskarMirrorConfig.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	plugin, err := mirror.New(ctx, beskarMirrorConfig)
	if err != nil {
		return err
	}

	go func() {
		errCh <- pluginsrv.Serve[*mirrorrepository.Handler](ln, plugin)
	}()

	return wait(false)
}

func main() {
	beskarMirrorCmd := flag.NewFlagSet("beskar-mirror", flag.ExitOnError)
	beskarMirrorCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	subCommand := ""
	if len(os.Args) > 1 {
		subCommand = os.Args[1]
	}

	switch subCommand {
	case "version":
		fmt.Println(version.Semver)
	default:
		if err := serve(beskarMirrorCmd); err != nil {
			log.Fatal(err)
		}
	}
}
