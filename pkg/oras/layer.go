// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
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
