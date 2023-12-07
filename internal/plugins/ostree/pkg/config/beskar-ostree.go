package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"github.com/distribution/distribution/v3/configuration"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/pkg/log"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	BeskarOSTreeConfigFile     = "beskar-ostree.yaml"
	DefaultBeskarOSTreeDataDir = "/tmp/beskar-ostree"
)

//go:embed default/beskar-ostree.yaml
var defaultBeskarOSTreeConfig string

type BeskarOSTreeConfig struct {
	Version         string        `yaml:"version"`
	Log             log.Config    `yaml:"log"`
	Addr            string        `yaml:"addr"`
	Gossip          gossip.Config `yaml:"gossip"`
	Profiling       bool          `yaml:"profiling"`
	DataDir         string        `yaml:"datadir"`
	ConfigDirectory string        `yaml:"-"`
}

type BeskarOSTreeConfigV1 BeskarOSTreeConfig

func ParseBeskarOSTreeConfig(dir string) (*BeskarOSTreeConfig, error) {
	customDir := false
	filename := filepath.Join(config.DefaultConfigDir, BeskarOSTreeConfigFile)
	if dir != "" {
		filename = filepath.Join(dir, BeskarOSTreeConfigFile)
		customDir = true
	}

	configDir := filepath.Dir(filename)

	var configReader io.Reader

	f, err := os.Open(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) || customDir {
			return nil, err
		}
		configReader = strings.NewReader(defaultBeskarOSTreeConfig)
		configDir = ""
	} else {
		defer f.Close()
		configReader = f
	}

	configBuffer := new(bytes.Buffer)
	if _, err := io.Copy(configBuffer, configReader); err != nil {
		return nil, err
	}

	configParser := configuration.NewParser("beskarostree", []configuration.VersionedParseInfo{
		{
			Version: configuration.MajorMinorVersion(1, 0),
			ParseAs: reflect.TypeOf(BeskarOSTreeConfigV1{}),
			ConversionFunc: func(c interface{}) (interface{}, error) {
				if v1, ok := c.(*BeskarOSTreeConfigV1); ok {
					v1.ConfigDirectory = configDir
					return (*BeskarOSTreeConfig)(v1), nil
				}
				return nil, fmt.Errorf("expected *BeskarOSTreeConfigV1, received %#v", c)
			},
		},
	})

	beskarOSTreeConfig := new(BeskarOSTreeConfig)

	if err := configParser.Parse(configBuffer.Bytes(), beskarOSTreeConfig); err != nil {
		return nil, err
	}

	if beskarOSTreeConfig.DataDir == "" {
		beskarOSTreeConfig.DataDir = DefaultBeskarOSTreeDataDir
	}

	return beskarOSTreeConfig, nil
}
