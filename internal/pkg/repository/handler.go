// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"gocloud.dev/blob"
)

type HandlerParams struct {
	Dir           string
	Bucket        *blob.Bucket
	RemoteOptions []remote.Option
	NameOptions   []name.Option
	remove        func(string)
}

func (hp HandlerParams) Remove(repository string) {
	hp.remove(repository)
}

// Handler - Interface for handling events for a repository.
type Handler interface {
	// QueueEvent - Called when a new event is received. If store is true, the event should be stored in the database.
	// Note: Avoid performing any long-running operations in this function.
	QueueEvent(event *eventv1.EventPayload, store bool) error

	// Started - Returns true if the handler has started.
	Started() bool

	// Start - Called when the handler should start processing events.
	// This is your chance to set up any resources, e.g., database connections, run loops, etc.
	// This will only be called once.
	Start(context.Context)

	// Stop - Called when the handler should stop processing events and clean up resources.
	Stop()
}

// RepoHandler - A partial default implementation of the Handler interface that provides some common functionality.
// You can embed this in your own handler to get some default functionality, e.g., an event queue.
type RepoHandler struct {
	Repository string
	Params     *HandlerParams

	queue      []*eventv1.EventPayload
	queueMutex sync.RWMutex
	Queued     chan struct{}

	Stopped   atomic.Bool
	startedCh chan struct{}

	cancel context.CancelFunc

	syncArtifactsMutex sync.RWMutex
	syncArtifacts      map[string]chan error
}

func NewRepoHandler(repository string, params *HandlerParams, cancel context.CancelFunc) *RepoHandler {
	return &RepoHandler{
		Repository:    repository,
		Params:        params,
		Queued:        make(chan struct{}, 1),
		startedCh:     make(chan struct{}),
		cancel:        cancel,
		syncArtifacts: make(map[string]chan error),
	}
}

func (rh *RepoHandler) EnqueueEvent(event *eventv1.EventPayload) {
	rh.queueMutex.Lock()
	rh.queue = append(rh.queue, event)
	rh.queueMutex.Unlock()

	rh.EventQueueUpdate()
}

func (rh *RepoHandler) DequeueEvents() []*eventv1.EventPayload {
	rh.queueMutex.Lock()
	events := rh.queue
	rh.queue = nil
	rh.queueMutex.Unlock()

	return events
}

func (rh *RepoHandler) EventQueueLength() int {
	rh.queueMutex.RLock()
	defer rh.queueMutex.RUnlock()
	return len(rh.queue)
}

func (rh *RepoHandler) EventQueueUpdate() {
	select {
	case rh.Queued <- struct{}{}:
	default:
	}
}

func (rh *RepoHandler) Started() bool {
	// closed when started
	<-rh.startedCh
	if rh.Stopped.Load() {
		<-rh.Queued
		return false
	}
	return true
}

func (rh *RepoHandler) Stop() {
	rh.Stopped.Store(true)
	rh.cancel()
	<-rh.Queued
}

func (rh *RepoHandler) DownloadBlob(ref string, destinationPath string) (errFn error) {
	downloadDir := filepath.Dir(destinationPath)
	if err := os.MkdirAll(downloadDir, 0o700); err != nil {
		return err
	} else if _, err := os.Stat(destinationPath); err == nil {
		return nil
	}

	dst, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		err = dst.Close()
		if errFn == nil {
			errFn = err
		}
	}()

	digest, err := name.NewDigest(ref, rh.Params.NameOptions...)
	if err != nil {
		return err
	}
	layer, err := remote.Layer(digest, rh.Params.RemoteOptions...)
	if err != nil {
		return err
	}
	rc, err := layer.Compressed()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(dst, rc)
	return err
}

func (rh *RepoHandler) GetManifestDigest(ref string) (string, error) {
	namedRef, err := name.ParseReference(ref, rh.Params.NameOptions...)
	if err != nil {
		return "", err
	}
	desc, err := remote.Head(namedRef, rh.Params.RemoteOptions...)
	if err != nil {
		return "", err
	}
	return desc.Digest.String(), nil
}

func (rh *RepoHandler) DeleteManifest(ref string) (errFn error) {
	namedRef, err := name.ParseReference(ref, rh.Params.NameOptions...)
	if err != nil {
		return err
	}
	return remote.Delete(namedRef, rh.Params.RemoteOptions...)
}

func (rh *RepoHandler) SyncArtifact(ctx context.Context, name string, timeout time.Duration) (chan error, func() error) {
	errCh := make(chan error, 1)

	rh.syncArtifactsMutex.Lock()
	rh.syncArtifacts[name] = errCh
	rh.syncArtifactsMutex.Unlock()

	return errCh, func() error {
		var err error

		pkgCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		select {
		case <-pkgCtx.Done():
			if errors.Is(pkgCtx.Err(), context.Canceled) {
				err = errors.New("waiting timeout")
			} else {
				err = errors.New("waiting interruption")
			}
		case err = <-errCh:
		}

		rh.syncArtifactsMutex.Lock()
		close(errCh)
		delete(rh.syncArtifacts, name)
		rh.syncArtifactsMutex.Unlock()

		return err
	}
}

func (rh *RepoHandler) SyncArtifactResult(name string, err error) {
	rh.syncArtifactsMutex.RLock()
	if errCh, ok := rh.syncArtifacts[name]; ok {
		errCh <- err
	}
	rh.syncArtifactsMutex.RUnlock()
}

func (rh *RepoHandler) SyncArtifactReset() {
	rh.syncArtifactsMutex.Lock()
	for k, v := range rh.syncArtifacts {
		delete(rh.syncArtifacts, k)
		close(v)
	}
	rh.syncArtifactsMutex.Unlock()
}
