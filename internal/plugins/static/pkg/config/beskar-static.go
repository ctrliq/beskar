// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/distribution/distribution/v3/configuration"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/pkg/log"
	"go.ciq.dev/beskar/internal/pkg/storage"
)

const (
	BeskarStaticConfigFile     = "beskar-static.yaml"
	DefaultBeskarStaticDataDir = "/tmp/beskar-static"
)

//go:embed default/beskar-static.yaml
var defaultBeskarStaticConfig string

type BeskarStaticConfig struct {
	Version         string         `yaml:"version"`
	Log             log.Config     `yaml:"log"`
	Addr            string         `yaml:"addr"`
	Gossip          gossip.Config  `yaml:"gossip"`
	Storage         storage.Config `yaml:"storage"`
	Profiling       bool           `yaml:"profiling"`
	DataDir         string         `yaml:"datadir"`
	ConfigDirectory string         `yaml:"-"`
}

func (bc BeskarStaticConfig) ListenIP() (string, error) {
	host, _, err := net.SplitHostPort(bc.Addr)
	if err != nil {
		return "", err
	}
	return host, nil
}

func (bc BeskarStaticConfig) ListenPort() (string, error) {
	_, port, err := net.SplitHostPort(bc.Addr)
	if err != nil {
		return "", err
	}
	return port, nil
}

type BeskarStaticConfigV1 BeskarStaticConfig

func ParseBeskarStaticConfig(dir string) (*BeskarStaticConfig, error) {
	customDir := false
	filename := filepath.Join(config.DefaultConfigDir, BeskarStaticConfigFile)
	if dir != "" {
		filename = filepath.Join(dir, BeskarStaticConfigFile)
		customDir = true
	}

	configDir := filepath.Dir(filename)

	var configReader io.Reader

	f, err := os.Open(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) || customDir {
			return nil, err
		}
		configReader = strings.NewReader(defaultBeskarStaticConfig)
		configDir = ""
	} else {
		defer f.Close()
		configReader = f
	}

	configBuffer := new(bytes.Buffer)
	if _, err := io.Copy(configBuffer, configReader); err != nil {
		return nil, err
	}

	configParser := configuration.NewParser("beskarstatic", []configuration.VersionedParseInfo{
		{
			Version: configuration.MajorMinorVersion(1, 0),
			ParseAs: reflect.TypeOf(BeskarStaticConfigV1{}),
			ConversionFunc: func(c interface{}) (interface{}, error) {
				if v1, ok := c.(*BeskarStaticConfigV1); ok {
					v1.ConfigDirectory = configDir
					return (*BeskarStaticConfig)(v1), nil
				}
				return nil, fmt.Errorf("expected *BeskarStaticConfigV1, received %#v", c)
			},
		},
	})

	beskarStaticConfig := new(BeskarStaticConfig)

	if err := configParser.Parse(configBuffer.Bytes(), beskarStaticConfig); err != nil {
		return nil, err
	}

	return beskarStaticConfig, nil
}
