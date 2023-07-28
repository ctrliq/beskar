package beskar

import (
	"context"
	"fmt"

	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver/factory"
	"github.com/distribution/distribution/v3/version"
	"github.com/docker/libtrust"
	"go.ciq.dev/beskar/internal/pkg/config"
)

func RunGC(ctx context.Context, beskarConfig *config.BeskarConfig, dryRun, removeUntagged bool) error {
	registryConfig := beskarConfig.Registry

	driver, err := factory.Create(registryConfig.Storage.Type(), registryConfig.Storage.Parameters())
	if err != nil {
		return fmt.Errorf("failed to construct %s driver: %w", registryConfig.Storage.Type(), err)
	}

	ctx = dcontext.WithVersion(ctx, version.Version)

	k, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return err
	}

	registry, err := storage.NewRegistry(ctx, driver, storage.Schema1SigningKey(k))
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
