// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBeskarConfig(t *testing.T) {
	bc, err := ParseBeskarConfig("")
	require.NoError(t, err)

	require.Equal(t, "1.0", bc.Version)
	require.Equal(t, true, bc.Profiling)

	require.Equal(t, "0.0.0.0:5103", bc.Cache.Addr)
	require.Equal(t, uint32(64), bc.Cache.Size)

	require.Equal(t, "0.0.0.0:5102", bc.Gossip.Addr)
	require.Equal(t, "XD1IOhcp0HWFgZJ/HAaARqMKJwfMWtz284Yj7wxmerA=", bc.Gossip.Key)
	require.Equal(t, []string{}, bc.Gossip.Peers)
}
