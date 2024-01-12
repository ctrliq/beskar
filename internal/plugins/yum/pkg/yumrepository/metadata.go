// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumrepository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/pkg/decompress"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
)

func (h *Handler) processMetadataManifest(ctx context.Context, metadataManifest *v1.Manifest, manifestDigest string) (errFn error) {
	defer func() {
		if errFn != nil {
			ref := filepath.Join(h.Repository, "repodata@"+manifestDigest)
			if err := h.DeleteManifest(ref); err != nil {
				h.logger.Error("delete metadata manifest", "digest", manifestDigest, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "delete metadata manifest %s: %s", manifestDigest, err)
			}
		}
	}()

	dataType := ""

	packageLayer, err := oras.GetLayerFilter(metadataManifest, func(mediatype types.MediaType) bool {
		n, err := fmt.Sscanf(string(mediatype), orasrpm.RepomdDataLayerTypeFormat, &dataType)
		if n == 0 || err != nil {
			return false
		}
		return true
	})
	if err != nil {
		h.logger.Error("process metadata manifest layers", "error", err.Error())
		return err
	}

	ref := filepath.Join(h.Repository, "repodata@"+packageLayer.Digest.String())

	metadataFilename := packageLayer.Annotations[imagespec.AnnotationTitle]
	metadataPath := filepath.Join(h.downloadDir(), metadataFilename)

	defer func() {
		h.SyncArtifactResult(metadataFilename, errFn)

		if errFn != nil {
			h.logger.Error("process metadata manifest", "metadata", metadataFilename, "error", errFn.Error())
			h.logDatabase(ctx, yumdb.LogError, "process metadata %s manifest: %s", metadataFilename, errFn)
		}
	}()

	if err := h.DownloadBlob(ref, metadataPath); err != nil {
		return fmt.Errorf("while downloading metadata %s: %w", metadataFilename, err)
	}
	defer os.Remove(metadataPath)

	data := new(bytes.Buffer)

	compressedHashBuffer := decompress.NewHashBuffer(sha256.New(), data)
	openedHashBuffer := decompress.NewHashBuffer(sha256.New(), nil)

	rc, err := decompress.File(
		metadataPath,
		decompress.WithHash(compressedHashBuffer),
		decompress.WithOpenHash(openedHashBuffer),
	)
	if err != nil {
		return fmt.Errorf("while reading %s extra metadata: %w", dataType, err)
	}

	_, err = io.Copy(io.Discard, rc)
	if err != nil {
		return fmt.Errorf("while computing %s extra metadata checksums: %w", dataType, err)
	}

	extraMetadata := &yumdb.ExtraMetadata{
		Type:      dataType,
		Filename:  metadataFilename,
		Checksum:  compressedHashBuffer.Hex(),
		Size:      uint64(compressedHashBuffer.BytesRead()),
		Timestamp: time.Now().UTC().Unix(),
		Data:      data.Bytes(),
		OpenSize:  uint64(openedHashBuffer.BytesRead()),
	}
	if extraMetadata.OpenSize > 0 {
		extraMetadata.OpenChecksum = openedHashBuffer.Hex()
	}

	return h.addExtraMetadataToDatabase(ctx, extraMetadata)
}

func (h *Handler) deleteMetadataManifest(ctx context.Context, metadataManifest *v1.Manifest) (errFn error) {
	dataType := ""

	packageLayer, err := oras.GetLayerFilter(metadataManifest, func(mediatype types.MediaType) bool {
		n, err := fmt.Sscanf(string(mediatype), orasrpm.RepomdDataLayerTypeFormat, &dataType)
		if n == 0 || err != nil {
			return false
		}
		return true
	})
	if err != nil {
		h.logger.Error("process metadata manifest layers", "error", err.Error())
		return err
	}

	metadataFilename := packageLayer.Annotations[imagespec.AnnotationTitle]

	defer func() {
		h.SyncArtifactResult(metadataFilename, errFn)

		if errFn != nil {
			h.logger.Error("process metadata manifest removal", "metadata", metadataFilename, "error", errFn.Error())
			h.logDatabase(ctx, yumdb.LogError, "process metadata %s manifest removal: %s", metadataFilename, errFn)
		}
	}()

	return h.removeExtraMetadataFromDatabase(ctx, dataType)
}
