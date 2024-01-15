// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostreerepository

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/internal/pkg/repository"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
)

const (
	beskarRemoteName = "_beskar_"
)

type State int32

const (
	// StateStopped - The repository _state is unknown.
	StateStopped State = iota
	// StateReady - The repository is ready.
	StateReady
	// StateProvisioning - The repository is being provisioned.
	StateProvisioning
	// StateSyncing - The repository is being synced.
	StateSyncing
	// StateDeleting - The repository is being deleted.
	StateDeleting
)

func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateReady:
		return "ready"
	case StateProvisioning:
		return "provisioning"
	case StateSyncing:
		return "syncing"
	case StateDeleting:
		return "deleting"
	default:
		return "unknown"
	}
}

type Handler struct {
	*repository.RepoHandler
	logger   *slog.Logger
	repoDir  string
	repoLock sync.RWMutex
	repoSync atomic.Pointer[RepoSync]

	_state atomic.Int32
}

func NewHandler(logger *slog.Logger, repoHandler *repository.RepoHandler) *Handler {
	return &Handler{
		RepoHandler: repoHandler,
		repoDir:     filepath.Join(repoHandler.Params.Dir, repoHandler.Repository),
		logger:      logger,
	}
}

func (h *Handler) setState(state State) error {
	current := h.getState()
	if current != StateReady && state != StateReady {
		return werror.Wrap(gcode.ErrUnavailable, fmt.Errorf("repository is busy: %s", current))
	}
	h._state.Swap(int32(state))
	if state == StateSyncing || current == StateSyncing {
		_ = h.updateSyncing(state == StateSyncing)
	}
	return nil
}

func (h *Handler) clearState() {
	h._state.Swap(int32(StateReady))
	_ = h.updateSyncing(false)
}

func (h *Handler) getState() State {
	if !h.Started() {
		return StateStopped
	}
	return State(h._state.Load())
}

func (h *Handler) cleanup() {
	h.logger.Debug("repository cleanup", "repository", h.Repository)
	h.repoLock.Lock()
	defer h.repoLock.Unlock()

	close(h.Queued)
	h.Params.Remove(h.Repository)
	_ = os.RemoveAll(h.repoDir)
}

func (h *Handler) QueueEvent(_ *eventv1.EventPayload, _ bool) error {
	return nil
}

func (h *Handler) Start(ctx context.Context) {
	h.logger.Debug("starting repository", "repository", h.Repository)
	h.clearState()

	go func() {
		for !h.Stopped.Load() {
			//nolint: gosimple
			select {
			case <-ctx.Done():
				h.Stopped.Store(true)
			}
		}
		h.cleanup()
	}()
}

// pullConfig pulls the config file from beskar.
func (h *Handler) pullFile(ctx context.Context, filename string) error {
	// TODO: Replace with appropriate puller mechanism
	url := "http://" + h.Params.GetBeskarRegistryHostPort() + path.Join("/", h.Repository, "repo", filename)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check Content-Length
	if resp.ContentLength <= 0 {
		return ctl.Errf("content-length is 0")
	}

	// Create the file
	out, err := os.Create(path.Join(h.repoDir, filename))
	if err != nil {
		return err
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
