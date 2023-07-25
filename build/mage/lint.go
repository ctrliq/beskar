package mage

import (
	"context"
	"fmt"

	"github.com/magefile/mage/mg"
)

type Lint mg.Namespace

func (Lint) All(ctx context.Context) {
	mg.CtxDeps(
		ctx,
		Lint.Go,
		Lint.Proto,
	)
}

func (Lint) Go(ctx context.Context) error {
	fmt.Println("Running Go linter")

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getSource(client)

	golangciLint := client.Container().From(GolangCILintImage)

	golangciLint = golangciLint.
		WithDirectory("/src", src).
		WithWorkdir("/src").
		With(goCache(client))

	golangciLint = golangciLint.WithExec([]string{
		"golangci-lint", "-v", "run", "--modules-download-mode", "readonly", "--timeout", "5m",
	})

	return printOutput(ctx, golangciLint)
}

func (Lint) Proto(ctx context.Context) error {
	fmt.Println("Running Protobuf linter")

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getProtoSource(client)

	protoLint := client.Container().From(ProtolintImage)

	protoLint = protoLint.
		WithDirectory("/api", src).
		WithWorkdir("/api")

	protoLint = protoLint.WithExec([]string{
		"lint", ".",
	})

	return printOutput(ctx, protoLint)
}
