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
		Lint.Helm,
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
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		With(goCache(client))

	// Set up the environment for the linter per the settings of each binary.
	// This could lead to conflicts if the binaries have different settings.
	for _, config := range binaries {
		for key, value := range config.buildEnv {
			golangciLint = golangciLint.WithEnvVariable(key, value)
		}

		for _, execStmt := range config.buildExecStmts {
			golangciLint = golangciLint.WithExec(execStmt)
		}
	}

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
		WithMountedDirectory("/api", src).
		WithWorkdir("/api")

	protoLint = protoLint.WithExec([]string{
		"lint", ".",
	})

	return printOutput(ctx, protoLint)
}

func (Lint) Helm(ctx context.Context) error {
	fmt.Println("Running Helm linter")

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getChartsSource(client)

	helmLint := client.Container().From(HelmImage)

	helmLint = helmLint.
		WithMountedDirectory("/charts", src).
		WithWorkdir("/charts")

	entries, err := src.Entries(ctx)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		helmLint = helmLint.WithExec([]string{
			"lint", entry,
		})
		if err := printOutput(ctx, helmLint); err != nil {
			return err
		}
	}

	return nil
}
