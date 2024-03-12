package mage

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"

	"dagger.io/dagger"
	"golang.org/x/sys/cpu"
	"golang.org/x/tools/go/packages"
)

const (
	GoImage           = "golang:1.21.5-alpine3.19"
	GolangCILintImage = "golangci/golangci-lint:v1.54.2-alpine"
	HelmImage         = "alpine/helm:3.12.2"
	ProtolintImage    = "yoheimuta/protolint:0.45.0"
	BaseImage         = "alpine:3.19"

	ProtocVersion    = "v23.4"
	ProtocFileFormat = "protoc-23.4-linux-%s.zip"

	ProtocGenGoVersion     = "v1.31.0"
	ProtocGenGoGrpcVersion = "v1.1.0"
)

var supportedPlatforms = map[dagger.Platform]map[string]string{
	"linux/amd64":   {"GOARCH": "amd64"},
	"linux/arm64":   {"GOARCH": "arm64"},
	"linux/s390x":   {"GOARCH": "s390x"},
	"linux/ppc64le": {"GOARCH": "ppc64le"},
	"linux/arm/v6":  {"GOARCH": "arm", "GOARM": "6"},
	"linux/arm/v7":  {"GOARCH": "arm", "GOARM": "7"},
}

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
			"scripts",
			"charts",
			".git",
			".github",
			"README.md",
			"go.work",
			"go.work.sum",
			"integration",
		},
	})
}

func getIntegrationSource(client *dagger.Client) *dagger.Directory {
	return client.Host().Directory(".", dagger.HostDirectoryOpts{
		Include: []string{
			"go.mod",
			"go.sum",
			"integration",
			"pkg",
		},
	})
}

func getBuildBinaries(client *dagger.Client, binaries ...string) *dagger.Directory {
	return client.Host().Directory("build/output", dagger.HostDirectoryOpts{
		Include: binaries,
	})
}

func getProtoSource(client *dagger.Client) *dagger.Directory {
	return client.Host().Directory("api")
}

func getChartsSource(client *dagger.Client) *dagger.Directory {
	return client.Host().Directory("charts")
}

func goCache(client *dagger.Client) func(dc *dagger.Container) *dagger.Container {
	return func(dc *dagger.Container) *dagger.Container {
		return dc.
			WithMountedCache("/go", client.CacheVolume("go-mod-cache"), dagger.ContainerWithMountedCacheOpts{Sharing: dagger.Locked}).
			WithMountedCache("/root/.cache", client.CacheVolume("go-build-cache"), dagger.ContainerWithMountedCacheOpts{Sharing: dagger.Locked})
	}
}

func printOutput(ctx context.Context, dc *dagger.Container) error {
	output, err := dc.Stdout(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%s", output)

	if strings.Contains(output, "Test Suite Failed\n") {
		return fmt.Errorf("test suite failed")
	}

	return nil
}

func getCurrentPlatform() (string, error) {
	switch runtime.GOARCH {
	case "amd64", "arm64", "s390x", "ppc64le":
		return fmt.Sprintf("linux/%s", runtime.GOARCH), nil
	case "arm":
		variant := "v6"

		if cpu.ARM.HasVFPv3 || cpu.ARM.HasVFPv3D16 || cpu.ARM.HasVFPv4 {
			variant = "v7"
		}

		return fmt.Sprintf("linux/%s/%s", runtime.GOARCH, variant), nil
	}
	return "", fmt.Errorf("%s architecture is not supported", runtime.GOARCH)
}

func getSupportedPlatforms() []dagger.Platform {
	platforms := make([]dagger.Platform, 0, len(supportedPlatforms))
	for platform := range supportedPlatforms {
		platforms = append(platforms, platform)
	}
	return platforms
}

func getPlatformBinarySuffix(platform string) string {
	platform = strings.TrimPrefix(platform, "linux/")
	return strings.ReplaceAll(platform, "/", "-")
}

func getGoFiles(dir string) ([]string, error) {
	cfg := packages.Config{
		Mode: packages.NeedFiles |
			packages.NeedEmbedFiles |
			packages.NeedImports |
			packages.NeedName |
			packages.NeedDeps |
			packages.NeedModule,
		Dir: dir,
	}

	initial, err := packages.Load(&cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("while loading directory %s: %s", dir, err)
	}
	for _, pkg := range initial {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("package error: %s", pkg.Errors[0])
		}
	}

	fileMap := make(map[string]struct{})
	files := make([]string, 0)

	add := func(sourceFiles []string) {
		for _, file := range sourceFiles {
			if _, ok := fileMap[file]; !ok {
				fileMap[file] = struct{}{}
				files = append(files, file)
			}
		}
	}

	packages.Visit(initial, nil, func(pkg *packages.Package) {
		if pkg.Module == nil {
			return
		}
		add(pkg.GoFiles)
		add(pkg.OtherFiles)
		add(pkg.EmbedFiles)
	})

	return files, nil
}
