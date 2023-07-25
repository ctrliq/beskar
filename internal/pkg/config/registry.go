package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/distribution/distribution/v3/configuration"
)

const RegistryConfigFile = "registry.yaml"

//go:embed default/registry.yaml
var RegistryDefaultConfig string

func ParseRegistryConfig(dir string) (*configuration.Configuration, error) {
	customDir := false
	filename := filepath.Join(DefaultConfigDir, RegistryConfigFile)
	if dir != "" {
		filename = filepath.Join(dir, RegistryConfigFile)
		customDir = true
	}

	var configReader io.Reader

	f, err := os.Open(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) || customDir {
			return nil, err
		}
		configReader = strings.NewReader(RegistryDefaultConfig)
	} else {
		defer f.Close()
		configReader = f
	}

	config, err := configuration.Parse(configReader)
	if err != nil {
		return nil, err
	}

	switch config.Storage.Type() {
	case "s3", "filesystem":
	default:
		return nil, fmt.Errorf("storage %s is not supported yet", config.Storage.Type())
	}

	return config, nil
}
