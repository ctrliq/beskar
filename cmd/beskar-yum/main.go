// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
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
	"go.ciq.dev/beskar/internal/plugins/yum"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/config"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumrepository"
	"go.ciq.dev/beskar/pkg/sighandler"
	"go.ciq.dev/beskar/pkg/version"
)

var configDir string

func serve(beskarYumCmd *flag.FlagSet) error {
	if err := beskarYumCmd.Parse(os.Args[1:]); err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM, syscall.SIGINT)

	beskarYumConfig, err := config.ParseBeskarYumConfig(configDir)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", beskarYumConfig.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	plugin, err := yum.New(ctx, beskarYumConfig)
	if err != nil {
		return err
	}

	go func() {
		errCh <- pluginsrv.Serve[*yumrepository.Handler](ln, plugin)
	}()

	return wait(false)
}

func main() {
	beskarYumCmd := flag.NewFlagSet("beskar-yum", flag.ExitOnError)
	beskarYumCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	subCommand := ""
	if len(os.Args) > 1 {
		subCommand = os.Args[1]
	}

	switch subCommand {
	case "version":
		fmt.Println(version.Semver)
	default:
		if err := serve(beskarYumCmd); err != nil {
			log.Fatal(err)
		}
	}
}
