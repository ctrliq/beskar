// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostree

import (
	"context"
	_ "embed"
	"github.com/RussellLuo/kun/pkg/httpcodec"
	"github.com/go-chi/chi"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/pkg/log"
	"go.ciq.dev/beskar/internal/pkg/pluginsrv"
	"go.ciq.dev/beskar/internal/pkg/repository"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/config"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/ostreerepository"
	pluginv1 "go.ciq.dev/beskar/pkg/api/plugin/v1"
	"go.ciq.dev/beskar/pkg/mtls"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/ostree/api/v1"
	"go.ciq.dev/beskar/pkg/version"
	"net"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"strconv"
)

const (
	PluginName           = "ostree"
	PluginAPIPathPattern = "/artifacts/ostree/api/v1"
)

//go:embed embedded/router.rego
var routerRego []byte

//go:embed embedded/data.json
var routerData []byte

type Plugin struct {
	ctx    context.Context
	config pluginsrv.Config

	repositoryManager *repository.Manager[*ostreerepository.Handler]
	handlerParams     *repository.HandlerParams
}

func New(ctx context.Context, beskarOSTreeConfig *config.BeskarOSTreeConfig) (*Plugin, error) {
	logger, err := beskarOSTreeConfig.Log.Logger(log.ContextHandler)
	if err != nil {
		return nil, err
	}
	ctx = log.SetContextLogger(ctx, logger)

	router := chi.NewRouter()

	// for kubernetes probes
	router.Handle("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	if beskarOSTreeConfig.Profiling {
		router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
		router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
	}

	params := &repository.HandlerParams{
		Dir: filepath.Join(beskarOSTreeConfig.DataDir, "_repohandlers_"),
	}

	return &Plugin{
		ctx: ctx,
		config: pluginsrv.Config{
			Router: router,
			Gossip: beskarOSTreeConfig.Gossip,
			Info: &pluginv1.Info{
				Name: PluginName,
				// Not registering media types so that Beskar doesn't send events.
				Mediatypes: []string{},
				Version:    version.Semver,
				Router: &pluginv1.Router{
					Rego: routerRego,
					Data: routerData,
				},
			},
		},
		handlerParams: params,
		repositoryManager: repository.NewManager[*ostreerepository.Handler](
			params,
			ostreerepository.NewHandler,
		),
	}, nil
}

func (p *Plugin) Start(transport http.RoundTripper, _ *mtls.CAPEM, beskarMeta *gossip.BeskarMeta) error {
	// Collection beskar http service endpoint for later pulls
	p.handlerParams.BeskarMeta = beskarMeta

	hostport := net.JoinHostPort(beskarMeta.Hostname, strconv.Itoa(int(beskarMeta.RegistryPort)))
	p.handlerParams.NameOptions = []name.Option{
		name.WithDefaultRegistry(hostport),
	}
	p.handlerParams.RemoteOptions = []remote.Option{
		remote.WithTransport(transport),
	}

	p.config.Router.Route(
		PluginAPIPathPattern,
		func(r chi.Router) {
			r.Use(pluginsrv.IsTLSMiddleware)
			r.Mount("/", apiv1.NewHTTPRouter(
				p,
				httpcodec.NewDefaultCodecs(nil),
			))
		},
	)

	return nil
}

func (p *Plugin) Context() context.Context {
	return p.ctx
}

func (p *Plugin) Config() pluginsrv.Config {
	return p.config
}

func (p *Plugin) RepositoryManager() *repository.Manager[*ostreerepository.Handler] {
	return p.repositoryManager
}
