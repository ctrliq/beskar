// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"

	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
)

func (h *Handler) addPackageToMetadataDatabase(ctx context.Context, pkg *yumdb.PackageMetadata) error {
	db, err := h.getMetadataDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(false)

	err = db.AddPackage(ctx, pkg)
	if err != nil {
		return fmt.Errorf("while adding package metadata to database: %w", err)
	}

	return db.Sync(ctx)
}

func (h *Handler) addPackageToRepositoryDatabase(ctx context.Context, pkg *yumdb.RepositoryPackage) error {
	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(false)

	err = db.AddPackage(ctx, pkg)
	if err != nil {
		return fmt.Errorf("while adding repository package to database: %w", err)
	}

	return db.Sync(ctx)
}

func (h *Handler) addExtraMetadataDatabase(ctx context.Context, extraMetadata *yumdb.ExtraMetadata) error {
	db, err := h.getMetadataDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(false)

	err = db.AddExtraMetadata(ctx, extraMetadata)
	if err != nil {
		return fmt.Errorf("while adding extra metadata to database: %w", err)
	}

	return db.Sync(ctx)
}

//nolint:unparam
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

func (h *Handler) updateReposyncDatabase(ctx context.Context, reposync *yumdb.Reposync) error {
	db, err := h.getStatusDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(false)

	err = db.UpdateReposync(ctx, reposync)
	if err != nil {
		return fmt.Errorf("while adding extra metadata to database: %w", err)
	}

	return db.Sync(ctx)
}

func (h *Handler) addSyncedPackageReposyncDatabase(ctx context.Context, syncedPackages, totalPackages int) (*yumdb.Reposync, error) {
	db, err := h.getStatusDB(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close(false)

	reposync := h.getReposync()
	reposync.TotalPackages = totalPackages
	reposync.SyncedPackages = syncedPackages

	err = db.UpdateReposync(ctx, reposync)
	if err != nil {
		return nil, fmt.Errorf("while adding extra metadata to database: %w", err)
	} else if err := db.Sync(ctx); err != nil {
		return nil, err
	}

	return reposync, nil
}
