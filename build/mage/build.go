package mage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dagger.io/dagger"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/target"
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
	version    string
}

type buildOptions struct {
	client         *dagger.Client
	platforms      []dagger.Platform
	publishOptions *publishOptions
	version        string
}

type genAPI struct {
	path          string
	filename      string
	interfaceName string
}

type binaryConfig struct {
	configFiles       map[string]string
	excludedPlatforms map[dagger.Platform]struct{}
	execStmts         [][]string
	useProto          bool
	genAPI            *genAPI
	buildTags         []string
	baseImage         string
	integrationTest   *integrationTest
}

const (
	beskarBinary       = "beskar"
	beskarctlBinary    = "beskarctl"
	beskarYUMBinary    = "beskar-yum"
	beskarStaticBinary = "beskar-static"
	beskarOSTreeBinary = "beskar-ostree"
)

var binaries = map[string]binaryConfig{
	BeskarBinary: {
		configFiles: map[string]string{
			"internal/pkg/config/default/beskar.yaml": "/etc/beskar/beskar.yaml",
		},
		useProto:  true,
		buildTags: []string{"include_gcs"},
		integrationTest: &integrationTest{
			envs: map[string]string{
				"BESKAR_REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY": "/tmp/integration/registry",
				"BESKAR_REGISTRY_LOG_ACCESSLOG_DISABLED":           "true",
				"BESKAR_REGISTRY_HTTP_ADDR":                        "127.0.0.1:5100",
				"BESKAR_GOSSIP_ADDR":                               "127.0.0.1:5102",
				"BESKAR_CACHE_ADDR":                                "127.0.0.1:5103",
			},
		},
	},
	BeskarctlBinary: {},
	BeskarYUMBinary: {
		configFiles: map[string]string{
			"internal/plugins/yum/pkg/config/default/beskar-yum.yaml": "/etc/beskar/beskar-yum.yaml",
		},
		execStmts: [][]string{
			{
				"apk", "add", "createrepo_c", "--repository=http://dl-cdn.alpinelinux.org/alpine/edge/testing/",
				// NOTE: restore in case alpine createrepo_c package is broken again
				//"sh", "-c", "apt-get update -y && apt-get install -y --no-install-recommends ca-certificates createrepo-c && " +
				//	"rm -rf /var/lib/apt/lists/* && rm -Rf /usr/share/doc && rm -Rf /usr/share/man && apt-get clean",
			},
		},
		genAPI: &genAPI{
			path:          "pkg/plugins/yum/api/v1",
			filename:      "api.go",
			interfaceName: "YUM",
		},
		useProto: true,
		// NOTE: restore in case alpine createrepo_c package is broken again
		//baseImage: "debian:bullseye-slim",
		integrationTest: &integrationTest{
			isPlugin: true,
			envs: map[string]string{
				"BESKARYUM_ADDR":                         "127.0.0.1:5200",
				"BESKARYUM_GOSSIP_ADDR":                  "127.0.0.1:5201",
				"BESKARYUM_STORAGE_FILESYSTEM_DIRECTORY": "/tmp/integration/beskar-yum",
			},
		},
	},
	BeskarStaticBinary: {
		configFiles: map[string]string{
			"internal/plugins/static/pkg/config/default/beskar-static.yaml": "/etc/beskar/beskar-static.yaml",
		},
		genAPI: &genAPI{
			path:          "pkg/plugins/static/api/v1",
			filename:      "api.go",
			interfaceName: "Static",
		},
		useProto: true,
		integrationTest: &integrationTest{
			isPlugin: true,
			envs: map[string]string{
				"BESKARSTATIC_ADDR":                         "127.0.0.1:5300",
				"BESKARSTATIC_GOSSIP_ADDR":                  "127.0.0.1:5301",
				"BESKARSTATIC_STORAGE_FILESYSTEM_DIRECTORY": "/tmp/integration/beskar-static",
			},
		},
	},
	beskarOSTreeBinary: {
		configFiles: map[string]string{
			"internal/plugins/ostree/pkg/config/default/beskar-ostree.yaml": "/etc/beskar/beskar-ostree.yaml",
		},
		genAPI: &genAPI{
			path:          "pkg/plugins/ostree/api/v1",
			filename:      "api.go",
			interfaceName: "OSTree",
		},
		useProto:  true,
		baseImage: "alpine:3.17",
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
		build, err := target.Dir("build/output/beskar", "api")
		if err != nil {
			return err
		} else if !build {
			return nil
		}
		return b.buildProto(ctx)
	})
}

