// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/pkg/decompress"
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

type LayerOption func(*LayerOptions)

func WithLayerAnnotations(annotations map[string]string) LayerOption {
	return func(o *LayerOptions) {
		o.annotations = annotations
	}
}

func WithLayerPlatform(platform *v1.Platform) LayerOption {
	return func(o *LayerOptions) {
		o.platform = platform
	}
}

func WithLayerMediaType(mediaType string) LayerOption {
	return func(o *LayerOptions) {
		o.mediaType = mediaType
	}
}

type LayerOptions struct {
	annotations map[string]string
	platform    *v1.Platform
	mediaType   string
}

// LocalFileLayer defines an OCI layer descriptor associated to a local file.
type LocalFileLayer struct {
	path string

	digestOnce sync.Once
	digest     v1.Hash

	options LayerOptions
}

// NewLocalLayer returns an OCI layer descriptor for the corresponding local file.
func NewLocalFileLayer(path string, options ...LayerOption) *LocalFileLayer {
	layer := &LocalFileLayer{
		path: path,
	}
	for _, opt := range options {
		opt(&layer.options)
	}
	if layer.options.annotations == nil {
		layer.options.annotations = make(map[string]string)
	}
	if layer.options.annotations[imagespec.AnnotationTitle] == "" {
		layer.options.annotations[imagespec.AnnotationTitle] = filepath.Base(path)
	}
	return layer
}

// Digest returns the Hash of the local file.
func (l *LocalFileLayer) Digest() (_ v1.Hash, err error) {
	l.digestOnce.Do(func() {
		var f *os.File

		f, err = os.Open(l.path)
		if err != nil {
			return
		}
		defer f.Close()

		l.digest, _, err = v1.SHA256(f)
	})

	if l.digest.Hex == "" {
		if err != nil {
			return l.digest, fmt.Errorf("while computing layer digest: %w", err)
		}
		return l.digest, fmt.Errorf("unable to compute layer digest")
	}

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
	return types.MediaType(l.options.mediaType), nil
}

// Annotations returns annotations associated to this layer.
func (l *LocalFileLayer) Annotations() map[string]string {
	return l.options.annotations
}

// Platform returns platform information for this layer.
func (l *LocalFileLayer) Platform() *v1.Platform {
	return l.options.platform
}

// StreamLayer defines an OCI layer descriptor associated to a stream.
type StreamLayer struct {
	io.Reader
	stream io.Reader

	digest     v1.Hash
	hashBuffer *decompress.HashBuffer

	options LayerOptions
}

// NewStreamLayer returns an OCI layer descriptor for the corresponding stream.
func NewStreamLayer(stream io.Reader, options ...LayerOption) *StreamLayer {
	hb := decompress.NewHashBuffer(sha256.New(), nil)
	layer := &StreamLayer{
		Reader:     io.TeeReader(stream, hb),
		stream:     stream,
		hashBuffer: hb,
		digest: v1.Hash{
			Algorithm: "sha256",
		},
	}
	for _, opt := range options {
		opt(&layer.options)
	}
	if layer.options.annotations == nil {
		layer.options.annotations = make(map[string]string)
	}
	return layer
}

// Digest returns the Hash of the consumed stream.
func (sl *StreamLayer) Digest() (v1.Hash, error) {
	if sl.digest.Hex == "" {
		return sl.digest, stream.ErrNotComputed
	}
	return sl.digest, nil
}

// DiffID returns the Hash of the uncompressed layer (not supported by ORAS).
func (sl *StreamLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("not supported by ORAS")
}

// Compressed returns an io.ReadCloser for the file content.
func (sl *StreamLayer) Compressed() (io.ReadCloser, error) {
	return sl, nil
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents
// (not supported by ORAS).
func (sl *StreamLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported by ORAS")
}

// Size returns the size of the consumed stream.
func (sl *StreamLayer) Size() (int64, error) {
	return sl.hashBuffer.BytesRead(), nil
}

// MediaType returns the media type of the Layer.
func (sl *StreamLayer) MediaType() (types.MediaType, error) {
	return types.MediaType(sl.options.mediaType), nil
}

// Annotations returns annotations associated to this layer.
func (sl *StreamLayer) Annotations() map[string]string {
	return sl.options.annotations
}

// Platform returns platform information for this layer.
func (sl *StreamLayer) Platform() *v1.Platform {
	return sl.options.platform
}

// Close closes the underlying stream and computes the digest.
func (sl *StreamLayer) Close() error {
	sl.digest.Hex = sl.hashBuffer.Hex()

	if closer, ok := sl.stream.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}
