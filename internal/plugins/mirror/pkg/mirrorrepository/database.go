// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
)

func (h *Handler) generateFileReference(file string) string {
	fileReferece := filepath.Join(h.Repository, file)

	if !strings.HasPrefix(fileReferece, "artifacts/") {
		if !strings.HasPrefix(fileReferece, "mirror/") {
			fileReferece = filepath.Join("artifacts", "mirror", fileReferece)
		} else {
			fileReferece = filepath.Join("artifacts", fileReferece)
		}
	}

	return fileReferece
}

func (h *Handler) updateSyncDatabase(ctx context.Context, sync *mirrordb.Sync) error {
	db, err := h.getStatusDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(false)

	err = db.UpdateSync(ctx, sync)
	if err != nil {
		return fmt.Errorf("while adding extra metadata to database: %w", err)
	}

	return db.Sync(ctx)
}

func (h *Handler) addFileToRepositoryDatabase(ctx context.Context, pkg *mirrordb.RepositoryFile) error {
	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(false)

	err = db.AddFile(ctx, pkg)
	if err != nil {
		return fmt.Errorf("while adding repository file to database: %w", err)
	}

	return db.Sync(ctx)
}

func (h *Handler) removeFileFromRepositoryDatabase(ctx context.Context, name string) error {
	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(false)

	//nolint:gosec
	s := md5.Sum([]byte(name))
	tag := hex.EncodeToString(s[:])

	deleted, err := db.RemoveFile(ctx, tag)
	if err != nil {
		return fmt.Errorf("while removing repository file from database: %w", err)
	} else if deleted {
		return db.Sync(ctx)
	}

	return nil
}

func (h *Handler) getRepositoryFile(ctx context.Context, file string) (repositoryFile *mirrordb.RepositoryFile, err error) {
	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	fileDB, err := db.GetFileByName(ctx, file)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return fileDB, nil
}

func (h *Handler) listRepositoryFilesByParent(ctx context.Context, parent string) (repositoryFiles []*mirrordb.RepositoryFile, err error) {
	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	err = db.WalkFilesByParent(ctx, parent, func(file *mirrordb.RepositoryFile) error {
		repositoryFiles = append(repositoryFiles, file)
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return repositoryFiles, nil
}

func (h *Handler) listRepositoryFilesByConfigID(ctx context.Context, configID uint64) (repositoryFiles []*mirrordb.RepositoryFile, err error) {
	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	err = db.WalkFilesByConfigID(ctx, configID, func(file *mirrordb.RepositoryFile) error {
		repositoryFiles = append(repositoryFiles, file)
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return repositoryFiles, nil
}

func (h *Handler) listRepositoryDistinctParents(ctx context.Context) (repositoryParents []string, err error) {
	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	err = db.WalkFilesByDistinctParent(ctx, func(parent *string) error {
		repositoryParents = append(repositoryParents, *parent)
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return repositoryParents, nil
}
