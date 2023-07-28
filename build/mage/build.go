package mage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"dagger.io/dagger"
	"github.com/magefile/mage/mg"
)

var buildContext int

func withBuildOptions(ctx context.Context, options *buildOptions) context.Context {
	return context.WithValue(ctx, &buildContext, options)
}

func getBuildOptions(ctx context.Context) (*buildOptions, bool) {
	bo, exists := ctx.Value(&buildContext).(*buildOptions)
	return bo, exists
}

type publishOptions struct {
	registry   string
	repository string
	username   string
	password   *dagger.Secret
}

type buildOptions struct {
	client         *dagger.Client
	platforms      []dagger.Platform
	publishOptions *publishOptions
}

type binaryConfig struct {
	configFiles       map[string]string
	excludedPlatforms map[dagger.Platform]struct{}
}

var binaries = map[string]binaryConfig{
	"beskar": {
		configFiles: map[string]string{
			"internal/pkg/config/default/beskar.yaml": "/etc/beskar/beskar.yaml",
		},
	},
	"beskarctl": {},
	"beskar-yum": {
		// int/uint overflow issues with doltdb and 32 bits architectures
		excludedPlatforms: map[dagger.Platform]struct{}{
			"linux/arm/v6": {},
			"linux/arm/v7": {},
		},
		configFiles: map[string]string{
			"internal/pkg/config/default/beskar-yum.yaml": "/etc/beskar/beskar-yum.yaml",
		},
	},
}

type Build mg.Namespace

// All builds all targets locally.
func (b Build) All(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		b.Beskar,
		b.Beskarctl,
		b.Plugins,
	)
	return nil
}

func (b Build) Proto(ctx context.Context) error {
	return onceCalls["proto"].call(func() error {
		return b.buildProto(ctx)
	})
}

func (b Build) Beskar(ctx context.Context) error {
	if err := b.Proto(ctx); err != nil {
		return err
	}
	return b.build(ctx, "beskar")
}

func (b Build) Beskarctl(ctx context.Context) error {
	return b.build(ctx, "beskarctl")
}

func (b Build) Plugins(ctx context.Context) {
	mg.CtxDeps(
		ctx,
		mg.F(b.Plugin, "beskar-yum"),
	)
}

func (b Build) Plugin(ctx context.Context, name string) error {
	if err := b.Proto(ctx); err != nil {
		return err
	}
	return b.build(ctx, name)
}

