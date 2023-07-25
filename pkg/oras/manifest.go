// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package oras

import (
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// HeadManifest returns the media type and digest of the manifest corresponding
// to the reference.
func HeadManifest(ref name.Reference, options ...remote.Option) (types.MediaType, string, error) {
	desc, err := remote.Head(ref, options...)
	if err != nil {
		return "", "", err
	}
	return desc.MediaType, desc.Digest.String(), nil
}

// CompatibleManifest returns if the media type corresponds to a compatible
// ORAS manifest.
func CompatibleManifest(mt types.MediaType) bool {
	return mt == types.OCIManifestSchema1 || mt == types.OCIImageIndex
}

// GetManifest returns the manifest corresponding to the reference.
func GetManifest(ref name.Reference, options ...remote.Option) (*v1.Manifest, error) {
	desc, err := remote.Get(ref, options...)
	if err != nil {
		return nil, err
	}
	manifest := new(v1.Manifest)
	return manifest, json.Unmarshal(desc.Manifest, manifest)
}
