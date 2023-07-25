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

type Build mg.Namespace

// All builds all targets locally.
func (b Build) All(ctx context.Context) error {
	if err := b.Proto(ctx); err != nil {
		return err
	}
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
		mg.F(b.build, "beskar-yum"),
	)
}

func (b Build) Plugin(ctx context.Context, name string) error {
	if err := b.Proto(ctx); err != nil {
		return err
	}
	return b.build(ctx, name)
}

func (b Build) build(ctx context.Context, name string) error {
	fmt.Println("Building", name)

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getSource(client)

	golang := client.Container().From(GoImage)

	golang = golang.
		WithDirectory("/src", src).
		WithWorkdir("/src").
		With(goCache(client))

	path := filepath.Join("build", "output", name)
	inputCmd := filepath.Join("cmd", name)

	golang = golang.WithExec([]string{
		"go", "build", "-mod=readonly", "-o", path, "./" + inputCmd,
	})

	if err := printOutput(ctx, golang); err != nil {
		return err
	}

	output := golang.File(path)

	_, err = output.Export(ctx, path)
	if err != nil {
		return err
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
