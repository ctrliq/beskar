// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
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
	BeskarMirrorConfigFile     = "beskar-mirror.yaml"
	DefaultBeskarMirrorDataDir = "/tmp/beskar-mirror"
)

//go:embed default/beskar-mirror.yaml
var defaultBeskarMirrorConfig string

type BeskarMirrorConfig struct {
	Version         string         `yaml:"version"`
	Log             log.Config     `yaml:"log"`
	Addr            string         `yaml:"addr"`
	Gossip          gossip.Config  `yaml:"gossip"`
	Storage         storage.Config `yaml:"storage"`
	Profiling       bool           `yaml:"profiling"`
	DataDir         string         `yaml:"datadir"`
	ConfigDirectory string         `yaml:"-"`
}

func (bc BeskarMirrorConfig) ListenIP() (string, error) {
	host, _, err := net.SplitHostPort(bc.Addr)
	if err != nil {
		return "", err
	}
	return host, nil
}

func (bc BeskarMirrorConfig) ListenPort() (string, error) {
	_, port, err := net.SplitHostPort(bc.Addr)
	if err != nil {
		return "", err
	}
	return port, nil
}

type BeskarMirrorConfigV1 BeskarMirrorConfig

func ParseBeskarMirrorConfig(dir string) (*BeskarMirrorConfig, error) {
	customDir := false
	filename := filepath.Join(config.DefaultConfigDir, BeskarMirrorConfigFile)
	if dir != "" {
		filename = filepath.Join(dir, BeskarMirrorConfigFile)
		customDir = true
	}

	configDir := filepath.Dir(filename)

	var configReader io.Reader

	f, err := os.Open(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) || customDir {
			return nil, err
		}
		configReader = strings.NewReader(defaultBeskarMirrorConfig)
		configDir = ""
	} else {
		defer f.Close()
		configReader = f
	}

	configBuffer := new(bytes.Buffer)
	if _, err := io.Copy(configBuffer, configReader); err != nil {
		return nil, err
	}

	configParser := configuration.NewParser("beskarmirror", []configuration.VersionedParseInfo{
		{
			Version: configuration.MajorMinorVersion(1, 0),
			ParseAs: reflect.TypeOf(BeskarMirrorConfigV1{}),
			ConversionFunc: func(c interface{}) (interface{}, error) {
				if v1, ok := c.(*BeskarMirrorConfigV1); ok {
					v1.ConfigDirectory = configDir
					return (*BeskarMirrorConfig)(v1), nil
				}
				return nil, fmt.Errorf("expected *BeskarMirrorConfigV1, received %#v", c)
			},
		},
	})

	beskarMirrorConfig := new(BeskarMirrorConfig)

	if err := configParser.Parse(configBuffer.Bytes(), beskarMirrorConfig); err != nil {
		return nil, err
	}

	return beskarMirrorConfig, nil
}
