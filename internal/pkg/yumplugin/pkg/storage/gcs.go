package storage

import (
	"context"
	"os"

	"go.ciq.dev/beskar/internal/pkg/config"
	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/gcp"
	"golang.org/x/oauth2/google"
	storagev1 "google.golang.org/api/storage/v1"
)

func initGCS(ctx context.Context, storageConfig config.BeskarYumGCSStorage, prefix string) (*blob.Bucket, error) {
	data, err := os.ReadFile(storageConfig.Keyfile)
	if err != nil {
		return nil, err
	}

	creds, err := google.CredentialsFromJSON(ctx, data, storagev1.DevstorageReadWriteScope)
	if err != nil {
		return nil, err
	}

	client, err := gcp.NewHTTPClient(
		gcp.DefaultTransport(),
		gcp.CredentialsTokenSource(creds))
	if err != nil {
		return nil, err
	}

	bucket, err := gcsblob.OpenBucket(ctx, client, storageConfig.Bucket, nil)
	if err != nil {
		return nil, err
	}

	if prefix != "" {
		bucket = blob.PrefixedBucket(bucket, prefix)
	}

	return bucket, nil
}