func (b Build) Beskar(ctx context.Context) error {
	return b.build(ctx, BeskarBinary)
}

func (b Build) Beskarctl(ctx context.Context) error {
	return b.build(ctx, BeskarctlBinary)
}

func (b Build) Plugins(ctx context.Context) {
	mg.CtxDeps(
		ctx,
		mg.F(b.Plugin, beskarYUMBinary),
		mg.F(b.Plugin, beskarStaticBinary),
		mg.F(b.Plugin, beskarOSTreeBinary),
	)
}

func (b Build) Plugin(ctx context.Context, name string) error {
	binary, ok := binaries[name]
	if !ok {
		return fmt.Errorf("unknown plugin %s", name)
	} else if binary.genAPI != nil {
		build, err := target.Dir(filepath.Join("build/output", name), binary.genAPI.path)
		if err != nil {
			return err
		} else if build {
			if err := b.genAPI(ctx, name, binary.genAPI); err != nil {
				return err
			}
		}
	}
	return b.build(ctx, name)
}

func (b Build) build(ctx context.Context, name string) error {
	binaryConfig := binaries[name]

	if binaryConfig.useProto {
		if err := b.Proto(ctx); err != nil {
			return err
		}
	}

	buildOpts, ok := getBuildOptions(ctx)

	currentPlatform, err := getCurrentPlatform()
	if err != nil {
		return err
	}

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

	files, err := getGoFiles(filepath.Join("cmd", name))
	if err != nil {
		return err
	}
	changed, err := target.Path(filepath.Join("build/output", name), files...)
	if err != nil {
		return err
	} else if !changed && !ok {
		return nil
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
			WithMountedDirectory("/src", src).
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

		buildArgs := []string{
			"go", "build", "-mod=readonly", "-o", path,
		}

		if buildOpts.version != "" {
			buildArgs = append(buildArgs, "-ldflags", fmt.Sprintf("-X go.ciq.dev/beskar/pkg/version.Semver=v%s", buildOpts.version))
		} else {
			buildTime := time.Now().Format("20060102150405")
			buildArgs = append(buildArgs, "-ldflags", fmt.Sprintf("-X go.ciq.dev/beskar/pkg/version.Semver=v0.0.0-dev-%s", buildTime))
		}

		if len(binaryConfig.buildTags) > 0 {
			buildArgs = append(buildArgs, "-tags", strings.Join(binaryConfig.buildTags, ","))
		}

		buildArgs = append(buildArgs, "./"+inputCmd)

		golang = golang.WithExec(buildArgs)

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
			baseImage := binaryConfig.baseImage
			if baseImage == "" {
				baseImage = BaseImage
			}

			container := client.
				Container(dagger.ContainerOpts{
					Platform: platform,
				}).
				From(baseImage)

			for _, execStmt := range binaryConfig.execStmts {
				container = container.WithExec(execStmt)
			}

			container = container.WithFile(filepath.Join("/usr/bin", name), output)

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
		imageRepository := fmt.Sprintf(
			"%s/%s:%s",
			buildOpts.publishOptions.registry, buildOpts.publishOptions.repository,
			buildOpts.publishOptions.version,
		)
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
		WithMountedDirectory("/api", src).
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

func (b Build) genAPI(ctx context.Context, name string, genAPI *genAPI) error {
	fmt.Printf("Generating %s API\n", name)

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getSource(client)

	workdir := filepath.Join("/src", genAPI.path)
	golang := client.Container().From(GoImage).
		WithMountedDirectory("/src", src).
		WithWorkdir(workdir).
		With(goCache(client)).
		WithExec([]string{"go", "install", "github.com/RussellLuo/kun/cmd/kungen@latest"}).
		WithExec([]string{"kungen", "-force", genAPI.filename, genAPI.interfaceName})

	outputDir := golang.Directory(workdir)
	entries, err := outputDir.Entries(ctx)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry == genAPI.filename {
			continue
		}
		outputDir.File(entry).Export(ctx, filepath.Join(genAPI.path, entry))
	}

	return printOutput(ctx, golang)
}
