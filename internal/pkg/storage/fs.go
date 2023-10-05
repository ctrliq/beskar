// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"os"

	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"
)

func initFS(_ context.Context, config FSStorageConfig, prefix string) (*blob.Bucket, error) {
	if err := os.MkdirAll(config.Directory, 0o700); err != nil {
		return nil, err
	}

	bucket, err := fileblob.OpenBucket(config.Directory, nil)
	if err != nil {
		return nil, err
	}

	if prefix != "" {
		bucket = blob.PrefixedBucket(bucket, prefix)
	}

	return bucket, nil
}
