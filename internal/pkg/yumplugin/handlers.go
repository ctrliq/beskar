// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumplugin

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/gorilla/mux"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
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

func blobsHandler(plugin *Plugin, blobType string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		filename := vars["filename"]
		repository := vars["repository"]

		if filename == "" || repository == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if filename == "repomd.xml" {
			uri, err := getURI(plugin, repository, blobType, orasrpm.RepomdXMLLayerType, "latest")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else if uri != "" {
				http.Redirect(w, r, uri, http.StatusMovedPermanently)
				return
			}
		} else {
			if blobType == "repodata" {
				digest := strings.SplitN(filename, "-", 2)[0]
				if digest != "" {
					uri := fmt.Sprintf("/v2/yum/%s/%s/blobs/sha256:%s", repository, blobType, digest)
					http.Redirect(w, r, uri, http.StatusMovedPermanently)
				}
			} else {
				tag := fmt.Sprintf("%x", sha256.Sum256([]byte(filename)))
				uri, err := getURI(plugin, repository, blobType, orasrpm.RPMPackageLayerType, tag)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else if uri != "" {
					http.Redirect(w, r, uri, http.StatusMovedPermanently)
					return
				}
			}
		}

		w.WriteHeader(http.StatusNotFound)
	}
}

func getURI(plugin *Plugin, repository, blobType, mediaType, tag string) (string, error) {
	rawRef := filepath.Join(plugin.registry, "yum", repository, fmt.Sprintf("%s:%s", blobType, tag))
	ref, err := name.ParseReference(rawRef, plugin.nameOptions...)
	if err != nil {
		return "", err
	}

	manifest, err := oras.GetManifest(ref, plugin.remoteOptions...)
	if err != nil {
		return "", err
	}

	for _, layer := range manifest.Layers {
		if layer.MediaType != types.MediaType(mediaType) {
			continue
		}
		uri := fmt.Sprintf(
			"/v2/yum/%s/%s/blobs/%s",
			repository, blobType, layer.Digest.String(),
		)
		return uri, nil
	}

	return "", nil
}
