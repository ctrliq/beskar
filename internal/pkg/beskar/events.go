// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package beskar

import (
	"context"

	"github.com/distribution/distribution/v3"
	"github.com/opencontainers/go-digest"
)

type ManifestEventHandler interface {
	Put(context.Context, distribution.Repository, digest.Digest, string, []byte) error
	Delete(context.Context, digest.Digest) error
}
