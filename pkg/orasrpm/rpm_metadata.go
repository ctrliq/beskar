// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasrpm

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	RepomdConfigType            = "application/vnd.ciq.rpm.repomd.v1.config+json"
	RepomdXMLLayerType          = "application/vnd.ciq.rpm.repomd.v1.xml"
	RepomdXMLSignatureLayerType = "application/vnd.ciq.rpm.repomd.v1.xml.asc"
	RepomdDataConfigType        = "application/vnd.ciq.rpm.repomd.extra.v1.config+xml"
	RepomdDataLayerTypeFormat   = "application/vnd.ciq.rpm.repomd.extra.v1.%s"
)

func GetRepomdDataLayerType(dataType string) string {
	return fmt.Sprintf(RepomdDataLayerTypeFormat, dataType)
}

type RPMMetadata interface {
	Path() string
	Digest() (string, string)
	Size() int64
	Mediatype() string
	Annotations() map[string]string
}

var _ oras.Pusher = &RPMPusher{}

func NewRPMMetadataPusher(ref name.Reference, configMediatype types.MediaType, layers ...oras.Layer) *RPMMetadataPusher {
	return &RPMMetadataPusher{
		ref:             ref,
		layers:          layers,
		configMediatype: configMediatype,
	}
}

// RPMMetadataPusher type to push RPM metadata to registry.
type RPMMetadataPusher struct {
	ref             name.Reference
	layers          []oras.Layer
	configMediatype types.MediaType
}

func (rp *RPMMetadataPusher) Reference() name.Reference {
	return rp.ref
}

func (rp *RPMMetadataPusher) ImageIndex() (v1.ImageIndex, error) {
	return nil, oras.ErrNoImageIndexToPush
}

func (rp *RPMMetadataPusher) Image() (v1.Image, error) {
	img := oras.NewImage()

	if err := img.AddConfig(rp.configMediatype, nil); err != nil {
		return nil, fmt.Errorf("while adding RPM metadata config: %w", err)
	}

	for _, layer := range rp.layers {
		if err := img.AddLayer(layer); err != nil {
			return nil, fmt.Errorf("while adding RPM metadata layer: %w", err)
		}
	}

	return img, nil
}

func NewRPMExtraMetadataPusher(path, repo, dataType string, opts ...name.Option) (*RPMMetadataPusher, error) {
	if !strings.HasPrefix(repo, "artifacts/") {
		if !strings.HasPrefix(repo, "yum/") {
			repo = filepath.Join("artifacts", "yum", repo)
		} else {
			repo = filepath.Join("artifacts", repo)
		}
	}

	rawRef := filepath.Join(repo, "repodata:"+dataType)

	ref, err := name.ParseReference(rawRef, opts...)
	if err != nil {
		return nil, err
	}

	metadata, err := NewGenericRPMMetadata(path, GetRepomdDataLayerType(dataType), map[string]string{
		imagespec.AnnotationTitle: filepath.Base(path),
	})
	if err != nil {
		return nil, err
	}

	return &RPMMetadataPusher{
		ref:             ref,
		layers:          []oras.Layer{NewRPMMetadataLayer(metadata)},
		configMediatype: RepomdDataConfigType,
	}, nil
}

// RPMMetadataLayer defines an OCI layer descriptor associated to RPM repository metadatas.
type RPMMetadataLayer struct {
	repomd RPMMetadata
}

// NewRPMMetadataLayer returns an OCI layer descriptor for the corresponding
// RPM metadata.
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

// GenericRPMMetadata defines a generic RPM repository metadata.
type GenericRPMMetadata struct {
	path        string
	digest      string
	mediatype   string
	size        int64
	annotations map[string]string
}

// NewGenericRPMMetadata returns a generic RPM repository metadata.
func NewGenericRPMMetadata(path string, mediatype string, annotations map[string]string) (*GenericRPMMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	checksum := sha256.New()

	size, err := io.Copy(checksum, f)
	if err != nil {
		return nil, err
	}

	return &GenericRPMMetadata{
		path:        path,
		digest:      fmt.Sprintf("%x", checksum.Sum(nil)),
		mediatype:   mediatype,
		size:        size,
		annotations: annotations,
	}, nil
}

// Path returns the metadata file path.
func (g *GenericRPMMetadata) Path() string {
	return g.path
}

// Digest returns the metadata file digest.
func (g *GenericRPMMetadata) Digest() (string, string) {
	return "sha256", g.digest
}

// Size returns the metadata file size.
func (g *GenericRPMMetadata) Size() int64 {
	return g.size
}

// Mediatype returns the mediatype for this metadata.
func (g *GenericRPMMetadata) Mediatype() string {
	return g.mediatype
}

// Annotations returns optional annotations associated with
// the metadata file.
func (g *GenericRPMMetadata) Annotations() map[string]string {
	return g.annotations
}
