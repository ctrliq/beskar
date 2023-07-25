// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"

	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/yumplugin"
	"go.ciq.dev/beskar/pkg/sighandler"
)

var Version = "dev"

var configDir string

func serve(beskarYumCmd *flag.FlagSet) error {
	if err := beskarYumCmd.Parse(os.Args[1:]); err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM)

	beskarYumConfig, err := config.ParseBeskarYumConfig(configDir)
	if err != nil {
		return err
	}

	yp, err := yumplugin.New(ctx, beskarYumConfig, true)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", beskarYumConfig.Addr)
	if err != nil {
		return err
	}

	go func() {
		errCh <- yp.Serve(ln)
	}()

	return wait()
}

func addPackage(beskarYumAddPkgCmd *flag.FlagSet) error {
	var (
		packageDir        string
		packageID         string
		packageRepository string
		keepDatabaseDir   bool
	)

	beskarYumAddPkgCmd.StringVar(&packageDir, "dir", "", "package directory")
	beskarYumAddPkgCmd.StringVar(&packageID, "id", "", "package identifier")
	beskarYumAddPkgCmd.StringVar(&packageRepository, "repository", "", "package repository")
	beskarYumAddPkgCmd.BoolVar(&keepDatabaseDir, "keep-db-dir", false, "keep database temporary directory")

	if err := beskarYumAddPkgCmd.Parse(os.Args[2:]); err != nil {
		return err
	}

	beskarYumConfig, err := config.ParseBeskarYumConfig(configDir)
	if err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM)

	yp, err := yumplugin.New(ctx, beskarYumConfig, false)
	if err != nil {
		return err
	}

	go func() {
		dbDir, err := yp.AddPackageToDatabase(ctx, packageID, packageRepository, packageDir, false, keepDatabaseDir)
		if err == nil {
			fmt.Fprintf(os.Stdout, "%s\n", dbDir)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
		errCh <- err
	}()

	return wait()
}

func genMetadata(beskarYumGenMetaCmd *flag.FlagSet) error {
	var (
		dbDir      string
		repository string
	)

	beskarYumGenMetaCmd.StringVar(&dbDir, "db-dir", "", "database directory")
	beskarYumGenMetaCmd.StringVar(&repository, "repository", "", "package repository")

	if err := beskarYumGenMetaCmd.Parse(os.Args[2:]); err != nil {
		return err
	}

	beskarYumConfig, err := config.ParseBeskarYumConfig(configDir)
	if err != nil {
		return err
	}

	errCh := make(chan error)

	ctx, wait := sighandler.New(errCh, syscall.SIGTERM)

	yp, err := yumplugin.New(ctx, beskarYumConfig, false)
	if err != nil {
		return err
	}

	go func() {
		err := yp.GenerateAndSaveMetadata(ctx, repository, dbDir, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		errCh <- err
	}()

	return wait()
}

func main() {
	beskarYumCmd := flag.NewFlagSet("beskar-yum", flag.ExitOnError)
	beskarYumCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	beskarYumAddPkgCmd := flag.NewFlagSet("beskar-yum-add-pkg", flag.ExitOnError)
	beskarYumAddPkgCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	beskarYumGenMetaCmd := flag.NewFlagSet("beskar-yum-gen-meta", flag.ExitOnError)
	beskarYumGenMetaCmd.StringVar(&configDir, "config-dir", "", "configuration directory")

	subCommand := ""
	if len(os.Args) > 1 {
		subCommand = os.Args[1]
	}

	switch subCommand {
	case "add-pkg":
		if err := addPackage(beskarYumAddPkgCmd); err != nil {
			log.Fatal(err)
		}
	case "gen-metadata":
		if err := genMetadata(beskarYumGenMetaCmd); err != nil {
			log.Fatal(err)
		}
	case "version":
		fmt.Println(Version)
	default:
		if err := serve(beskarYumCmd); err != nil {
			log.Fatal(err)
		}
	}
}
