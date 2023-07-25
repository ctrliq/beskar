// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
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
)

const (
	BeskarYumConfigFile     = "beskar-yum.yaml"
	DefaultBeskarYumDataDir = "/tmp/beskar-yum"

	FSStorageDriver    = "filesystem"
	S3StorageDriver    = "s3"
	GCSStorageDriver   = "gcs"
	AzureStorageDriver = "azure"
)

//go:embed default/beskar-yum.yaml
var defaultBeskarYumConfig string

type BeskarYumRegistry struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type BeskarYumS3Storage struct {
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	AccessKeyID     string `yaml:"access-key-id"`
	SecretAccessKey string `yaml:"secret-access-key"`
	SessionToken    string `yaml:"session-token"`
	Region          string `yaml:"region"`
	DisableSSL      bool   `yaml:"disable-ssl"`
}

type BeskarYumFSStorage struct {
	Directory string `yaml:"directory"`
}

type BeskarYumGCSStorage struct {
	Bucket  string `yaml:"bucket"`
	Keyfile string `yaml:"keyfile"`
}

type BeskarYumAzureStorage struct {
	Container   string `yaml:"container"`
	AccountName string `yaml:"account-name"`
	AccountKey  string `yaml:"account-key"`
}

type BeskarYumStorage struct {
	Driver     string                `yaml:"driver"`
	Prefix     string                `yaml:"prefix"`
	S3         BeskarYumS3Storage    `yaml:"s3"`
	Filesystem BeskarYumFSStorage    `yaml:"filesystem"`
	GCS        BeskarYumGCSStorage   `yaml:"gcs"`
	Azure      BeskarYumAzureStorage `yaml:"azure"`
}

type BeskarYumConfig struct {
	Version         string            `yaml:"version"`
	Addr            string            `yaml:"addr"`
	Registry        BeskarYumRegistry `yaml:"registry"`
	Storage         BeskarYumStorage  `yaml:"storage"`
	Profiling       bool              `yaml:"profiling"`
	DataDir         string            `yaml:"datadir"`
	ConfigDirectory string            `yaml:"-"`
}

func (bc BeskarYumConfig) ListenIP() (string, error) {
	host, _, err := net.SplitHostPort(bc.Addr)
	if err != nil {
		return "", err
	}
	return host, nil
}

func (bc BeskarYumConfig) ListenPort() (string, error) {
	_, port, err := net.SplitHostPort(bc.Addr)
	if err != nil {
		return "", err
	}
	return port, nil
}

type BeskarYumConfigV1 BeskarYumConfig

func ParseBeskarYumConfig(dir string) (*BeskarYumConfig, error) {
	customDir := false
	filename := filepath.Join(DefaultConfigDir, BeskarYumConfigFile)
	if dir != "" {
		filename = filepath.Join(dir, BeskarYumConfigFile)
		customDir = true
	}

	configDir := filepath.Dir(filename)

	var configReader io.Reader

	f, err := os.Open(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) || customDir {
			return nil, err
		}
		configReader = strings.NewReader(defaultBeskarYumConfig)
		configDir = ""
	} else {
		defer f.Close()
		configReader = f
	}

	configBuffer := new(bytes.Buffer)
	if _, err := io.Copy(configBuffer, configReader); err != nil {
		return nil, err
	}

	configParser := configuration.NewParser("beskaryum", []configuration.VersionedParseInfo{
		{
			Version: configuration.MajorMinorVersion(1, 0),
			ParseAs: reflect.TypeOf(BeskarYumConfigV1{}),
			ConversionFunc: func(c interface{}) (interface{}, error) {
				if v1, ok := c.(*BeskarYumConfigV1); ok {
					v1.ConfigDirectory = configDir
					return (*BeskarYumConfig)(v1), nil
				}
				return nil, fmt.Errorf("expected *BeskarConfigV1, received %#v", c)
			},
		},
	})

	beskarYumConfig := new(BeskarYumConfig)

	if err := configParser.Parse(configBuffer.Bytes(), beskarYumConfig); err != nil {
		return nil, err
	}

	return beskarYumConfig, nil
}
