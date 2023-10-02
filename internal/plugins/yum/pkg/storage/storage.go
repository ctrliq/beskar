// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"

	"go.ciq.dev/beskar/internal/pkg/config"
	"gocloud.dev/blob"
)

func GetPrefix(pluginConfig *config.BeskarYumConfig) string {
	prefix := pluginConfig.Storage.Prefix

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

func Init(ctx context.Context, pluginConfig *config.BeskarYumConfig, prefix string) (*blob.Bucket, error) {
	switch pluginConfig.Storage.Driver {
	case config.S3StorageDriver:
		return initS3(ctx, pluginConfig.Storage.S3, prefix)
	case config.FSStorageDriver:
		return initFS(ctx, pluginConfig.Storage.Filesystem, prefix)
	case config.GCSStorageDriver:
		return initGCS(ctx, pluginConfig.Storage.GCS, prefix)
	case config.AzureStorageDriver:
		return initAzure(ctx, pluginConfig.Storage.Azure, prefix)
	}
	return nil, fmt.Errorf("unknown storage driver %s", pluginConfig.Storage.Driver)
}
