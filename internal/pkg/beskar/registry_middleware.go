// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package beskar

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/distribution/distribution/v3"
	middleware "github.com/distribution/distribution/v3/registry/middleware/registry"
	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/distribution/reference"
	"github.com/mailgun/groupcache/v2"
)

type registryCallbackFunc func(distribution.Namespace) *pluginManager

type RegistryMiddleware struct {
	registry             distribution.Namespace
	manifestEventHandler ManifestEventHandler
	cache                atomic.Pointer[groupcache.Group]
	pluginManager        *pluginManager
}

func registerRegistryMiddleware(meh ManifestEventHandler, callbackFn registryCallbackFunc) error {
	return middleware.Register("beskar", initRegistryMiddleware(meh, callbackFn))
}

func initRegistryMiddleware(meh ManifestEventHandler, callbackFn registryCallbackFunc) middleware.InitFunc {
	return func(ctx context.Context, registry distribution.Namespace, driver storagedriver.StorageDriver, options map[string]interface{}) (distribution.Namespace, error) {
		mr := &RegistryMiddleware{
			registry:             registry,
			manifestEventHandler: meh,
		}
		mr.pluginManager = callbackFn(mr)
		return mr, nil
	}
}

// Scope describes the names that can be used with this Namespace. The
// global namespace will have a scope that matches all names. The scope
// effectively provides an identity for the namespace.
func (m *RegistryMiddleware) Scope() distribution.Scope {
	return m.registry.Scope()
}

// Repository should return a reference to the named repository. The
// registry may or may not have the repository but should always return a
// reference.
func (m *RegistryMiddleware) Repository(ctx context.Context, name reference.Named) (distribution.Repository, error) {
	if mc, ok := ctx.Value(&manifestCacheKey).(*manifestCache); ok {
		m.cache.Store(mc.Group)
	}

	matches := artifactsMatch.FindStringSubmatch(name.String())
	if len(matches) == 2 {
		if !m.pluginManager.hasPlugin(matches[1]) {
			return nil, fmt.Errorf("no plugin found for %s artifacts", matches[1])
		}
	}

	repository, err := m.registry.Repository(ctx, name)
	if err != nil {
		return nil, err
	}

	if _, ok := ctx.Value(&noCacheKey).(*int); ok {
		return &RepositoryMiddleware{
			repository:           repository,
			manifestEventHandler: m.manifestEventHandler,
		}, nil
	}

	return &RepositoryMiddleware{
		repository:           repository,
		manifestEventHandler: m.manifestEventHandler,
		cache:                m.cache.Load(),
	}, nil
}

// Repositories fills 'repos' with a lexicographically sorted catalog of repositories
// up to the size of 'repos' and returns the value 'n' for the number of entries
// which were filled.  'last' contains an offset in the catalog, and 'err' will be
// set to io.EOF if there are no more entries to obtain.
func (m *RegistryMiddleware) Repositories(ctx context.Context, repos []string, last string) (n int, err error) {
	return m.registry.Repositories(ctx, repos, last)
}

// Blobs returns a blob enumerator to access all blobs
func (m *RegistryMiddleware) Blobs() distribution.BlobEnumerator {
	return m.registry.Blobs()
}

// BlobStatter returns a BlobStatter to control
func (m *RegistryMiddleware) BlobStatter() distribution.BlobStatter {
	return m.registry.BlobStatter()
}
