// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	ErrNoImageIndexToPush = errors.New("no image index to push")
	ErrNoImageToPush      = errors.New("no image to push")
)

type Pusher interface {
	// Reference returns the image reference to pull.
	Reference() name.Reference
	ImageIndex() (v1.ImageIndex, error)
	Image() (v1.Image, error)
}

// Push images to a remote OCI registry.
func Push(pusher Pusher, options ...remote.Option) error {
	if pusher == nil {
		return fmt.Errorf("pusher is not specified")
	}

	ref := pusher.Reference()

	imageIndex, err := pusher.ImageIndex()
	if err != nil && !errors.Is(err, ErrNoImageIndexToPush) {
		return fmt.Errorf("while getting image index to push: %w", err)
	} else if err == nil {
		if err := remote.WriteIndex(ref, imageIndex, options...); err != nil {
			return fmt.Errorf("while pushing image to %s: %w", ref.Name(), err)
		}
		return nil
	}

	image, err := pusher.Image()
	if err != nil && !errors.Is(err, ErrNoImageToPush) {
		return fmt.Errorf("while getting image to push: %w", err)
	} else if err == nil {
		if err := remote.Write(ref, image, options...); err != nil {
			return fmt.Errorf("while pushing image to %s: %w", ref.Name(), err)
		}
		return nil
	}

	return fmt.Errorf("no image nor image index to push to %s", ref.Name())
}
