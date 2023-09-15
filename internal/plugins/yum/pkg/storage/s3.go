// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

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

	opts := []s3.AuthMethodOption{
		s3.WithDisableSSL(storageConfig.DisableSSL),
	}

	if storageConfig.AccessKeyID != "" || storageConfig.SecretAccessKey != "" || storageConfig.SessionToken != "" {
		opts = append(opts,
			s3.WithCredentials(
				storageConfig.AccessKeyID,
				storageConfig.SecretAccessKey,
				storageConfig.SessionToken,
			))
	}

	if storageConfig.Region != "" {
		opts = append(opts, s3.WithRegion(storageConfig.Region))
	}

	authMethod, err := s3.NewAuthMethod(
		storageConfig.Endpoint,
		opts...,
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
