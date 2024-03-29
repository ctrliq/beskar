// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"context"
	"fmt"
	"io"
	"os"

	apiv1 "go.ciq.dev/beskar/pkg/plugins/mirror/api/v1"
	"go.ciq.dev/go-rsync/rsync"
)

func (h *Handler) repositorySync(_ context.Context) (errFn error) {
	sync := h.updateSyncing(true)

	defer func() {
		h.logger.Debug("sync artifact reset")

		h.SyncArtifactReset()

		h.logger.Debug("update syncing")

		sync = h.updateSyncing(false)

		if errFn != nil {
			sync.SyncError = errFn.Error()
		} else {
			sync.SyncError = ""
		}

		h.logger.Debug("update sync database")
		if err := h.updateSyncDatabase(dbCtx, sync); err != nil {
			if errFn == nil {
				errFn = err
			} else {
				h.logger.Error("sync database update failed", "error", err.Error())
			}
		}

		h.logger.Debug("defer work done")
	}()

	if err := h.updateSyncDatabase(dbCtx, sync); err != nil {
		return err
	}

	for i, config := range h.mirrorConfigs {
		switch config.URL.Scheme {
		case "rsync":
			if err := h.rsync(config, i); err != nil {
				return err
			}
		case "http", "https":
			if err := h.mirrorSync(config, i); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported scheme: %s", config.URL.Scheme)
		}
	}

	h.logger.Debug("generating index.html files")
	err := h.GenerateIndexes()
	if err != nil {
		return err
	}

	h.logger.Debug("index.html files generated")

	return nil
}

func copyTo(src io.Reader, dest string) error {
	pkg, err := os.Create(dest)
	if err != nil {
		return err
	}

	_, err = io.Copy(pkg, src)
	closeErr := pkg.Close()
	if err != nil {
		return err
	} else if closeErr != nil {
		return closeErr
	}

	return nil
}

func (h *Handler) getSyncPlan() (plan *apiv1.RepositorySyncPlan, err error) {
	for i, config := range h.mirrorConfigs {
		switch config.URL.Scheme {
		case "rsync":
			plan, err = h.rsyncPlan(config, i)
			if err != nil {
				return nil, err
			}
		case "http", "https":
			plan, err = h.mirrorSyncPlan(config, i)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported scheme: %s", config.URL.Scheme)
		}
	}

	return plan, nil
}

func (h *Handler) rsync(config mirrorConfig, configIndex int) error {
	addr, module, path, err := rsync.SplitURL(config.URL)
	if err != nil {
		return err
	}

	cOpts := []rsync.ClientOption{rsync.WithLogger(h.logger)}
	if len(config.Exclusions) > 0 {
		cOpts = append(cOpts, rsync.WithExclusionList(config.Exclusions))
	}

	if config.URL.User != nil {
		password, _ := config.URL.User.Password()
		cOpts = append(cOpts, rsync.WithClientAuth(config.URL.User.Username(), password))
	}

	s := NewStorage(h, config, uint64(configIndex))
	defer s.Close()

	ppath := rsync.TrimPrepath(path)
	client, err := rsync.SocketClient(s, addr, module, ppath, cOpts...)
	if err != nil {
		return err
	}

	if config.HTTPURL != nil {
		sp, err := client.GetSyncPlan()
		if err != nil {
			return err
		}

		ps := NewPlanSyncer(h, config, uint64(configIndex), h.Params.Sync.MaxWorkerCount, sp)
		if err := ps.Sync(); err != nil {
			return err
		}
	} else {
		if err := client.Sync(); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) rsyncPlan(config mirrorConfig, configIndex int) (*apiv1.RepositorySyncPlan, error) {
	addr, module, path, err := rsync.SplitURL(config.URL)
	if err != nil {
		return nil, err
	}

	cOpts := []rsync.ClientOption{rsync.WithLogger(h.logger)}
	if len(config.Exclusions) > 0 {
		cOpts = append(cOpts, rsync.WithExclusionList(config.Exclusions))
	}

	if config.URL.User != nil {
		password, _ := config.URL.User.Password()
		cOpts = append(cOpts, rsync.WithClientAuth(config.URL.User.Username(), password))
	}

	s := NewStorage(h, config, uint64(configIndex))
	defer s.Close()

	ppath := rsync.TrimPrepath(path)
	client, err := rsync.SocketClient(s, addr, module, ppath, cOpts...)
	if err != nil {
		return nil, err
	}

	sp, err := client.GetSyncPlan()
	if err != nil {
		return nil, err
	}

	var plan apiv1.RepositorySyncPlan
	for _, f := range sp.AddRemoteFiles {
		plan.Add = append(plan.Add, string(sp.RemoteFiles[f].Path))
	}

	for _, f := range sp.DeleteLocalFiles {
		plan.Remove = append(plan.Remove, string(sp.LocalFiles[f].Path))
	}

	return &plan, nil
}

func (h *Handler) mirrorSync(config mirrorConfig, configIndex int) error {
	syncer, err := NewMirrorSyncer(h, config, uint64(configIndex), h.Params.Sync.MaxWorkerCount)
	if err != nil {
		return err
	}

	return syncer.Sync()
}

func (h *Handler) mirrorSyncPlan(config mirrorConfig, configIndex int) (*apiv1.RepositorySyncPlan, error) {
	syncer, err := NewMirrorSyncer(h, config, uint64(configIndex), h.Params.Sync.MaxWorkerCount)
	if err != nil {
		return nil, err
	}

	p, err := syncer.Plan()
	if err != nil {
		return nil, err
	}

	var plan apiv1.RepositorySyncPlan
	for _, f := range p.AddRemoteFiles {
		plan.Add = append(plan.Add, f.Name)
	}

	for _, f := range p.DeleteLocalFiles {
		plan.Remove = append(plan.Remove, f.Name)
	}

	return &plan, nil
}
