// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasfile

import (
	"crypto/md5" //nolint:gosec
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	StaticFileConfigType = "application/vnd.ciq.static.file.v1.config+json"
	StaticFileLayerType  = "application/vnd.ciq.static.v1.file"
)

func NewStaticFilePusher(path, repo string, opts ...name.Option) (oras.Pusher, error) {
	if !strings.HasPrefix(repo, "artifacts/") {
		if !strings.HasPrefix(repo, "static/") {
			repo = filepath.Join("artifacts", "static", repo)
		} else {
			repo = filepath.Join("artifacts", repo)
		}
	}

	filename := filepath.Base(path)
	//nolint:gosec
	fileTag := fmt.Sprintf("%x", md5.Sum([]byte(filename)))

	rawRef := filepath.Join(repo, "files:"+fileTag)
	ref, err := name.ParseReference(rawRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("while parsing reference %s: %w", rawRef, err)
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(StaticFileConfigType, nil),
		oras.NewLocalFileLayer(path, oras.WithLocalFileLayerMediaType(StaticFileLayerType)),
	), nil
}
