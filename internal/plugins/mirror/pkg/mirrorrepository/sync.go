// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"context"

	"github.com/antoniomika/go-rsync/rsync"
)

func (h *Handler) repositorySync(_ context.Context) (errFn error) {
	sync := h.updateSyncing(true)

	defer func() {
		h.SyncArtifactReset()

		sync = h.updateSyncing(false)

		if errFn != nil {
			sync.SyncError = errFn.Error()
		} else {
			sync.SyncError = ""
		}
		if err := h.updateSyncDatabase(dbCtx, sync); err != nil {
			if errFn == nil {
				errFn = err
			} else {
				h.logger.Error("sync database update failed", "error", err.Error())
			}
		}
	}()

	if err := h.updateSyncDatabase(dbCtx, sync); err != nil {
		return err
	}

	addr, module, path, err := rsync.SplitURI(h.mirrorURLs[0].String())
	if err != nil {
		return err
	}

	ppath := rsync.TrimPrepath(path)
	client, err := rsync.SocketClient(h, addr, module, ppath, nil)
	if err != nil {
		return err
	}

	if err := client.Sync(); err != nil {
		return err
	}

	fileList, err := h.List()
	if err != nil {
		return err
	}

	err = h.GenerateIndexes(fileList)
	if err != nil {
		return err
	}

	return nil
}
