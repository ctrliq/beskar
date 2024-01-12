// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// AuthConfig returns option for the authentication to a remote registry.
func AuthConfig(username, password string) remote.Option {
	return remote.WithAuth(&authn.Basic{
		Username: username,
		Password: password,
	})
}
