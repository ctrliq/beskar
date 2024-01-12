// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package beskar

import (
	"context"
	"fmt"

	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver/factory"
	"go.ciq.dev/beskar/internal/pkg/config"
)

func RunGC(ctx context.Context, beskarConfig *config.BeskarConfig, dryRun, removeUntagged bool) error {
	registryConfig := beskarConfig.Registry

	driver, err := factory.Create(ctx, registryConfig.Storage.Type(), registryConfig.Storage.Parameters())
	if err != nil {
		return fmt.Errorf("failed to construct %s driver: %w", registryConfig.Storage.Type(), err)
	}

	registry, err := storage.NewRegistry(ctx, driver)
	if err != nil {
		return fmt.Errorf("failed to construct registry: %w", err)
	}

	err = storage.MarkAndSweep(ctx, driver, registry, storage.GCOpts{
		DryRun:         dryRun,
		RemoveUntagged: removeUntagged,
	})
	if err != nil {
		return fmt.Errorf("failed to garbage collect: %w", err)
	}

	return nil
}
