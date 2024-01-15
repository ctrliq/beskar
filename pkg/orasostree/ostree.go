// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
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
	ArtifactsPathPrefix = "artifacts"
	OSTreePathPrefix    = "ostree"

	OSTreeConfigType = "application/vnd.ciq.ostree.file.v1.config+json"
	OSTreeLayerType  = "application/vnd.ciq.ostree.v1.file"

	FileSummary    = "summary"
	FileSummarySig = "summary.sig"
	FileConfig     = "config"
)

func NewOSTreeFilePusher(repoRootDir, path, repo string, opts ...name.Option) (oras.Pusher, error) {
	if !strings.HasPrefix(repo, ArtifactsPathPrefix+"/") {
		if !strings.HasPrefix(repo, OSTreePathPrefix+"/") {
			repo = filepath.Join(OSTreePathPrefix, repo)
		}

		repo = filepath.Join(ArtifactsPathPrefix, repo)
	}

	// Sanitize the path to match the format of the tag. See internal/plugins/ostree/embedded/data.json.
	// In this case the file path needs to be relative to the repository root and not contain a leading slash.
	path = strings.TrimPrefix(path, repoRootDir)
	path = strings.TrimPrefix(path, "/")

	fileTag := MakeTag(path)
	rawRef := filepath.Join(repo, "file:"+fileTag)
	ref, err := name.ParseReference(rawRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("while parsing reference %s: %w", rawRef, err)
	}

	absolutePath := filepath.Join(repoRootDir, path)

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(OSTreeConfigType, nil),
		oras.NewLocalFileLayer(absolutePath, oras.WithLayerMediaType(OSTreeLayerType)),
	), nil
}

// specialTags are tags that are not md5 hashes of the filename.
// These files are meant to stand out in the registry.
// Note: Values are not limited to the repo's root directory, but at the moment on the following have been identified.
var specialTags = []string{
	FileSummary,
	FileSummarySig,
	FileConfig,
}

// MakeTag creates a tag for a file.
// If the filename starts with a special tag, the tag is returned as-is.
// Otherwise, the tag is the md5 hash of the filename.
func MakeTag(filename string) string {
	for _, tag := range specialTags {
		if filename == tag {
			return tag
		}
	}

	//nolint:gosec
	return fmt.Sprintf("%x", md5.Sum([]byte(filename)))
}
