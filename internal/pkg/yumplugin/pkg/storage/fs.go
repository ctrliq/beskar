package storage

import (
	"context"
	"os"

	"go.ciq.dev/beskar/internal/pkg/config"
	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"
)

func initFS(_ context.Context, pluginConfig config.BeskarYumFSStorage, prefix string) (*blob.Bucket, error) {
	if err := os.MkdirAll(pluginConfig.Directory, 0o700); err != nil {
		return nil, err
	}

	bucket, err := fileblob.OpenBucket(pluginConfig.Directory, nil)
	if err != nil {
		return nil, err
	}

	if prefix != "" {
		bucket = blob.PrefixedBucket(bucket, prefix)
	}

	return bucket, nil
}
