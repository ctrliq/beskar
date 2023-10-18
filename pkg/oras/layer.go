// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

type MediaTypeFilter func(types.MediaType) bool

func GetLayer(manifest *v1.Manifest, mediaType types.MediaType) (v1.Descriptor, error) {
	for _, layer := range manifest.Layers {
		if layer.MediaType != mediaType {
			continue
		}
		return layer, nil
	}

	return v1.Descriptor{}, fmt.Errorf("no layer with mediatype %s found in manifest", mediaType)
}

func GetLayerFilter(manifest *v1.Manifest, mediaTypeFilter MediaTypeFilter) (v1.Descriptor, error) {
	for _, layer := range manifest.Layers {
		if mediaTypeFilter(layer.MediaType) {
			return layer, nil
		}
	}

	return v1.Descriptor{}, fmt.Errorf("no recognized layer found in manifest")
}

type LocalFileLayerOption func(*LocalFileLayer)

func WithLocalFileLayerAnnotations(annotations map[string]string) LocalFileLayerOption {
	return func(l *LocalFileLayer) {
		l.annotations = annotations
	}
}

func WithLocalFileLayerPlatform(platform *v1.Platform) LocalFileLayerOption {
	return func(l *LocalFileLayer) {
		l.platform = platform
	}
}

func WithLocalFileLayerMediaType(mediaType string) LocalFileLayerOption {
	return func(l *LocalFileLayer) {
		l.mediaType = mediaType
	}
}

// LocalFileLayer defines an OCI layer descriptor associated to a local file.
type LocalFileLayer struct {
	path string

	digestOnce sync.Once
	digest     v1.Hash

	annotations map[string]string
	platform    *v1.Platform
	mediaType   string
}

// NewLocalLayer returns an OCI layer descriptor for the corresponding local file.
func NewLocalFileLayer(path string, options ...LocalFileLayerOption) *LocalFileLayer {
	layer := &LocalFileLayer{
		path: path,
	}
	for _, opt := range options {
		opt(layer)
	}
	if layer.annotations == nil {
		layer.annotations = make(map[string]string)
	}
	return layer
}

// Digest returns the Hash of the local file.
func (l *LocalFileLayer) Digest() (v1.Hash, error) {
	var err error

	l.digestOnce.Do(func() {
		var f *os.File

		f, err = os.Open(l.path)
		if err != nil {
			return
		}
		defer f.Close()

		l.digest, _, err = v1.SHA256(f)
	})

	return l.digest, err
}

// DiffID returns the Hash of the uncompressed layer (not supported by ORAS).
func (l *LocalFileLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("not supported by ORAS")
}

// Compressed returns an io.ReadCloser for the file content.
func (l *LocalFileLayer) Compressed() (io.ReadCloser, error) {
	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents
// (not supported by ORAS).
func (l *LocalFileLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported by ORAS")
}

// Size returns the size of the local file.
func (l *LocalFileLayer) Size() (int64, error) {
	st, err := os.Stat(l.path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// MediaType returns the media type of the Layer.
func (l *LocalFileLayer) MediaType() (types.MediaType, error) {
	return types.MediaType(l.mediaType), nil
}

// Annotations returns annotations associated to this layer.
func (l *LocalFileLayer) Annotations() map[string]string {
	if l.annotations[imagespec.AnnotationTitle] == "" {
		l.annotations[imagespec.AnnotationTitle] = filepath.Base(l.path)
	}
	return l.annotations
}

// Platform returns platform information for this layer.
func (l *LocalFileLayer) Platform() *v1.Platform {
	return l.platform
}
