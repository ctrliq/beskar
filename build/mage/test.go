package mage

import (
	"context"
	"dagger.io/dagger"
	"fmt"
	"github.com/magefile/mage/mg"
	"strings"
)

type Test mg.Namespace

func (Test) Unit(ctx context.Context) error {
	fmt.Println("Running unit tests")

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getSource(client)

	unitTest := client.Container().From(GoImage)

	unitTest = unitTest.
		WithDirectory("/src", src).
		WithWorkdir("/src").
		With(goCache(client))

	for _, config := range binaries {
		for key, value := range config.buildEnv {
			unitTest = unitTest.WithEnvVariable(key, value)
		}

		for _, execStmt := range config.buildExecStmts {
			unitTest = unitTest.WithExec(execStmt)
		}
	}

	unitTest = unitTest.WithExec([]string{
		"go", "test", "-v", "-count=1", "./...",
	})

	return printOutput(ctx, unitTest)
}

type integrationTest struct {
	envs     map[string]string
	isPlugin bool
}

func (Test) Integration(ctx context.Context) error {
	mg.CtxDeps(
		ctx,
		Build.Beskar,
		Build.Plugins,
	)

	fmt.Println("Running integration tests")
	fmt.Println("")

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	var buildBinaries []string
	var pluginBinaries []string

	for bin, config := range binaries {
		if config.integrationTest == nil {
			continue
		}
		buildBinaries = append(buildBinaries, bin)
		if config.integrationTest.isPlugin {
			pluginBinaries = append(pluginBinaries, bin)
		}
	}

	setEnvs := func(c *dagger.Container) *dagger.Container {
		c = c.WithEnvVariable("BESKAR_PLUGINS", strings.Join(pluginBinaries, " "))

		for _, config := range binaries {
			if config.integrationTest == nil {
				continue
			}
			for key, val := range config.integrationTest.envs {
				c = c.WithEnvVariable(key, val)
			}
			// execute exec statements for plugins using alpine base image
			if config.baseImage != "" && config.baseImage != BaseImage {
				continue
			}
			for _, execStmt := range config.execStmts {
				c = c.WithExec(execStmt)
			}
		}

		return c
	}

	integrationTest := client.Container().
		From(GoImage).
		With(goCache(client)).
		WithEnvVariable("GOBIN", "/usr/bin").
		WithExec([]string{"go", "install", "github.com/onsi/ginkgo/v2/ginkgo@v2.13.2"}).
		WithExec([]string{"apk", "add", "mailcap"}) // mime types for the static file server

	integrationTest = integrationTest.
		With(goCache(client)).
		WithDirectory("/src", getIntegrationSource(client)).
		WithDirectory("/app", getBuildBinaries(client, buildBinaries...)).
		WithWorkdir("/src").
		With(setEnvs).
		WithExec([]string{
			"sh", "-c", "ginkgo -v -p --repeat 4 --timeout 5m ./integration || cat /tmp/logs ; rm -f /tmp/logs",
		}, dagger.ContainerWithExecOpts{
			SkipEntrypoint: true,
		})

	return printOutput(ctx, integrationTest)
}
