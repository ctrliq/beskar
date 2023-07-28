package config

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"go.ciq.dev/beskar/pkg/mtls"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir = "/etc/beskar"
	BeskarConfigFile = "beskar.yaml"
)

//go:embed default/beskar.yaml
var defaultBeskarConfig string

type Cache struct {
	Addr string `yaml:"addr"`
	Size uint32 `yaml:"size"`
}

type Gossip struct {
	Addr  string   `yaml:"addr"`
	Key   string   `yaml:"key"`
	Peers []string `yaml:"peers"`
	CA    string   `yaml:"ca-cert"`
	CAKey string   `yaml:"ca-key"`
}

func (g Gossip) LoadCACertificate() (tls.Certificate, error) {
	caCertReader, caKeyReader, err := g.getCAReader()
	if err != nil {
		return tls.Certificate{}, err
	}
	return mtls.LoadCACertificate(caCertReader, caKeyReader)
}

func (g Gossip) LoadCAPem() (*mtls.CAPEM, error) {
	caCertReader, caKeyReader, err := g.getCAReader()
	if err != nil {
		return nil, err
	}

	caPem := &mtls.CAPEM{}

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, caCertReader)
	if err != nil {
		return nil, err
	}
	caPem.Cert = buf.Bytes()

	buf = new(bytes.Buffer)
	_, err = io.Copy(buf, caKeyReader)
	if err != nil {
		return nil, err
	}
	caPem.Key = buf.Bytes()

	return caPem, nil
}

func (g Gossip) getCAReader() (io.Reader, io.Reader, error) {
	var (
		caCertReader io.Reader
		caKeyReader  io.Reader
	)

	if g.CA == "" {
		return nil, nil, fmt.Errorf("no CA certificate found")
	} else if g.CAKey == "" {
		return nil, nil, fmt.Errorf("no CA certificate key found")
	}

	if g.CA[0] == '/' {
		f, err := os.Open(g.CA)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()
		caCertReader = f
	} else {
		caCertReader = strings.NewReader(g.CA)
	}

	if g.CAKey[0] == '/' {
		f, err := os.Open(g.CAKey)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()
		caKeyReader = f
	} else {
		caKeyReader = strings.NewReader(g.CAKey)
	}

	return caCertReader, caKeyReader, nil
}

type PluginMTLS struct {
	Enabled bool   `yaml:"enabled"`
	CA      string `yaml:"ca-cert"`
	CAKey   string `yaml:"ca-key"`
}

type PluginBackend struct {
	URL  string     `yaml:"url"`
	MTLS PluginMTLS `yaml:"mtls"`
}

type Plugin struct {
	Prefix    string          `yaml:"prefix"`
	Mediatype string          `yaml:"mediatype"`
	Backends  []PluginBackend `yaml:"backends"`
}

type Registry struct {
	*configuration.Configuration `yaml:",inline"`
}

func (r *Registry) UnmarshalYAML(value *yaml.Node) error {
	v := make(map[string]interface{})
	if err := value.Decode(v); err != nil {
		return err
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	r.Configuration, err = configuration.Parse(bytes.NewReader(out))
	return err
}

type BeskarConfig struct {
	Profiling bool              `yaml:"profiling"`
	Cache     Cache             `yaml:"cache"`
	Gossip    Gossip            `yaml:"gossip"`
	Plugins   map[string]Plugin `yaml:"plugins"`
	Registry  Registry          `yaml:"registry"`
}

func (bc *BeskarConfig) RunInKubernetes() bool {
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

func ParseBeskarConfig(dir string) (*BeskarConfig, error) {
	inMemoryConfig := false
	customDir := false
	filename := filepath.Join(DefaultConfigDir, BeskarConfigFile)
	if dir != "" {
		filename = filepath.Join(dir, BeskarConfigFile)
		customDir = true
	}

	var configReader io.Reader

	f, err := os.Open(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) || customDir {
			return nil, err
		}
		configReader = strings.NewReader(defaultBeskarConfig)
		inMemoryConfig = true
	} else {
		defer f.Close()
		configReader = f
	}

	beskarConfig := new(BeskarConfig)
	if err := yaml.NewDecoder(configReader).Decode(beskarConfig); err != nil {
		return nil, err
	}

	if beskarConfig.Registry.Configuration == nil {
		return nil, fmt.Errorf("registry configuration is missing")
	}

	storage := beskarConfig.Registry.Configuration.Storage

	if inMemoryConfig && storage.Type() == "filesystem" {
		params := storage.Parameters()
		params["rootdirectory"] = "/tmp/beskar-registry"
	}

	runInKubernetes := beskarConfig.RunInKubernetes()

	if beskarConfig.Gossip.Key == "" {
		return nil, fmt.Errorf("gossip key is missing")
	} else if beskarConfig.Gossip.CA == "" && (len(beskarConfig.Gossip.Peers) > 0 || runInKubernetes) {
		return nil, fmt.Errorf("gossip ca-cert is empty")
	} else if beskarConfig.Gossip.CAKey == "" && (len(beskarConfig.Gossip.Peers) > 0 || runInKubernetes) {
		return nil, fmt.Errorf("gossip ca-key is empty")
	}

	if beskarConfig.Gossip.CA == "" || beskarConfig.Gossip.CAKey == "" {
		caCert, caKey, err := mtls.GenerateCA("beskar", time.Now().AddDate(10, 0, 0), mtls.ECDSAKey)
		if err != nil {
			return nil, err
		}
		beskarConfig.Gossip.CA = string(caCert)
		beskarConfig.Gossip.CAKey = string(caKey)
	}

	return beskarConfig, nil
}
