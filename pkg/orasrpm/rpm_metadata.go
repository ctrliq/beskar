// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasrpm

import (
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	RepomdConfigType         = "application/vnd.ciq.rpm.repomd.v1.config+json"
	RepomdXMLLayerType       = "application/vnd.ciq.rpm.repomd.v1.xml"
	OtherXMLLayerType        = "application/vnd.ciq.rpm.other.v1.xml+gzip"
	OtherSQLiteLayerType     = "application/vnd.ciq.rpm.other.sqlite.v1.gzip"
	PrimaryXMLLayerType      = "application/vnd.ciq.rpm.primary.v1.xml+gzip"
	PrimarySQLiteLayerType   = "application/vnd.ciq.rpm.primary.sqlite.v1.gzip"
	FilelistsXMLLayerType    = "application/vnd.ciq.rpm.filelists.v1.xml+gzip"
	FilelistsSQLiteLayerType = "application/vnd.ciq.rpm.filelists.sqlite.v1.gzip"
)

type RPMMetadata interface {
	Path() string
	Digest() (string, string)
	Size() int64
	Mediatype() string
	Annotations() map[string]string
}

var _ oras.Pusher = &RPMPusher{}

func NewRPMMetadataPusher(ref name.Reference, layers ...oras.Layer) *RPMMetadataPusher {
	return &RPMMetadataPusher{
		ref:    ref,
		layers: layers,
	}
}

// RPMMetadataPusher type to push RPM metadata to registry.
type RPMMetadataPusher struct {
	ref    name.Reference
	layers []oras.Layer
}

func (rp *RPMMetadataPusher) Reference() name.Reference {
	return rp.ref
}

func (rp *RPMMetadataPusher) ImageIndex() (v1.ImageIndex, error) {
	return nil, oras.ErrNoImageIndexToPush
}

func (rp *RPMMetadataPusher) Image() (v1.Image, error) {
	img := oras.NewImage()

	if err := img.AddConfig(RepomdConfigType, nil); err != nil {
		return nil, fmt.Errorf("while adding RPM repomd config: %w", err)
	}

	for _, layer := range rp.layers {
		if err := img.AddLayer(layer); err != nil {
			return nil, fmt.Errorf("while adding RPM metadata layer: %w", err)
		}
	}

	return img, nil
}

// RPMMetadataLayer defines an OCI layer descriptor associated to RPM repository metadatas.
type RPMMetadataLayer struct {
	repomd RPMMetadata
}

// NewRPMMetadataLayer returns an OCI layer descriptor for the corresponding
// RPM package.
func NewRPMMetadataLayer(repomd RPMMetadata) *RPMMetadataLayer {
	return &RPMMetadataLayer{
		repomd: repomd,
	}
}

// Digest returns the Hash of the RPM package.
func (l *RPMMetadataLayer) Digest() (v1.Hash, error) {
	alg, hex := l.repomd.Digest()
	return v1.Hash{
		Algorithm: alg,
		Hex:       hex,
	}, nil
}

// DiffID returns the Hash of the uncompressed layer (not supported by ORAS).
func (l *RPMMetadataLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("not supported by ORAS")
}

// Compressed returns an io.ReadCloser for the RPM file content.
func (l *RPMMetadataLayer) Compressed() (io.ReadCloser, error) {
	f, err := os.Open(l.repomd.Path())
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents
// (not supported by ORAS).
func (l *RPMMetadataLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported by ORAS")
}

// Size returns the size of the RPM file.
func (l *RPMMetadataLayer) Size() (int64, error) {
	return l.repomd.Size(), nil
}

// MediaType returns the media type of the Layer.
func (l *RPMMetadataLayer) MediaType() (types.MediaType, error) {
	return types.MediaType(l.repomd.Mediatype()), nil
}

// Annotations returns annotations associated to this layer.
func (l *RPMMetadataLayer) Annotations() map[string]string {
	return l.repomd.Annotations()
}

// Platform returns platform information for this layer.
func (l *RPMMetadataLayer) Platform() *v1.Platform {
	return nil
}
