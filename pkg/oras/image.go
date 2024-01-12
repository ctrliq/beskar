// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Layer defines the image layer interface.
type Layer interface {
	v1.Layer
	Annotations() map[string]string
	Platform() *v1.Platform
}

var _ v1.Image = &Image{}

// Image defines an ORAS artifact manifest with the associated
// images layers.
type Image struct {
	m sync.RWMutex

	streamLayers []*StreamLayer

	layers    map[string]v1.Layer
	manifest  *v1.Manifest
	rawConfig []byte
}

// NewImage returns a Image instance for uploading artifacts
// to OCI registries.
func NewImage() *Image {
	return &Image{
		layers: make(map[string]v1.Layer),
		manifest: &v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.OCIManifestSchema1,
			Layers:        make([]v1.Descriptor, 0),
		},
	}
}

func (i *Image) AddConfig(mt types.MediaType, rawConfig []byte) error {
	h, size, err := v1.SHA256(bytes.NewReader(rawConfig))
	if err != nil {
		return err
	}
	i.manifest.Config = v1.Descriptor{
		MediaType: mt,
		Size:      size,
		Digest:    h,
	}
	i.rawConfig = rawConfig
	return nil
}

// AddLayer adds a blob layer to the image manifest.
func (i *Image) AddLayer(layer Layer) error {
	i.m.Lock()
	defer i.m.Unlock()

	if sl, ok := layer.(*StreamLayer); ok {
		i.streamLayers = append(i.streamLayers, sl)
		return nil
	}

	return i.addLayer(layer)
}

// addLayer adds the layer to the image manifest, must
// be wrapped in a write lock section.
func (i *Image) addLayer(layer Layer) error {
	digest, err := layer.Digest()
	if err != nil {
		return err
	}

	mt, _ := layer.MediaType()
	size, err := layer.Size()
	if err != nil {
		return err
	}

	i.layers[digest.String()] = layer
	i.manifest.Layers = append(i.manifest.Layers, v1.Descriptor{
		MediaType:   mt,
		Size:        size,
		Digest:      digest,
		Annotations: layer.Annotations(),
		Platform:    layer.Platform(),
	})

	return nil
}

// Layers returns a unordered collection of SIF file.
func (i *Image) Layers() ([]v1.Layer, error) {
	i.m.RLock()
	defer i.m.RUnlock()

	layers := make([]v1.Layer, 0, len(i.layers)+len(i.streamLayers))

	// TODO: consider layer ordering
	for _, sl := range i.layers {
		layers = append(layers, sl)
	}
	for _, sl := range i.streamLayers {
		layers = append(layers, sl)
	}

	return layers, nil
}

// Size returns the size of the manifest.
func (i *Image) Size() (int64, error) {
	b, err := i.RawManifest()
	if err != nil {
		return 0, err
	}
	return int64(len(b)), nil
}

// ConfigName returns the hash of the image's config file, also known as
// the Image ID.
func (i *Image) ConfigName() (v1.Hash, error) {
	return i.manifest.Config.Digest, nil
}

// MediaType of this image's manifest.
func (i *Image) MediaType() (types.MediaType, error) {
	return i.manifest.MediaType, nil
}

// RawConfigFile returns the serialized bytes of ConfigFile().
func (i *Image) RawConfigFile() ([]byte, error) {
	return i.rawConfig, nil
}

// ConfigFile returns this image's config file.
func (i *Image) ConfigFile() (*v1.ConfigFile, error) {
	return nil, fmt.Errorf("not supported by ORAS")
}

// Digest returns the sha256 of this image's manifest.
func (i *Image) Digest() (v1.Hash, error) {
	notComputed := false

	i.m.Lock()

	for j := len(i.streamLayers) - 1; j >= 0; j-- {
		if i.streamLayers[j].digest.Hex == "" {
			notComputed = true
		} else {
			if err := i.addLayer(i.streamLayers[j]); err != nil {
				i.m.Unlock()
				return v1.Hash{}, err
			}
			i.streamLayers = append(i.streamLayers[:j], i.streamLayers[j+1:]...)
		}
	}

	i.m.Unlock()

	if notComputed {
		return v1.Hash{}, stream.ErrNotComputed
	}

	b, err := i.RawManifest()
	if err != nil {
		return v1.Hash{}, fmt.Errorf("failed to get image sha manifest: %w", err)
	}
	hash, _, err := v1.SHA256(bytes.NewReader(b))
	return hash, err
}

// Manifest returns this image's Manifest object.
func (i *Image) Manifest() (*v1.Manifest, error) {
	i.m.RLock()
	defer i.m.RUnlock()

	return i.manifest, nil
}

// RawManifest returns the serialized bytes of Manifest().
func (i *Image) RawManifest() ([]byte, error) {
	i.m.RLock()
	defer i.m.RUnlock()

	return json.Marshal(i.manifest)
}

// LayerByDigest returns a Layer for interacting with a particular layer of
// the image, looking it up by "digest" (the compressed hash).
func (i *Image) LayerByDigest(hash v1.Hash) (v1.Layer, error) {
	i.m.RLock()
	defer i.m.RUnlock()

	l, ok := i.layers[hash.String()]
	if ok {
		return l, nil
	}

	return nil, fmt.Errorf("no layer found for digest %s", hash)
}

// LayerByDiffID is an analog to LayerByDigest, looking up by "diff id"
// (not supported by ORAS).
func (i *Image) LayerByDiffID(v1.Hash) (v1.Layer, error) {
	return nil, fmt.Errorf("not supported by ORAS")
}
