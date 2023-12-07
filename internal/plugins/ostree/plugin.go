package ostree

import (
	"context"
	_ "embed"
	"github.com/RussellLuo/kun/pkg/httpcodec"
	"github.com/go-chi/chi"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/pkg/log"
	"go.ciq.dev/beskar/internal/pkg/pluginsrv"
	"go.ciq.dev/beskar/internal/pkg/repository"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/config"
	pluginv1 "go.ciq.dev/beskar/pkg/api/plugin/v1"
	"go.ciq.dev/beskar/pkg/mtls"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/ostree/api/v1"
	"go.ciq.dev/beskar/pkg/version"
	"net/http"
	"net/http/pprof"
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
}

func New(ctx context.Context, beskarOSTreeConfig *config.BeskarOSTreeConfig) (*Plugin, error) {
	logger, err := beskarOSTreeConfig.Log.Logger(log.ContextHandler)
	if err != nil {
		return nil, err
	}
	ctx = log.SetContextLogger(ctx, logger)

	apiSrv := newAPIService()
	router := makeRouter(apiSrv, beskarOSTreeConfig.Profiling)

	return &Plugin{
		ctx: ctx,
		config: pluginsrv.Config{
			Router: router,
			Gossip: beskarOSTreeConfig.Gossip,
			Info: &pluginv1.Info{
				Name: PluginName,
				// Not registering media types so that Beskar doesn't send events.
				// This plugin as no internal state so events are not needed.
				Mediatypes: []string{},
				Version:    version.Semver,
				Router: &pluginv1.Router{
					Rego: routerRego,
					Data: routerData,
				},
			},
		},
	}, nil
}

func (p *Plugin) Start(_ http.RoundTripper, _ *mtls.CAPEM, _ *gossip.BeskarMeta) error {
	// Nothing to do here as this plugin has no internal state
	// and router is already configured.
	return nil
}

func (p *Plugin) Context() context.Context {
	return p.ctx
}

func (p *Plugin) Config() pluginsrv.Config {
	return p.config
}

func (p *Plugin) RepositoryManager() *repository.Manager {
	// this plugin has no internal state so no need for a repository manager
	return nil
}

func makeRouter(apiSrv *apiService, profilingEnabled bool) *chi.Mux {
	router := chi.NewRouter()

	// for kubernetes probes
	router.Handle("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	if profilingEnabled {
		router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
		router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
	}

	router.Route(
		PluginAPIPathPattern,
		func(r chi.Router) {
			r.Use(pluginsrv.IsTLSMiddleware)
			r.Mount("/", apiv1.NewHTTPRouter(
				apiSrv,
				httpcodec.NewDefaultCodecs(nil),
			))
		},
	)

	return router
}
