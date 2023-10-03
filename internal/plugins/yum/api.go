// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yum

import (
	"context"
	"strings"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/yum/api/v1"
)

func checkRepository(repository string) error {
	if !strings.HasPrefix(repository, "artifacts/yum/") {
		return werror.Wrapf(gcode.ErrInvalidArgument, "invalid repository name, must start with artifacts/yum/ prefix")
	}
	return nil
}

func (p *Plugin) CreateRepository(ctx context.Context, repository string, properties *apiv1.RepositoryProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryHandler(repository).CreateRepository(ctx, properties)
}

func (p *Plugin) DeleteRepository(ctx context.Context, repository string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryHandler(repository).DeleteRepository(ctx)
}

func (p *Plugin) UpdateRepository(ctx context.Context, repository string, properties *apiv1.RepositoryProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryHandler(repository).UpdateRepository(ctx, properties)
}

func (p *Plugin) GetRepository(ctx context.Context, repository string) (properties *apiv1.RepositoryProperties, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryHandler(repository).GetRepository(ctx)
}

func (p *Plugin) SyncRepository(ctx context.Context, repository string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryHandler(repository).SyncRepository(ctx)
}

func (p *Plugin) GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *apiv1.SyncStatus, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryHandler(repository).GetRepositorySyncStatus(ctx)
}

func (p *Plugin) ListRepositoryLogs(ctx context.Context, repository string, page *apiv1.Page) (logs []apiv1.RepositoryLog, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryHandler(repository).ListRepositoryLogs(ctx, page)
}

func (p *Plugin) RemoveRepositoryPackage(ctx context.Context, repository string, id string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	return p.repositoryHandler(repository).RemoveRepositoryPackage(ctx, id)
}

func (p *Plugin) GetRepositoryPackage(ctx context.Context, repository string, id string) (repositoryPackage *apiv1.RepositoryPackage, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryHandler(repository).GetRepositoryPackage(ctx, id)
}

func (p *Plugin) ListRepositoryPackages(ctx context.Context, repository string, page *apiv1.Page) (repositoryPackages []*apiv1.RepositoryPackage, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	return p.repositoryHandler(repository).ListRepositoryPackages(ctx, page)
}
