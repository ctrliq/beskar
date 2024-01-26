// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirror

import (
	"context"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/mirror/api/v1"
)

func checkRepository(repository string) error {
	if !apiv1.RepositoryMatch(repository) {
		return werror.Wrapf(gcode.ErrInvalidArgument, "invalid repository name, must match expression %q", apiv1.RepositoryRegex)
	}
	return nil
}

func (p *Plugin) CreateRepository(ctx context.Context, repository string, properties *apiv1.RepositoryProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryManager.Get(ctx, repository).CreateRepository(ctx, properties)
}

func (p *Plugin) DeleteRepository(ctx context.Context, repository string, deleteFiles bool) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryManager.Get(ctx, repository).DeleteRepository(ctx, deleteFiles)
}

func (p *Plugin) UpdateRepository(ctx context.Context, repository string, properties *apiv1.RepositoryProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryManager.Get(ctx, repository).UpdateRepository(ctx, properties)
}

func (p *Plugin) GetRepository(ctx context.Context, repository string) (properties *apiv1.RepositoryProperties, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).GetRepository(ctx)
}

func (p *Plugin) SyncRepository(ctx context.Context, repository string, wait bool) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryManager.Get(ctx, repository).SyncRepository(ctx, wait)
}

func (p *Plugin) GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *apiv1.SyncStatus, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).GetRepositorySyncStatus(ctx)
}

func (p *Plugin) ListRepositoryLogs(ctx context.Context, repository string, page *apiv1.Page) (logs []apiv1.RepositoryLog, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).ListRepositoryLogs(ctx, page)
}

func (p *Plugin) ListRepositoryFiles(ctx context.Context, repository string, page *apiv1.Page) (repositoryFiles []*apiv1.RepositoryFile, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).ListRepositoryFiles(ctx, page)
}

func (p *Plugin) GetRepositoryFile(ctx context.Context, repository, file string) (repositoryFile *apiv1.RepositoryFile, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryManager.Get(ctx, repository).GetRepositoryFile(ctx, file)
}
