// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"go.ciq.dev/beskar/internal/plugins/static/pkg/staticrepository"
	"log"
	"net"
	"os"
	"syscall"

	"go.ciq.dev/beskar/internal/pkg/pluginsrv"
	"go.ciq.dev/beskar/internal/plugins/static"
	"go.ciq.dev/beskar/internal/plugins/static/pkg/config"
	"go.ciq.dev/beskar/pkg/sighandler"
	"go.ciq.dev/beskar/pkg/version"
)

var configDir string

func serve(beskarStaticCmd *flag.FlagSet) error {
	if err := beskarStaticCmd.Parse(os.Args[1:]); err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM, syscall.SIGINT)

	beskarStaticConfig, err := config.ParseBeskarStaticConfig(configDir)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", beskarStaticConfig.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	plugin, err := static.New(ctx, beskarStaticConfig)
	if err != nil {
		return err
	}

	go func() {
		errCh <- pluginsrv.Serve[*staticrepository.Handler](ln, plugin)
	}()

	return wait(false)
}

func main() {
	beskarStaticCmd := flag.NewFlagSet("beskar-static", flag.ExitOnError)
	beskarStaticCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	subCommand := ""
	if len(os.Args) > 1 {
		subCommand = os.Args[1]
	}

	switch subCommand {
	case "version":
		fmt.Println(version.Semver)
	default:
		if err := serve(beskarStaticCmd); err != nil {
			log.Fatal(err)
		}
	}
}
