// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBeskarStaticConfig(t *testing.T) {
	bc, err := ParseBeskarStaticConfig("")
	require.NoError(t, err)

	require.Equal(t, "1.0", bc.Version)

	require.Equal(t, "0.0.0.0:5200", bc.Addr)

	require.Equal(t, true, bc.Profiling)

	require.Equal(t, "/tmp/beskar-static", bc.DataDir)

	require.Equal(t, "filesystem", bc.Storage.Driver)
	require.Equal(t, "", bc.Storage.Prefix)

	require.Equal(t, "127.0.0.1:9100", bc.Storage.S3.Endpoint)
	require.Equal(t, "beskar-static", bc.Storage.S3.Bucket)
	require.Equal(t, "minioadmin", bc.Storage.S3.AccessKeyID)
	require.Equal(t, "minioadmin", bc.Storage.S3.SecretAccessKey)
	require.Equal(t, "", bc.Storage.S3.SessionToken)
	require.Equal(t, "us-east-1", bc.Storage.S3.Region)
	require.Equal(t, true, bc.Storage.S3.DisableSSL)

	require.Equal(t, "/tmp/beskar-static", bc.Storage.Filesystem.Directory)

	require.Equal(t, "beskar-static", bc.Storage.GCS.Bucket)
	require.Equal(t, "/path/to/keyfile", bc.Storage.GCS.Keyfile)

	require.Equal(t, "beskar-static", bc.Storage.Azure.Container)
	require.Equal(t, "account_name", bc.Storage.Azure.AccountName)
	require.Equal(t, "base64_encoded_account_key", bc.Storage.Azure.AccountKey)

	require.Equal(t, "debug", bc.Log.Level)
	require.Equal(t, "json", bc.Log.Format)

	require.Equal(t, "0.0.0.0:5201", bc.Gossip.Addr)
	require.Equal(t, "XD1IOhcp0HWFgZJ/HAaARqMKJwfMWtz284Yj7wxmerA=", bc.Gossip.Key)
	require.Equal(t, []string{"127.0.0.1:5102"}, bc.Gossip.Peers)
}
