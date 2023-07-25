// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Puller defines the interface for pulling images.
type Puller interface {
	// Reference returns the image reference to pull.
	Reference() name.Reference
	// IndexManifest returns the image hash manifest to pull from.
	IndexManifest(*v1.IndexManifest) *v1.Hash
	// Config inspects manifest config descriptor.
	Config(v1.Descriptor) error
	// RawConfig parses config blob descriptor content.
	RawConfig([]byte) error
	// Layers downloads image blobs.
	Layers([]v1.Layer) error
}

// Pull pulls an image from a remote OCI registry.
func Pull(puller Puller, options ...remote.Option) error {
	if puller == nil {
		return fmt.Errorf("no puller specified")
	}

	ref := puller.Reference()

	rimg, err := remote.Image(ref, options...)
	if err != nil {
		return fmt.Errorf("while pulling image from %s: %w", ref.Name(), err)
	}

	man, err := rimg.Manifest()
	if err != nil {
		return fmt.Errorf("while getting %s image manifest: %w", ref.Name(), err)
	}

	if err := puller.Config(man.Config); err != nil {
		return err
	}

	config, err := rimg.RawConfigFile()
	if err != nil {
		return fmt.Errorf("while getting raw configuration bytes: %w", err)
	} else if err := puller.RawConfig(config); err != nil {
		return err
	}

	layers, err := rimg.Layers()
	if err != nil {
		return fmt.Errorf("while getting image layers: %w", err)
	}

	return puller.Layers(layers)
}

// PullIndex pulls an image index from a remote OCI registry.
func PullIndex(puller Puller, options ...remote.Option) error {
	if puller == nil {
		return fmt.Errorf("no puller specified")
	}

	ref := puller.Reference()

	index, err := remote.Index(ref, options...)
	if err != nil {
		return fmt.Errorf("while pulling index from %s: %w", ref.Name(), err)
	}

	indexMan, err := index.IndexManifest()
	if err != nil {
		return fmt.Errorf("while getting %s index manifest: %w", ref.Name(), err)
	}

	hash := puller.IndexManifest(indexMan)
	if hash == nil {
		return fmt.Errorf("no matching image manifest found for %s", ref.Name())
	}

	image, err := index.Image(*hash)
	if err != nil {
		return fmt.Errorf("while pulling image from %s: %w", ref.Name(), err)
	}

	man, err := image.Manifest()
	if err != nil {
		return fmt.Errorf("while getting %s image manifest: %w", ref.Name(), err)
	}

	if err := puller.Config(man.Config); err != nil {
		return err
	}

	config, err := image.RawConfigFile()
	if err != nil {
		return fmt.Errorf("while getting raw configuration bytes: %w", err)
	} else if err := puller.RawConfig(config); err != nil {
		return err
	}

	layers, err := image.Layers()
	if err != nil {
		return fmt.Errorf("while getting image layers: %w", err)
	}

	return puller.Layers(layers)
}
