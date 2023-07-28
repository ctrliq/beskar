package beskar

import (
	"context"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/reference"
	middleware "github.com/distribution/distribution/v3/registry/middleware/registry"
	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/mailgun/groupcache/v2"
)

type RegistryMiddleware struct {
	registry             distribution.Namespace
	manifestEventHandler ManifestEventHandler
	cache                *groupcache.Group
}

func registerRegistryMiddleware(meh ManifestEventHandler, cache *groupcache.Group) (<-chan distribution.Namespace, error) {
	registryCh := make(chan distribution.Namespace, 1)
	err := middleware.Register("beskar", initRegistryMiddleware(meh, cache, registryCh))
	return registryCh, err
}

func initRegistryMiddleware(meh ManifestEventHandler, cache *groupcache.Group, registryCh chan distribution.Namespace) middleware.InitFunc {
	return func(ctx context.Context, registry distribution.Namespace, driver storagedriver.StorageDriver, options map[string]interface{}) (distribution.Namespace, error) {
		mr := &RegistryMiddleware{
			registry:             registry,
			manifestEventHandler: meh,
			cache:                cache,
		}
		registryCh <- registry
		close(registryCh)
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
	repository, err := m.registry.Repository(ctx, name)
	if err != nil {
		return nil, err
	}

	return &RepositoryMiddleware{
		repository:           repository,
		manifestEventHandler: m.manifestEventHandler,
		cache:                m.cache,
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
