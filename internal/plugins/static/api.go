// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package static

import (
	"context"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/static/api/v1"
)

func checkRepository(repository string) error {
	if !apiv1.RepositoryMatch(repository) {
		return werror.Wrapf(gcode.ErrInvalidArgument, "invalid repository name, must match expression %q", apiv1.RepositoryRegex)
	}
	return nil
}

func (p *Plugin) DeleteRepository(ctx context.Context, repository string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryManager.Get(ctx, repository).DeleteRepository(ctx, repository)
}

func (p *Plugin) ListRepositoryLogs(ctx context.Context, repository string, page *apiv1.Page) (logs []apiv1.RepositoryLog, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).ListRepositoryLogs(ctx, page)
}

func (p *Plugin) RemoveRepositoryFile(ctx context.Context, repository string, tag string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryManager.Get(ctx, repository).RemoveRepositoryFile(ctx, tag)
}

func (p *Plugin) GetRepositoryFileByTag(ctx context.Context, repository string, tag string) (repositoryFile *apiv1.RepositoryFile, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).GetRepositoryFileByTag(ctx, tag)
}

func (p *Plugin) GetRepositoryFileByName(ctx context.Context, repository string, name string) (repositoryFile *apiv1.RepositoryFile, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).GetRepositoryFileByName(ctx, name)
}

func (p *Plugin) ListRepositoryFiles(ctx context.Context, repository string, page *apiv1.Page) (repositoryFiles []*apiv1.RepositoryFile, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).ListRepositoryFiles(ctx, page)
}
