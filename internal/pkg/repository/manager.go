// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"log/slog"
	"sync"

	"go.ciq.dev/beskar/internal/pkg/log"
)

type Manager[H Handler] struct {
	repositoryMutex  sync.RWMutex
	repositories     map[string]H
	repositoryParams *HandlerParams

	newHandler func(*slog.Logger, *RepoHandler) H
}

func NewManager[H Handler](
	params *HandlerParams,
	newHandler func(*slog.Logger, *RepoHandler) H,
) *Manager[H] {
	m := &Manager[H]{
		repositories:     make(map[string]H),
		repositoryParams: params,
		newHandler:       newHandler,
	}
	params.remove = m.remove

	return m
}

func (m *Manager[H]) remove(repository string) {
	m.repositoryMutex.Lock()
	delete(m.repositories, repository)
	m.repositoryMutex.Unlock()
}

func (m *Manager[H]) Get(ctx context.Context, repository string) H {
	m.repositoryMutex.RLock()
	r, ok := m.repositories[repository]
	m.repositoryMutex.RUnlock()

	if ok && r.Started() {
		return r
	}

	logger := log.GetContextLogger(ctx)
	logger = logger.With("repository", repository)

	handlerCtx, cancel := context.WithCancel(context.Background())

	rh := m.newHandler(logger, NewRepoHandler(repository, m.repositoryParams, cancel))
	rh.Start(handlerCtx)

	logger.Info("repository handler started")

	m.repositoryMutex.Lock()
	m.repositories[repository] = rh
	m.repositoryMutex.Unlock()

	return rh
}

func (m *Manager[H]) Has(repository string) bool {
	m.repositoryMutex.RLock()
	_, ok := m.repositories[repository]
	m.repositoryMutex.RUnlock()

	return ok
}

func (m *Manager[H]) GetAll() map[string]H {
	m.repositoryMutex.RLock()

	handlers := make(map[string]H)
	for name, handler := range m.repositories {
		handlers[name] = handler
	}

	m.repositoryMutex.RUnlock()

	return handlers
}
