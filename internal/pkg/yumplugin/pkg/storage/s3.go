package storage

import (
	"context"

	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

func initS3(ctx context.Context, storageConfig config.BeskarYumS3Storage, prefix string) (*blob.Bucket, error) {
	bucketName := storageConfig.Bucket

	authMethod, err := s3.NewAuthMethod(
		storageConfig.Endpoint,
		s3.WithCredentials(
			storageConfig.AccessKeyID,
			storageConfig.SecretAccessKey,
			storageConfig.SessionToken,
		),
		s3.WithRegion(storageConfig.Region),
		s3.WithDisableSSL(storageConfig.DisableSSL),
	)
	if err != nil {
		return nil, err
	}

	bucket, err := s3blob.OpenBucket(ctx, authMethod.Session(), bucketName, nil)
	if err != nil {
		return nil, err
	}

	if prefix != "" {
		bucket = blob.PrefixedBucket(bucket, prefix)
	}

	return bucket, nil
}
