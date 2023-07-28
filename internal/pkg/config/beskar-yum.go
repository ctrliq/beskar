package config

import (
	_ "embed"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	Driver string                `yaml:"driver"`
	Prefix string                `yaml:"prefix"`
	S3     BeskarYumS3Storage    `yaml:"s3"`
	FS     BeskarYumFSStorage    `yaml:"filesystem"`
	GCS    BeskarYumGCSStorage   `yaml:"gcs"`
	Azure  BeskarYumAzureStorage `yaml:"azure"`
}

func (b BeskarYumStorage) IsS3() bool {
	return b.Driver == S3StorageDriver
}

func (b BeskarYumStorage) IsFS() bool {
	return b.Driver == FSStorageDriver
}

type BeskarYumConfig struct {
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

	beskarYumConfig := new(BeskarYumConfig)
	if err := yaml.NewDecoder(configReader).Decode(beskarYumConfig); err != nil {
		return nil, err
	}

	beskarYumConfig.ConfigDirectory = configDir

	return beskarYumConfig, nil
}
