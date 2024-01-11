package repoconfig

import "gopkg.in/yaml.v3"

type File struct {
	SHA256 string `yaml:"sha256"`
	Size   uint64 `yaml:"size"`
}

type Repository struct {
	URL           string          `yaml:"url"`
	LocalPath     string          `yaml:"local-path"`
	MirrorURL     string          `yaml:"mirror-url"`
	AuthMirrorURL string          `yaml:"auth-mirror-url"`
	GPGKey        string          `yaml:"gpgkey"`
	Files         map[string]File `yaml:"files"`
}

type Repositories map[string]Repository

func Parse(data []byte) (Repositories, error) {
	src := make(Repositories)
	err := yaml.Unmarshal(data, src)
	return src, err
}
