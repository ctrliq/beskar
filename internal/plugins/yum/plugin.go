// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yum

import (
	"context"
	_ "embed"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strconv"

	"github.com/RussellLuo/kun/pkg/httpcodec"
	"github.com/go-chi/chi"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/pkg/log"
	"go.ciq.dev/beskar/internal/pkg/pluginsrv"
	"go.ciq.dev/beskar/internal/pkg/repository"
	"go.ciq.dev/beskar/internal/pkg/storage"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/config"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumrepository"
	pluginv1 "go.ciq.dev/beskar/pkg/api/plugin/v1"
	"go.ciq.dev/beskar/pkg/mtls"
	"go.ciq.dev/beskar/pkg/orasrpm"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/yum/api/v1"
	"go.ciq.dev/beskar/pkg/version"
)

//go:embed embedded/router.rego
var routerRego []byte

//go:embed embedded/data.json
var routerData []byte

type Plugin struct {
	ctx    context.Context
	config pluginsrv.Config

	repositoryManager *repository.Manager[*yumrepository.Handler]
	handlerParams     *repository.HandlerParams
}

var _ pluginsrv.Service[*yumrepository.Handler] = &Plugin{}

func New(ctx context.Context, beskarYumConfig *config.BeskarYumConfig) (*Plugin, error) {
	logger, err := beskarYumConfig.Log.Logger(log.ContextHandler)
	if err != nil {
		return nil, err
	}

	if beskarYumConfig.DataDir == "" {
		beskarYumConfig.DataDir = config.DefaultBeskarYumDataDir
	}

	ctx = log.SetContextLogger(ctx, logger)

	plugin := &Plugin{
		ctx: ctx,
		handlerParams: &repository.HandlerParams{
			Dir: filepath.Join(beskarYumConfig.DataDir, "_repohandlers_"),
		},
	}
	plugin.repositoryManager = repository.NewManager[*yumrepository.Handler](
		plugin.handlerParams,
		yumrepository.NewHandler,
	)

	prefix := storage.GetPrefix(beskarYumConfig.Storage)

	plugin.handlerParams.Bucket, err = storage.Init(ctx, beskarYumConfig.Storage, prefix)
	if err != nil {
		return nil, err
	}

	if err := os.RemoveAll(plugin.handlerParams.Dir); err != nil {
		return nil, err
	} else if err := os.MkdirAll(plugin.handlerParams.Dir, 0o700); err != nil {
		return nil, err
	}

	router := chi.NewRouter()
	// for kubernetes probes
	router.Handle("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	plugin.config.Router = router
	plugin.config.Gossip = beskarYumConfig.Gossip
	plugin.config.Info = &pluginv1.Info{
		Name:    "yum",
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

	if beskarYumConfig.Profiling {
		router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
		router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
	}

	return plugin, nil
}

func (p *Plugin) Start(transport http.RoundTripper, _ *mtls.CAPEM, beskarMeta *gossip.BeskarMeta) error {
	hostport := net.JoinHostPort(beskarMeta.Hostname, strconv.Itoa(int(beskarMeta.RegistryPort)))

	p.handlerParams.NameOptions = []name.Option{
		name.WithDefaultRegistry(hostport),
	}
	p.handlerParams.RemoteOptions = []remote.Option{
		remote.WithTransport(transport),
	}

	p.config.Router.Route(
		"/artifacts/yum/api/v1",
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

func (p *Plugin) Config() pluginsrv.Config {
	return p.config
}

func (p *Plugin) Context() context.Context {
	return p.ctx
}

func (p *Plugin) RepositoryManager() *repository.Manager[*yumrepository.Handler] {
	return p.repositoryManager
}
