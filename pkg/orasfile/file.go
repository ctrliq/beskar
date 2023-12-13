// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasfile

import (
	"crypto/md5" //nolint:gosec
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	StaticFileConfigType = "application/vnd.ciq.static.file.v1.config+json"
	StaticFileLayerType  = "application/vnd.ciq.static.v1.file"
)

func FileReference(filename, repo string, opts ...name.Option) (name.Reference, error) {
	if !strings.HasPrefix(repo, "artifacts/") {
		if !strings.HasPrefix(repo, "static/") {
			repo = filepath.Join("artifacts", "static", repo)
		} else {
			repo = filepath.Join("artifacts", repo)
		}
	}

	//nolint:gosec
	fileTag := fmt.Sprintf("%x", md5.Sum([]byte(filename)))

	rawRef := filepath.Join(repo, "files:"+fileTag)
	return name.ParseReference(rawRef, opts...)
}

func NewStaticFilePusher(path, repo string, opts ...name.Option) (oras.Pusher, error) {
	filename := filepath.Base(path)

	ref, err := FileReference(filename, repo, opts...)
	if err != nil {
		return nil, err
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(StaticFileConfigType, nil),
		oras.NewLocalFileLayer(
			path,
			oras.WithLayerMediaType(StaticFileLayerType),
		),
	), nil
}

func NewStaticFileStreamPusher(stream io.Reader, filename, repo string, opts ...name.Option) (oras.Pusher, error) {
	ref, err := FileReference(filename, repo, opts...)
	if err != nil {
		return nil, err
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(StaticFileConfigType, nil),
		oras.NewStreamLayer(
			stream,
			oras.WithLayerMediaType(StaticFileLayerType),
			oras.WithLayerAnnotations(map[string]string{
				imagespec.AnnotationTitle: filename,
			}),
		),
	), nil
}
