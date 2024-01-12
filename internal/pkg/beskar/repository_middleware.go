// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package beskar

import (
	"context"
	"errors"
	"time"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
	"github.com/mailgun/groupcache/v2"
	"github.com/opencontainers/go-digest"
)

var manifestCacheKey int

type manifestCache struct {
	*groupcache.Group
}

type RepositoryMiddleware struct {
	repository           distribution.Repository
	manifestEventHandler ManifestEventHandler
	cache                *groupcache.Group
}

// Named returns the name of the repository.
func (m *RepositoryMiddleware) Named() reference.Named {
	return m.repository.Named()
}

// Manifests returns a reference to this repository's manifest service.
// with the supplied options applied.
func (m *RepositoryMiddleware) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	manifestService, err := m.repository.Manifests(ctx, options...)
	if err != nil {
		return nil, err
	}

	msw := &manifestServiceWrapper{
		ManifestService:      manifestService,
		manifestEventHandler: m.manifestEventHandler,
		repository:           m,
		cache:                m.cache,
	}

	for _, option := range options {
		if err := option.Apply(manifestService); err != nil {
			return nil, err
		}
	}

	return msw, nil
}

// Blobs returns a reference to this repository's blob service.
func (m *RepositoryMiddleware) Blobs(ctx context.Context) distribution.BlobStore {
	return m.repository.Blobs(ctx)
}

// Tags returns a reference to this repositories tag service
func (m *RepositoryMiddleware) Tags(ctx context.Context) distribution.TagService {
	return m.repository.Tags(ctx)
}

type manifestServiceWrapper struct {
	distribution.ManifestService
	manifestEventHandler ManifestEventHandler
	repository           distribution.Repository
	cache                *groupcache.Group
}

// Exists returns true if the manifest exists.
func (w *manifestServiceWrapper) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	return w.ManifestService.Exists(ctx, dgst)
}

// Get retrieves the manifest specified by the given digest
func (w *manifestServiceWrapper) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	if w.cache != nil {
		destSink := newManifestSink(w.ManifestService, options...)

		if err := w.cache.Get(ctx, getCacheKey(w.repository, dgst), destSink); err != nil {
			if errors.Is(err, &groupcache.ErrNotFound{}) {
				return nil, distribution.ErrManifestUnknownRevision{
					Name:     w.repository.Named().Name(),
					Revision: dgst,
				}
			}
			return nil, err
		}

		return destSink.ToManifest()
	}

	return w.ManifestService.Get(ctx, dgst, options...)
}

// Put creates or updates the given manifest returning the manifest digest
func (w *manifestServiceWrapper) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	dgst, err := w.ManifestService.Put(ctx, manifest, options...)
	if err != nil {
		return "", err
	}

	mediaType, payload, err := manifest.Payload()
	if err != nil {
		return "", err
	}

	if w.cache != nil {
		value, err := encodeManifest(mediaType, payload)
		if err != nil {
			return "", err
		}

		cacheKey := getCacheKey(w.repository, dgst)
		if err := w.cache.Set(ctx, cacheKey, value, time.Now().Add(1*time.Hour), true); err != nil {
			return "", err
		}
	}

	return dgst, w.manifestEventHandler.Put(ctx, w.repository, dgst, mediaType, payload)
}

// Delete removes the manifest specified by the given digest. Deleting
// a manifest that doesn't exist will return ErrManifestNotFound
func (w *manifestServiceWrapper) Delete(ctx context.Context, dgst digest.Digest) error {
	manifest, err := w.Get(ctx, dgst, nil)
	if err != nil {
		return err
	}

	if err := w.ManifestService.Delete(ctx, dgst); err != nil {
		return err
	}

	if w.cache != nil {
		if err := w.cache.Remove(ctx, getCacheKey(w.repository, dgst)); err != nil {
			return err
		}
	}

	mediaType, payload, err := manifest.Payload()
	if err != nil {
		return err
	}

	return w.manifestEventHandler.Delete(ctx, w.repository, dgst, mediaType, payload)
}
