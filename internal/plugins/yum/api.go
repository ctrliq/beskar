// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yum

import (
	"context"

	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumrepository"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/yum/api/v1"
)

func checkRepository(repository string) error {
	if !apiv1.RepositoryMatch(repository) {
		return werror.Wrapf(gcode.ErrInvalidArgument, "invalid repository name, must match expression %q", apiv1.RepositoryRegex)
	}
	return nil
}

func (p *Plugin) getHandlerForRepository(ctx context.Context, repository string) (*yumrepository.Handler, error) {
	h, ok := p.repositoryManager.Get(ctx, repository).(*yumrepository.Handler)
	if !ok {
		return nil, werror.Wrapf(gcode.ErrNotFound, "repository %q does not exist in the required form", repository)
	}

	return h, nil
}

func (p *Plugin) CreateRepository(ctx context.Context, repository string, properties *apiv1.RepositoryProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return err
	}

	return h.CreateRepository(ctx, properties)
}

func (p *Plugin) DeleteRepository(ctx context.Context, repository string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return err
	}

	return h.DeleteRepository(ctx)
}

func (p *Plugin) UpdateRepository(ctx context.Context, repository string, properties *apiv1.RepositoryProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return err
	}

	return h.UpdateRepository(ctx, properties)
}

func (p *Plugin) GetRepository(ctx context.Context, repository string) (properties *apiv1.RepositoryProperties, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	return h.GetRepository(ctx)
}

func (p *Plugin) SyncRepository(ctx context.Context, repository string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return err
	}

	return h.SyncRepository(ctx)
}

func (p *Plugin) GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *apiv1.SyncStatus, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	return h.GetRepositorySyncStatus(ctx)
}

func (p *Plugin) ListRepositoryLogs(ctx context.Context, repository string, page *apiv1.Page) (logs []apiv1.RepositoryLog, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	return h.ListRepositoryLogs(ctx, page)
}

func (p *Plugin) RemoveRepositoryPackage(ctx context.Context, repository string, id string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return err
	}

	return h.RemoveRepositoryPackage(ctx, id)
}

func (p *Plugin) RemoveRepositoryPackageByTag(ctx context.Context, repository string, tag string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return err
	}

	return h.RemoveRepositoryPackageByTag(ctx, tag)
}

func (p *Plugin) GetRepositoryPackage(ctx context.Context, repository string, id string) (repositoryPackage *apiv1.RepositoryPackage, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	return h.GetRepositoryPackage(ctx, id)
}

func (p *Plugin) GetRepositoryPackageByTag(ctx context.Context, repository string, tag string) (repositoryPackage *apiv1.RepositoryPackage, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	return h.GetRepositoryPackageByTag(ctx, tag)
}

func (p *Plugin) ListRepositoryPackages(ctx context.Context, repository string, page *apiv1.Page) (repositoryPackages []*apiv1.RepositoryPackage, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}
	h, err := p.getHandlerForRepository(ctx, repository)
	if err != nil {
		return nil, err
	}

	return h.ListRepositoryPackages(ctx, page)
}
