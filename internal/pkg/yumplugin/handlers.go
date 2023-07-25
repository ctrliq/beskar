// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumplugin

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gorilla/mux"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/orasrpm"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"go.ciq.dev/beskar/pkg/oras"
	"google.golang.org/protobuf/proto"
)

func (p *Plugin) eventHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}

		buf := new(bytes.Buffer)

		_, err := io.Copy(buf, r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		event := new(eventv1.ManifestEvent)
		if err := proto.Unmarshal(buf.Bytes(), event); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		ociManifest, err := v1.ParseManifest(bytes.NewReader(event.Payload))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if ociManifest.Annotations == nil {
			ociManifest.Annotations = make(map[string]string)
		}

		ociManifest.Annotations["repository"] = event.Repository

		p.enqueue(ociManifest)
	}
}

func repomdHandler(plugin *Plugin) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		rawRef := filepath.Join(plugin.registry, "yum", vars["repository"], "repodata:latest")
		ref, err := name.ParseReference(rawRef, plugin.nameOptions...)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		manifest, err := oras.GetManifest(ref, plugin.remoteOptions...)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, layer := range manifest.Layers {
			if layer.MediaType != orasrpm.RepomdXMLLayerType {
				continue
			}
			uri := fmt.Sprintf(
				"/v2/yum/%s/repodata/blobs/%s",
				vars["repository"], layer.Digest.String(),
			)
			http.Redirect(w, r, uri, http.StatusMovedPermanently)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}
}

func blobsHandler(blobType string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		uri := fmt.Sprintf("/v2/yum/%s/%s/blobs/%s", vars["repository"], blobType, vars["digest"])
		http.Redirect(w, r, uri, http.StatusMovedPermanently)
	}
}
