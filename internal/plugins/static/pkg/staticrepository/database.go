// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package staticrepository

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"

	"go.ciq.dev/beskar/internal/plugins/static/pkg/staticdb"
)

func (h *Handler) logDatabase(ctx context.Context, level string, format string, args ...any) {
	db, err := h.getLogDB(ctx)
	if err != nil {
		h.logger.Error("getting log database", "error", err.Error())
		return
	}
	defer db.Close(false)

	err = db.AddLog(ctx, level, fmt.Sprintf(format, args...))
	if err != nil {
		h.logger.Error("adding log entry to database", "error", err.Error())
		return
	} else if err := db.Sync(ctx); err != nil {
		h.logger.Error("syncing log database", "error", err.Error())
	}
}

func (h *Handler) addFileToRepositoryDatabase(ctx context.Context, pkg *staticdb.RepositoryFile) error {
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
