package mage

import (
	"context"
	"fmt"
	"io"
	"sync"

	"dagger.io/dagger"
)

const (
	GoImage           = "golang:1.20.6-alpine"
	GolangCILintImage = "golangci/golangci-lint:v1.53-alpine"
	ProtolintImage    = "yoheimuta/protolint:0.45.0"
	BaseImage         = "alpine:3.17"

	ProtocVersion    = "v23.4"
	ProtocFileFormat = "protoc-23.4-linux-%s.zip"

	ProtocGenGoVersion     = "v1.31.0"
	ProtocGenGoGrpcVersion = "v1.1.0"
)

var onceCalls = map[string]*onceCall{
	"proto": {},
}

type onceCall struct {
	sync.Once
}

func (oc *onceCall) call(fn func() error) (err error) {
	oc.Do(func() {
		err = fn()
	})
	return err
}

func getDaggerClient(ctx context.Context) (*dagger.Client, error) {
	return dagger.Connect(ctx, dagger.WithLogOutput(io.Discard))
}

func getSource(client *dagger.Client) *dagger.Directory {
	return client.Host().Directory(".", dagger.HostDirectoryOpts{
		Exclude: []string{
			"build",
			"README.md",
		},
	})
}

func getProtoSource(client *dagger.Client) *dagger.Directory {
	return client.Host().Directory("api")
}

func goCache(client *dagger.Client) func(dc *dagger.Container) *dagger.Container {
	return func(dc *dagger.Container) *dagger.Container {
		return dc.
			WithMountedCache("/go", client.CacheVolume("go-mod-cache")).
			WithMountedCache("/root/.cache", client.CacheVolume("go-build-cache"))
	}
}

func printOutput(ctx context.Context, dc *dagger.Container) error {
	output, err := dc.Stdout(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%s", output)
	return nil
}
