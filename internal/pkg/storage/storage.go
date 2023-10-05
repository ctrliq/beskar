// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"

	"gocloud.dev/blob"
)

func GetPrefix(config Config) string {
	prefix := config.Prefix

	if prefix != "" && prefix != "/" {
		if prefix[0] == '/' {
			prefix = prefix[1:]
		} else if prefix[len(prefix)-1] != '/' {
			prefix += "/"
		}
	} else if prefix == "/" {
		prefix = ""
	}

	return prefix
}

func Init(ctx context.Context, config Config, prefix string) (*blob.Bucket, error) {
	switch config.Driver {
	case S3StorageDriver:
		return initS3(ctx, config.S3, prefix)
	case FSStorageDriver:
		return initFS(ctx, config.Filesystem, prefix)
	case GCSStorageDriver:
		return initGCS(ctx, config.GCS, prefix)
	case AzureStorageDriver:
		return initAzure(ctx, config.Azure, prefix)
	}
	return nil, fmt.Errorf("unknown storage driver %s", config.Driver)
}
