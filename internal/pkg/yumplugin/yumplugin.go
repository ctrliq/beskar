// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumplugin

import (
	"context"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gorilla/mux"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/storage"
	"go.ciq.dev/beskar/pkg/oras"
	"gocloud.dev/blob"
)

type Plugin struct {
	registry        string
	dataDir         string
	manifests       []*v1.Manifest
	manifestMutex   sync.Mutex
	queued          chan struct{}
	server          http.Server
	remoteOptions   []remote.Option
	nameOptions     []name.Option
	bucket          *blob.Bucket
	beskarYumConfig *config.BeskarYumConfig
}

func New(ctx context.Context, beskarYumConfig *config.BeskarYumConfig, server bool) (*Plugin, error) {
	registryURL, err := url.Parse(beskarYumConfig.Registry.URL)
	if err != nil {
		return nil, err
	}

	os.Setenv("HOME", beskarYumConfig.DataDir)

	plugin := &Plugin{
		registry:        registryURL.Host,
		manifests:       make([]*v1.Manifest, 0, 32),
		queued:          make(chan struct{}, 1),
		dataDir:         beskarYumConfig.DataDir,
		beskarYumConfig: beskarYumConfig,
		remoteOptions: []remote.Option{
			oras.AuthConfig(beskarYumConfig.Registry.Username, beskarYumConfig.Registry.Password),
		},
	}

	if registryURL.Scheme == "http" {
		plugin.nameOptions = append(plugin.nameOptions, name.Insecure)
	}

	plugin.bucket, err = storage.Init(ctx, beskarYumConfig)
	if err != nil {
		return nil, err
	}

	if beskarYumConfig.DataDir == "" {
		beskarYumConfig.DataDir = config.DefaultBeskarYumDataDir
	}

	if err := os.MkdirAll(plugin.dataDir, 0o700); err != nil {
		return nil, err
	}

	if server {
		router := mux.NewRouter()
		router.HandleFunc("/event", plugin.eventHandler())
		router.HandleFunc("/yum/repo/{repository}/repodata/repomd.xml", repomdHandler(plugin))
		router.HandleFunc("/yum/repo/{repository}/repodata/{digest}-{file}", blobsHandler("repodata"))
		router.HandleFunc("/yum/repo/{repository}/packages/{digest}/{file}", blobsHandler("packages"))

		if beskarYumConfig.Profiling {
			plugin.setProfiling(router)
		}

		plugin.server = http.Server{
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go plugin.dequeue(ctx)
	}

	return plugin, nil
}

func (p *Plugin) setProfiling(router *mux.Router) {
	router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
}

func (p *Plugin) Serve(ln net.Listener) error {
	return p.server.Serve(ln)
}

func (p *Plugin) dequeue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.queued:
			p.manifestMutex.Lock()
			length := len(p.manifests)
			if length > 20 {
				length = 20
			}
			manifests := make([]*v1.Manifest, length)
			copy(manifests, p.manifests)
			p.manifests = p.manifests[length:]
			if len(p.manifests) > 0 {
				p.enqueueNotify()
			}
			p.manifestMutex.Unlock()
			p.processPackages(ctx, manifests)
		}
	}
}

func (p *Plugin) enqueueNotify() {
	select {
	case p.queued <- struct{}{}:
	default:
	}
}

func (p *Plugin) enqueue(manifest *v1.Manifest) {
	p.manifestMutex.Lock()
	p.manifests = append(p.manifests, manifest)
	p.manifestMutex.Unlock()
	p.enqueueNotify()
}
