// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package staticrepository

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/plugins/static/pkg/staticdb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasfile"
)

func (h *Handler) processFileManifest(ctx context.Context, fileManifest *v1.Manifest) (errFn error) {
	fileLayer, err := oras.GetLayer(fileManifest, orasfile.StaticFileLayerType)
	if err != nil {
		return err
	}
	ref := filepath.Join(h.Repository, "files@sha256:"+fileLayer.Digest.Hex)

	fileName := fileLayer.Annotations[imagespec.AnnotationTitle]

	defer func() {
		h.SyncArtifactResult(fileName, errFn)
		if errFn == nil {
			return
		}
		h.logger.Error("process file manifest", "filename", fileName, "error", errFn.Error())
		h.logDatabase(ctx, staticdb.LogError, "process file %s manifest: %s", fileName, errFn)

		if err := h.DeleteManifest(ref); err != nil {
			h.logger.Error("delete file manifest", "filename", fileName, "error", err.Error())
			h.logDatabase(ctx, staticdb.LogError, "delete file %s manifest: %s", fileName, err)
		}
	}()

	//nolint:gosec
	s := md5.Sum([]byte(fileName))
	tag := hex.EncodeToString(s[:])

	repositoryFile := &staticdb.RepositoryFile{
		Tag:        tag,
		ID:         fileLayer.Digest.Hex,
		Name:       fileName,
		UploadTime: time.Now().UTC().Unix(),
		Size:       uint64(fileLayer.Size),
	}
	err = h.addFileToRepositoryDatabase(ctx, repositoryFile)
	if err != nil {
		return fmt.Errorf("while adding file %s to repository database: %w", fileName, err)
	}

	return nil
}

func (h *Handler) deleteFileManifest(ctx context.Context, fileManifest *v1.Manifest) (errFn error) {
	fileLayer, err := oras.GetLayer(fileManifest, orasfile.StaticFileLayerType)
	if err != nil {
		return err
	}
	fileName := fileLayer.Annotations[imagespec.AnnotationTitle]

	defer func() {
		h.SyncArtifactResult(fileName, errFn)
		if errFn == nil {
			return
		}
		h.logger.Error("process file manifest removal", "filename", fileName, "error", errFn.Error())
		h.logDatabase(ctx, staticdb.LogError, "process file %s manifest removal: %s", fileName, errFn)
	}()

	err = h.removeFileFromRepositoryDatabase(ctx, fileName)
	if err != nil {
		return fmt.Errorf("while removing file %s from metadata database: %w", fileName, err)
	}

	return nil
}
