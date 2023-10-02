// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yum

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"path/filepath"

	"go.ciq.dev/beskar/internal/plugins/yum/pkg/log"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	pluginv1 "go.ciq.dev/beskar/pkg/api/plugin/v1"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"go.ciq.dev/beskar/pkg/version"
	"google.golang.org/protobuf/proto"
)

func isTLS(w http.ResponseWriter, r *http.Request) bool {
	if r.TLS == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

func (p *Plugin) eventHandler(w http.ResponseWriter, r *http.Request) {
	if !isTLS(w, r) {
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

	switch event.Action {
	case eventv1.Action_ACTION_PUT, eventv1.Action_ACTION_DELETE:
		err = p.repositoryHandler(repositoryName).QueueEvent(event, true)
		if err != nil {
			logger.ErrorContext(ctx, "process put/delete event", "repository", repositoryName, "error", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case eventv1.Action_ACTION_START:
		logger.InfoContext(ctx, "process start event", "repository", repositoryName)
		p.repositoryHandler(repositoryName)
	case eventv1.Action_ACTION_STOP:
		logger.InfoContext(ctx, "process stop event", "repository", repositoryName)
		if p.hasRepositoryHander(repositoryName) {
			p.repositoryHandler(repositoryName).Stop()
		}
	}
}

//go:embed embedded/router.rego
var routerRego []byte

//go:embed embedded/data.json
var routerData []byte

func (p *Plugin) infoHandler(w http.ResponseWriter, r *http.Request) {
	if !isTLS(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")

	info := &pluginv1.Info{
		Name:    "yum",
		Prefix:  "/yum",
		Version: version.Semver,
		Mediatypes: []string{
			orasrpm.RPMConfigType,
			orasrpm.RepomdDataConfigType,
		},
		Router: &pluginv1.Router{
			Rego: routerRego,
			Data: routerData,
		},
	}

	data, err := proto.Marshal(info)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}
