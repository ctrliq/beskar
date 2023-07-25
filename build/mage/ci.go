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

	idx := strings.LastIndexByte(repoSplitted[1], ':')
	if idx < 0 {
		return fmt.Errorf("missing tag in repository URL")
	}

	semver := repoSplitted[1][idx+1:]
	version := semver
	if semver == "" {
		return fmt.Errorf("version is missing in repository URL")
	} else if version[0] == 'v' {
		version = semver[1:]
	}

	// trim version
	repoSplitted[1] = repoSplitted[1][:idx]
	// target
	target := filepath.Base(repoSplitted[1])

	buildCtx := withBuildOptions(ctx, &buildOptions{
		client:    client,
		platforms: getSupportedPlatforms(),
		version:   semver,
		publishOptions: &publishOptions{
			registry:   repoSplitted[0],
			repository: repoSplitted[1],
			username:   username,
			password:   client.SetSecret("password", password),
			version:    version,
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

	if version == "" {
		return fmt.Errorf("version is missing in repository URL")
	} else if version[0] == 'v' {
		version = version[1:]
	}

	target := filepath.Base(repository)
	repository = filepath.Dir(repository)

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
