// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasostree

import (
	"crypto/md5" //nolint:gosec
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	OSTreeConfigType = "application/vnd.ciq.ostree.file.v1.config+json"
	OSTreeLayerType  = "application/vnd.ciq.ostree.v1.file"

	KnownFileSummary = "summary"
	KnownFileConfig  = "config"
)

func NewOSTreePusher(path, repo string, opts ...name.Option) (oras.Pusher, error) {
	if !strings.HasPrefix(repo, "artifacts/") {
		if !strings.HasPrefix(repo, "ostree/") {
			repo = filepath.Join("static", repo)
		}

		repo = filepath.Join("artifacts", repo)
	}

	filename := filepath.Base(path)
	//nolint:gosec
	fileTag := makeTag(filename)

	rawRef := filepath.Join(repo, "files:"+fileTag)
	ref, err := name.ParseReference(rawRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("while parsing reference %s: %w", rawRef, err)
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(OSTreeConfigType, nil),
		oras.NewLocalFileLayer(path, oras.WithLocalFileLayerMediaType(OSTreeLayerType)),
	), nil
}

// specialTags
var specialTags = []string{
	KnownFileSummary,
	KnownFileConfig,
}

func makeTag(filename string) string {
	for _, tag := range specialTags {
		if strings.HasPrefix(filename, tag) {
			return tag
		}
	}

	//nolint:gosec
	return fmt.Sprintf("%x", md5.Sum([]byte(filename)))
}
