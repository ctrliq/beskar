package ostreerepository

import (
	"time"
)

type RepoSync struct {
	Syncing   bool
	StartTime int64
	EndTime   int64
	SyncError string
}

func (h *Handler) setRepoSync(repoSync *RepoSync) {
	rs := *repoSync
	h.repoSync.Store(&rs)
}

func (h *Handler) updateSyncing(syncing bool) *RepoSync {
	if h.repoSync.Load() == nil {
		h.repoSync.Store(&RepoSync{})
	}

	repoSync := *h.repoSync.Load()
	previousSyncing := repoSync.Syncing
	repoSync.Syncing = syncing
	if syncing && !previousSyncing {
		repoSync.StartTime = time.Now().UTC().Unix()
		repoSync.SyncError = ""
	} else if !syncing && previousSyncing {
		repoSync.EndTime = time.Now().UTC().Unix()
	}
	h.repoSync.Store(&repoSync)
	return h.repoSync.Load()
}
