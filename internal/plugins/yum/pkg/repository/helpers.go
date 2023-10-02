// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type mediaTypeFilter func(types.MediaType) bool

func getLayer(manifest *v1.Manifest, mediaType types.MediaType) (v1.Descriptor, error) {
	for _, layer := range manifest.Layers {
		if layer.MediaType != mediaType {
			continue
		}
		return layer, nil
	}

	return v1.Descriptor{}, fmt.Errorf("no layer with mediatype %s found in manifest", mediaType)
}

func getLayerFilter(manifest *v1.Manifest, mediaTypeFilter mediaTypeFilter) (v1.Descriptor, error) {
	for _, layer := range manifest.Layers {
		if mediaTypeFilter(layer.MediaType) {
			return layer, nil
		}
	}

	return v1.Descriptor{}, fmt.Errorf("no recognized layer found in manifest")
}

func downloadBlob(ref string, destinationPath string, params *HandlerParams) (errFn error) {
	downloadDir := filepath.Dir(destinationPath)
	if err := os.MkdirAll(downloadDir, 0o700); err != nil {
		return err
	} else if _, err := os.Stat(destinationPath); err == nil {
		return nil
	}

	dst, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		err = dst.Close()
		if errFn == nil {
			errFn = err
		}
	}()

	digest, err := name.NewDigest(ref, params.NameOptions...)
	if err != nil {
		return err
	}
	layer, err := remote.Layer(digest, params.RemoteOptions...)
	if err != nil {
		return err
	}
	rc, err := layer.Compressed()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(dst, rc)
	return err
}
