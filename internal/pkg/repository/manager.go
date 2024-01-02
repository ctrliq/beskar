// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"log/slog"
	"sync"

	"go.ciq.dev/beskar/internal/pkg/log"
)

type HandlerMap = map[string]Handler

type HandlerFactory = func(*slog.Logger, *RepoHandler) Handler

type Manager struct {
	repositoryMutex  sync.RWMutex
	repositories     HandlerMap
	repositoryParams *HandlerParams

	newHandler func(*slog.Logger, *RepoHandler) Handler
}

func NewManager(params *HandlerParams, newHandler HandlerFactory) *Manager {
	m := &Manager{
		repositories:     make(HandlerMap),
		repositoryParams: params,
		newHandler:       newHandler,
	}
	params.remove = m.remove

	return m
}

func (m *Manager) remove(repository string) {
	m.repositoryMutex.Lock()
	delete(m.repositories, repository)
	m.repositoryMutex.Unlock()
}

func (m *Manager) Get(ctx context.Context, repository string) Handler {
	m.repositoryMutex.Lock()

	r, ok := m.repositories[repository]

	if ok {
		m.repositoryMutex.Unlock()
		if r.Started() {
			return r
		}
		// TODO: in case of repeated start failure this will be a problem
		m.repositoryMutex.Lock()
	}

	logger := log.GetContextLogger(ctx)
	logger = logger.With("repository", repository)

	handlerCtx, cancel := context.WithCancel(context.Background())

	parentHandler := NewRepoHandler(repository, m.repositoryParams, cancel)
	rh := m.newHandler(logger, parentHandler)

	m.repositories[repository] = rh

	m.repositoryMutex.Unlock()

	rh.Start(handlerCtx)
	close(parentHandler.startedCh)

	logger.Info("repository handler started")

	return rh
}

func (m *Manager) Has(repository string) bool {
	m.repositoryMutex.RLock()
	_, ok := m.repositories[repository]
	m.repositoryMutex.RUnlock()

	return ok
}

func (m *Manager) GetAll() HandlerMap {
	m.repositoryMutex.RLock()

	handlers := make(HandlerMap)
	for name, handler := range m.repositories {
		handlers[name] = handler
	}

	m.repositoryMutex.RUnlock()

	return handlers
}
