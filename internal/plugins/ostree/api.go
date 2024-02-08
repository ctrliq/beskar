// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostree

import (
	"context"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"

	apiv1 "go.ciq.dev/beskar/pkg/plugins/ostree/api/v1"
)

func checkRepository(repository string) error {
	if !apiv1.RepositoryMatch(repository) {
		return werror.Wrapf(gcode.ErrInvalidArgument, "invalid repository name, must match expression %q", apiv1.RepositoryRegex)
	}
	return nil
}

func (p *Plugin) CreateRepository(ctx context.Context, repository string, properties *apiv1.OSTreeRepositoryProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}

	return p.repositoryManager.Get(ctx, repository).CreateRepository(ctx, properties)
}

func (p *Plugin) DeleteRepository(ctx context.Context, repository string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}

	return p.repositoryManager.Get(ctx, repository).DeleteRepository(ctx)
}

func (p *Plugin) ListRepositoryRefs(ctx context.Context, repository string) (refs []apiv1.OSTreeRef, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}

	return p.repositoryManager.Get(ctx, repository).ListRepositoryRefs(ctx)
}

func (p *Plugin) AddRemote(ctx context.Context, repository string, properties *apiv1.OSTreeRemoteProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}

	return p.repositoryManager.Get(ctx, repository).AddRemote(ctx, properties)
}

func (p *Plugin) UpdateRemote(ctx context.Context, repository string, remoteName string, properties *apiv1.OSTreeRemoteProperties) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}

	return p.repositoryManager.Get(ctx, repository).UpdateRemote(ctx, remoteName, properties)
}

func (p *Plugin) DeleteRemote(ctx context.Context, repository string, remoteName string) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}

	return p.repositoryManager.Get(ctx, repository).DeleteRemote(ctx, remoteName)
}

func (p *Plugin) SyncRepository(ctx context.Context, repository string, properties *apiv1.OSTreeRepositorySyncRequest) (err error) {
	if err := checkRepository(repository); err != nil {
		return err
	}

	if properties.Timeout <= 0 {
		properties.Timeout = p.beskarOSTreeConfig.Sync.GetTimeout()
	}

	return p.repositoryManager.Get(ctx, repository).SyncRepository(ctx, properties)
}

func (p *Plugin) GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *apiv1.SyncStatus, err error) {
	if err := checkRepository(repository); err != nil {
		return nil, err
	}

	return p.repositoryManager.Get(ctx, repository).GetRepositorySyncStatus(ctx)
}
