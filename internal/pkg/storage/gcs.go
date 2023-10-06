// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"os"

	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/gcp"
	"golang.org/x/oauth2/google"
	storagev1 "google.golang.org/api/storage/v1"
)

func initGCS(ctx context.Context, config GCSStorageConfig, prefix string) (*blob.Bucket, error) {
	data, err := os.ReadFile(config.Keyfile)
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

	bucket, err := gcsblob.OpenBucket(ctx, client, config.Bucket, nil)
	if err != nil {
		return nil, err
	}

	if prefix != "" {
		bucket = blob.PrefixedBucket(bucket, prefix)
	}

	return bucket, nil
}