func (b Build) build(ctx context.Context, name string) error {
	binaryConfig := binaries[name]

	currentPlatform, err := getCurrentPlatform()
	if err != nil {
		return err
	}

	buildOpts, ok := getBuildOptions(ctx)

	if !ok {
		buildOpts = &buildOptions{
			platforms: []dagger.Platform{
				dagger.Platform(currentPlatform),
			},
		}
	} else if len(buildOpts.platforms) == 0 {
		buildOpts.platforms = []dagger.Platform{
			dagger.Platform(currentPlatform),
		}
	}

	client := buildOpts.client

	if client == nil {
		client, err = getDaggerClient(ctx)
		if err != nil {
			return err
		}
		defer client.Close()
	}

	containers := make([]*dagger.Container, 0, len(buildOpts.platforms))

	for _, platform := range buildOpts.platforms {
		if _, ok := binaryConfig.excludedPlatforms[platform]; ok {
			continue
		}

		binary := name
		if string(platform) != currentPlatform {
			binary += "-" + getPlatformBinarySuffix(string(platform))
		}

		fmt.Printf("Building %s (%s)\n", binary, platform)

		src := getSource(client)

		golang := client.Container().From(GoImage)

		golang = golang.
			WithDirectory("/src", src).
			WithDirectory("/output", client.Directory()).
			WithWorkdir("/src").
			WithEnvVariable("CGO_ENABLED", "0").
			WithEnvVariable("GOOS", "linux").
			With(goCache(client))

		envs, ok := supportedPlatforms[platform]
		if !ok {
			return fmt.Errorf("platform %s not supported", platform)
		}

		for key, value := range envs {
			golang = golang.WithEnvVariable(key, value)
		}

		path := filepath.Join("/output", binary)
		inputCmd := filepath.Join("cmd", name)

		golang = golang.WithExec([]string{
			"go", "build", "-mod=readonly", "-o", path, "./" + inputCmd,
		})

		if err := printOutput(ctx, golang); err != nil {
			return err
		}

		output := golang.File(path)

		if buildOpts.publishOptions == nil {
			_, err = output.Export(ctx, filepath.Join("build", "output", binary))
			if err != nil {
				return err
			}
		} else {
			container := client.
				Container(dagger.ContainerOpts{
					Platform: platform,
				}).
				From(BaseImage).
				WithFile(filepath.Join("/usr/bin", name), output)

			for configSrc, configDst := range binaryConfig.configFiles {
				container = container.WithFile(configDst, golang.File(configSrc))
			}

			containers = append(
				containers,
				container,
			)
		}
	}

	if len(containers) > 0 && buildOpts.publishOptions != nil {
		imageRepository := fmt.Sprintf("%s/%s", buildOpts.publishOptions.registry, buildOpts.publishOptions.repository)
		images := client.Container()

		if buildOpts.publishOptions.username != "" || buildOpts.publishOptions.password != nil {
			images = images.WithRegistryAuth(
				buildOpts.publishOptions.registry,
				buildOpts.publishOptions.username,
				buildOpts.publishOptions.password,
			)
		}

		digest, err := images.Publish(ctx, imageRepository, dagger.ContainerPublishOpts{
			PlatformVariants: containers,
		})
		if err != nil {
			return err
		}
		fmt.Println("Image pushed", digest)
	}

	return nil
}

func (b Build) buildProto(ctx context.Context) error {
	fmt.Println("Generating Go protobuf files")

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getProtoSource(client)

	protoFiles, err := filepath.Glob("api/*/v1/*.proto")
	if err != nil {
		return err
	}

	protocArches := map[string]string{
		"amd64":    "x86_64",
		"amd64p32": "x86_32",
		"386":      "x86_32",
		"s390x":    "s390_64",
		"arm64":    "aarch_64",
		"ppc64le":  "ppcle_64",
	}
	arch, ok := protocArches[runtime.GOARCH]
	if !ok {
		return fmt.Errorf("protoc release not found for architecture %s", runtime.GOARCH)
	}

	protoc := client.Container().
		With(goCache(client)).
		Build(src, dagger.ContainerBuildOpts{
			Dockerfile: "build/protoc.Dockerfile",
			BuildArgs: []dagger.BuildArg{
				{
					Name:  "PROTOC_VERSION",
					Value: ProtocVersion,
				},
				{
					Name:  "PROTOC_FILE",
					Value: fmt.Sprintf(ProtocFileFormat, arch),
				},
				{
					Name:  "PROTOC_GEN_GO_VERSION",
					Value: ProtocGenGoVersion,
				},
				{
					Name:  "PROTOC_GEN_GO_GRPC_VERSION",
					Value: ProtocGenGoGrpcVersion,
				},
				{
					Name:  "GOLANG_IMAGE",
					Value: GoImage,
				},
				{
					Name:  "BASE_IMAGE",
					Value: BaseImage,
				},
			},
		})

	protoc = protoc.
		WithDirectory("/api", src).
		WithWorkdir("/")

	protoc = protoc.
		WithExec(
			append([]string{"protoc", "--go_out=module=go.ciq.dev/beskar:.", "-I", "api"}, protoFiles...),
		).
		WithExec(
			append([]string{"protoc", "--go-grpc_out=module=go.ciq.dev/beskar:.", "-I", "api"}, protoFiles...),
		)

	if err := os.RemoveAll("pkg/api"); err != nil {
		return err
	}

	_, err = protoc.Directory("/pkg").Export(ctx, "pkg")
	if err != nil {
		return err
	}

	return printOutput(ctx, protoc)
}
