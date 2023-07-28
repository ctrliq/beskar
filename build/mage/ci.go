package mage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
)

type CI mg.Namespace

func (ci CI) Image(ctx context.Context, repository, username, password string) error {
	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	repoSplitted := strings.SplitN(repository, "/", 2)
	if len(repoSplitted) != 2 {
		return fmt.Errorf("bad repository format")
	}

	target := filepath.Base(repository)
	idx := strings.LastIndexByte(target, ':')
	if idx < 0 {
		return fmt.Errorf("missing tag in repository URL")
	}

	target = target[:idx]

	passwordSecret := client.SetSecret("password", password)

	buildCtx := withBuildOptions(ctx, &buildOptions{
		client:    client,
		platforms: getSupportedPlatforms(),
		publishOptions: &publishOptions{
			registry:   repoSplitted[0],
			repository: repoSplitted[1],
			username:   username,
			password:   passwordSecret,
		},
	})

	return Build{}.build(buildCtx, target)
}

func (ci CI) Chart(ctx context.Context, repository, username, password string) error {
	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	repoSplitted := strings.SplitN(repository, "/", 2)
	if len(repoSplitted) != 2 {
		return fmt.Errorf("bad repository format")
	}

	idx := strings.LastIndexByte(repository, ':')
	if idx < 0 {
		return fmt.Errorf("missing tag in repository URL")
	}

	registry := repoSplitted[0]
	version := repository[idx+1:]
	repository = repository[:idx]

	idx = strings.LastIndexByte(repository, '/')
	if idx < 0 {
		return fmt.Errorf("bad repository URL")
	}
	target := repository[idx+1:]
	repository = repository[:idx]

	charts := client.Host().Directory(filepath.Join("charts", target))

	helmImage := client.Container().From(HelmImage)

	helmImage = helmImage.
		WithDirectory("/charts", charts).
		WithWorkdir("/charts").
		With(goCache(client))

	helmImage = helmImage.
		WithExec([]string{
			"package", "--app-version", version, "--version", version, ".",
		}).
		WithExec([]string{
			"registry", "login", "-u", username, "-p", password, registry,
		}).
		WithExec([]string{
			"push", fmt.Sprintf("%s-%s.tgz", target, version), fmt.Sprintf("oci://%s", repository),
		})

	return printOutput(ctx, helmImage)
}
