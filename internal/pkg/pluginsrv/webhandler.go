// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package pluginsrv

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"

	"go.ciq.dev/beskar/internal/pkg/log"
	"go.ciq.dev/beskar/internal/pkg/repository"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	pluginv1 "go.ciq.dev/beskar/pkg/api/plugin/v1"
	"google.golang.org/protobuf/proto"
)

type webHandler struct {
	pluginInfo *pluginv1.Info
	manager    *repository.Manager
}

func IsTLS(w http.ResponseWriter, r *http.Request) bool {
	if r.TLS == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

// IsTLSMiddleware is a middleware that checks if the request is TLS. This is a convenience wrapper around IsTLS.
func IsTLSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsTLS(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (wh *webHandler) event(w http.ResponseWriter, r *http.Request) {
	if wh.manager == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	ctx := r.Context()
	logger := log.GetContextLogger(ctx)

	buf := new(bytes.Buffer)

	_, err := io.Copy(buf, r.Body)
	if err != nil {
		logger.ErrorContext(ctx, "manifest copy", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	event := new(eventv1.EventPayload)
	if err := proto.Unmarshal(buf.Bytes(), event); err != nil {
		logger.ErrorContext(ctx, "unmarshal event", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	repositoryName := filepath.Dir(event.Repository)
	if repositoryName == "" {
		logger.ErrorContext(ctx, "empty repository name")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.InfoContext(ctx, "process event", "action", event.Action.String(), "repository", repositoryName)

	switch event.Action {
	case eventv1.Action_ACTION_PUT, eventv1.Action_ACTION_DELETE:
		err = wh.manager.Get(ctx, repositoryName).QueueEvent(event, true)
		if err != nil {
			logger.ErrorContext(ctx, "process put/delete event", "repository", repositoryName, "error", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case eventv1.Action_ACTION_START:
		_ = wh.manager.Get(ctx, repositoryName)
	case eventv1.Action_ACTION_STOP:
		if wh.manager.Has(repositoryName) {
			wh.manager.Get(ctx, repositoryName).Stop()
		}
	}
}

func (wh *webHandler) info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")

	data, err := proto.Marshal(wh.pluginInfo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}
