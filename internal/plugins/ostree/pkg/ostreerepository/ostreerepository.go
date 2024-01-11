package ostreerepository

import (
	"context"
	"fmt"
	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"go.ciq.dev/beskar/internal/pkg/repository"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	if current != StateReady {
		return werror.Wrap(gcode.ErrUnavailable, fmt.Errorf("repository is not ready: %s", current))
	}
	h._state.Swap(int32(state))
	if state == StateSyncing || current == StateSyncing {
		h.updateSyncing(state == StateSyncing)
	}
	return nil
}

func (h *Handler) clearState() {
	h._state.Swap(int32(StateReady))
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
			select {
			case <-ctx.Done():
				h.Stopped.Store(true)
			}
		}
		h.cleanup()
	}()
}
