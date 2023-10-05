// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"gocloud.dev/blob"
	"gocloud.dev/blob/azureblob"
)

func initAzure(ctx context.Context, config AzureStorageConfig, prefix string) (*blob.Bucket, error) {
	sharedKeyCred, err := azblob.NewSharedKeyCredential(config.AccountName, config.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed azblob.NewSharedKeyCredential: %w", err)
	}

	options := azureblob.NewDefaultServiceURLOptions()
	options.AccountName = config.AccountName

	serviceURL, err := azureblob.NewServiceURL(options)
	if err != nil {
		return nil, err
	}

	azClientOpts := &container.ClientOptions{}
	azClientOpts.Telemetry = policy.TelemetryOptions{
		ApplicationID: "beskar",
	}

	containerClient, err := container.NewClientWithSharedKeyCredential(string(serviceURL), sharedKeyCred, azClientOpts)
	if err != nil {
		return nil, err
	}

	bucket, err := azureblob.OpenBucket(ctx, containerClient, nil)
	if err != nil {
		return nil, err
	}

	if prefix != "" {
		bucket = blob.PrefixedBucket(bucket, prefix)
	}

	return bucket, nil
}
