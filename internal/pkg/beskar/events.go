package beskar

import (
	"context"

	"github.com/distribution/distribution/v3"
	"github.com/opencontainers/go-digest"
)

type ManifestEventHandler interface {
	Put(context.Context, distribution.Repository, distribution.Manifest, digest.Digest) error
	Delete(context.Context, digest.Digest) error
}
