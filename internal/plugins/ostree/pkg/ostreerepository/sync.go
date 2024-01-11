package ostreerepository

import (
	"time"
)

type RepoSync struct {
	Syncing   bool   `db:"syncing"`
	StartTime int64  `db:"start_time"`
	EndTime   int64  `db:"end_time"`
	SyncError string `db:"sync_error"`
}

func (h *Handler) getRepoSync() *RepoSync {
	return h.repoSync.Load()
}

func (h *Handler) setRepoSync(repoSync *RepoSync) {
	rs := *repoSync
	h.repoSync.Store(&rs)
}

func (h *Handler) updateSyncing(syncing bool) *RepoSync {
	repoSync := *h.getRepoSync()
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
