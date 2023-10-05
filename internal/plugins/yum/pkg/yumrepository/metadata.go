// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
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
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/mirror"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/pkg/decompress"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
)

func (h *Handler) processMetadataManifest(ctx context.Context, metadataManifest *v1.Manifest, sem *mirror.Semaphore) (errFn error) {
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

	ref := filepath.Join(h.Repository, "repodata@sha256:"+packageLayer.Digest.Hex)

	metadataFilename := packageLayer.Annotations[imagespec.AnnotationTitle]
	metadataPath := filepath.Join(h.downloadDir(), metadataFilename)

	defer func() {
		if h.syncing.Load() {
			sem.Release(1)
		}
		if errFn == nil {
			return
		}
		h.logger.Error("process metadata manifest", "error", errFn.Error())
		h.logDatabase(ctx, yumdb.LogError, "process metadata %s manifest: %s", metadataPath, err)
		// TODO: remove metadata
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

	idx := strings.IndexByte(metadataFilename, '.')
	if idx >= 0 {
		metadataFilename = fmt.Sprintf("%s%s", dataType, metadataFilename[idx:])
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

	return h.addExtraMetadataDatabase(ctx, extraMetadata)
}

func (h *Handler) deleteMetadataManifest(_ context.Context, _ *v1.Manifest) error {
	// TODO: implement this
	return fmt.Errorf("not supported yet")
}
