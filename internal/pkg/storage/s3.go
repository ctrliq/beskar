// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"

	"go.ciq.dev/beskar/internal/pkg/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

func initS3(ctx context.Context, config S3StorageConfig, prefix string) (*blob.Bucket, error) {
	bucketName := config.Bucket

	opts := []s3.AuthMethodOption{
		s3.WithDisableSSL(config.DisableSSL),
	}

	if config.AccessKeyID != "" || config.SecretAccessKey != "" || config.SessionToken != "" {
		opts = append(opts,
			s3.WithCredentials(
				config.AccessKeyID,
				config.SecretAccessKey,
				config.SessionToken,
			))
	}

	if config.Region != "" {
		opts = append(opts, s3.WithRegion(config.Region))
	}

	authMethod, err := s3.NewAuthMethod(
		config.Endpoint,
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
