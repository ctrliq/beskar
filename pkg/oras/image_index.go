// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var _ v1.ImageIndex = &ImageIndex{}

// ImageIndex defines an image index.
type ImageIndex struct {
	m sync.RWMutex

	manifest *v1.IndexManifest
	images   map[string]*Image
}

// ImageIndexIndexFromReference returns an image index populated
// with the remote image reference to update an existing image index.
func ImageIndexFromReference(ref name.Reference, options ...remote.Option) (*ImageIndex, error) {
	index, err := remote.Index(ref, options...)
	if err != nil {
		return nil, fmt.Errorf("while getting image index from %s: %w", ref.Name(), err)
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("while getting %s index manifest: %w", ref.Name(), err)
	}

	return &ImageIndex{
		images:   make(map[string]*Image),
		manifest: manifest,
	}, nil
}

// NewImageIndex returns an image index.
func NewImageIndex() *ImageIndex {
	return &ImageIndex{
		images: make(map[string]*Image),
		manifest: &v1.IndexManifest{
			SchemaVersion: 2,
			MediaType:     types.OCIImageIndex,
			Manifests:     make([]v1.Descriptor, 0),
		},
	}
}

// AddImage adds an image to the image index.
func (i *ImageIndex) AddImage(image *Image) error {
	digest, err := image.Digest()
	if err != nil {
		return err
	}

	mt, _ := image.MediaType()
	size, err := image.Size()
	if err != nil {
		return err
	}

	manifest, err := image.Manifest()
	if err != nil {
		return err
	} else if len(manifest.Layers) == 0 {
		return fmt.Errorf("no layer associated to the image")
	}

	i.m.Lock()
	i.manifest.Manifests = append(i.manifest.Manifests, v1.Descriptor{
		MediaType:   mt,
		Size:        size,
		Digest:      digest,
		Annotations: manifest.Layers[0].Annotations,
		Platform:    manifest.Layers[0].Platform,
	})
	i.images[digest.String()] = image
	i.m.Unlock()

	return nil
}

// RemoveImage removes image manifest and layers if any from
// the current image index.
func (i *ImageIndex) RemoveImage(hash v1.Hash) {
	i.m.Lock()
	for idx, manifest := range i.manifest.Manifests {
		if manifest.Digest.String() == hash.String() {
			i.manifest.Manifests = append(i.manifest.Manifests[:idx], i.manifest.Manifests[idx+1:]...)
			break
		}
	}
	delete(i.images, hash.String())
	i.m.Unlock()
}

// MediaType of this image's manifest.
func (i *ImageIndex) MediaType() (types.MediaType, error) {
	return i.manifest.MediaType, nil
}

// Digest returns the sha256 of this index's manifest.
func (i *ImageIndex) Digest() (v1.Hash, error) {
	b, err := i.RawManifest()
	if err != nil {
		return v1.Hash{}, fmt.Errorf("failed to get image sha manifest: %w", err)
	}
	hash, _, err := v1.SHA256(bytes.NewReader(b))
	return hash, err
}

// RawManifest returns the serialized bytes of IndexManifest().
func (i *ImageIndex) RawManifest() ([]byte, error) {
	i.m.RLock()
	defer i.m.RUnlock()
	return json.Marshal(i.manifest)
}

// Size returns the size of the manifest.
func (i *ImageIndex) Size() (int64, error) {
	b, err := i.RawManifest()
	if err != nil {
		return 0, err
	}
	return int64(len(b)), nil
}

// IndexManifest returns this image index's manifest object.
func (i *ImageIndex) IndexManifest() (*v1.IndexManifest, error) {
	return i.manifest, nil
}

// Image returns a v1.Image that this ImageIndex references.
func (i *ImageIndex) Image(hash v1.Hash) (v1.Image, error) {
	i.m.RLock()
	defer i.m.RUnlock()

	image, ok := i.images[hash.String()]
	if ok {
		return image, nil
	}

	return nil, fmt.Errorf("no image found with digest %s", hash)
}

// ImageIndex returns a v1.ImageIndex that this ImageIndex references.
// Not supported for ORAS.
func (i *ImageIndex) ImageIndex(v1.Hash) (v1.ImageIndex, error) {
	return nil, fmt.Errorf("not supported by ORAS")
}
