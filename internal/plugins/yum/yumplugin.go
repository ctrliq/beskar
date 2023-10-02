// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yum

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/RussellLuo/kun/pkg/httpcodec"
	"github.com/go-chi/chi"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/hashicorp/memberlist"
	"go.ciq.dev/beskar/internal/pkg/cmux"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/log"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/repository"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/storage"
	"go.ciq.dev/beskar/pkg/mtls"
	"go.ciq.dev/beskar/pkg/netutil"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/yum/api/v1"
	"golang.org/x/exp/slog"
)

type Plugin struct {
	ctx             context.Context
	server          http.Server
	beskarYumConfig *config.BeskarYumConfig

	repositoryMutex  sync.RWMutex
	repositories     map[string]*repository.Handler
	repositoryParams *repository.HandlerParams
}

func New(ctx context.Context, beskarYumConfig *config.BeskarYumConfig, server bool) (*Plugin, error) {
	logger, err := beskarYumConfig.Log.Logger(log.ContextHandler)
	if err != nil {
		return nil, err
	}

	if beskarYumConfig.DataDir == "" {
		beskarYumConfig.DataDir = config.DefaultBeskarYumDataDir
	}

	plugin := &Plugin{
		ctx:             log.SetContextLogger(ctx, logger),
		beskarYumConfig: beskarYumConfig,
		repositories:    make(map[string]*repository.Handler),
		repositoryParams: &repository.HandlerParams{
			Dir: filepath.Join(beskarYumConfig.DataDir, "_repohandlers_"),
		},
	}

	plugin.repositoryParams.Remove = func(repositoryName string) {
		plugin.repositoryMutex.Lock()
		delete(plugin.repositories, repositoryName)
		plugin.repositoryMutex.Unlock()
	}

	prefix := storage.GetPrefix(beskarYumConfig)

	plugin.repositoryParams.Bucket, err = storage.Init(plugin.ctx, beskarYumConfig, prefix)
	if err != nil {
		return nil, err
	}

	if err := os.RemoveAll(plugin.repositoryParams.Dir); err != nil {
		return nil, err
	} else if err := os.MkdirAll(plugin.repositoryParams.Dir, 0o700); err != nil {
		return nil, err
	}

	if server {
		router := chi.NewRouter()
		// for kubernetes probes
		router.Handle("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

		httpContext := log.SetContextAttrs(plugin.ctx, slog.String("context", "http"))

		plugin.server = http.Server{
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
			BaseContext: func(net.Listener) context.Context {
				return httpContext
			},
		}

		if beskarYumConfig.Profiling {
			plugin.setProfilingRoute()
		}
	}

	return plugin, nil
}

func (p *Plugin) setProfilingRoute() {
	router, ok := p.server.Handler.(*chi.Mux)
	if !ok {
		return
	}
	router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
}

func (p *Plugin) setRoute() {
	router, ok := p.server.Handler.(*chi.Mux)
	if !ok {
		return
	}

	router.HandleFunc("/event", http.HandlerFunc(p.eventHandler))
	router.HandleFunc("/info", http.HandlerFunc(p.infoHandler))
	router.Route(
		"/yum/api/v1",
		func(r chi.Router) {
			r.Use(p.apiMiddleware)
			r.Mount("/", apiv1.NewHTTPRouter(
				p,
				httpcodec.NewDefaultCodecs(nil),
			))
		},
	)
}

func (p *Plugin) apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isTLS(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) startGossipWatcher(member *gossip.Member) <-chan *gossip.BeskarMeta {
	var hostnameOnce sync.Once
	beskarMetaCh := make(chan *gossip.BeskarMeta, 1)

	logger := log.GetContextLogger(p.ctx)

	self := member.LocalNode()

	go func() {
		for event := range member.Watch() {
			switch event.EventType {
			case gossip.NodeJoin:
				node, ok := event.Arg.(*memberlist.Node)
				if !ok || self.Name == node.Name {
					continue
				}

				logger.Debug("node added to cluster", "addr", node.Addr)

				if self.Port != node.Port || !self.Addr.Equal(node.Addr) {
					meta := gossip.NewBeskarMeta()
					if err := meta.Decode(node.Meta); err == nil {
						if meta.InstanceType == gossip.BeskarInstance {
							hostnameOnce.Do(func() {
								logger.Info("beskar instance added", "hostname", meta.Hostname, "port", meta.RegistryPort)
								beskarMetaCh <- meta
								close(beskarMetaCh)
							})
						}
					}
				}
			case gossip.NodeLeave:
				node, ok := event.Arg.(*memberlist.Node)
				if !ok || self.Name == node.Name {
					continue
				}
				logger.Debug("node removed from cluster", "addr", node.Addr)
			}
		}
	}()

	return beskarMetaCh
}

func (p *Plugin) initGossip() (_ *gossip.Member, _ *mtls.CAPEM, errFn error) {
	_, port, err := net.SplitHostPort(p.beskarYumConfig.Addr)
	if err != nil {
		return nil, nil, err
	}
	servicePort, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, nil, err
	}

	meta := gossip.NewBeskarMeta()
	meta.ServicePort = uint16(servicePort)
	meta.InstanceType = gossip.PluginInstance
	meta.Hostname, err = os.Hostname()
	if err != nil {
		return nil, nil, err
	}

	member, err := gossip.Start(p.beskarYumConfig.Gossip, meta, nil, 300*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if errFn != nil {
			_ = member.Shutdown()
		}
	}()

	remoteState, err := member.RemoteState()
	if err != nil {
		if len(member.Nodes()) == 0 {
			return nil, nil, err
		}
		remoteState, err = member.LocalState()
		if err != nil {
			return nil, nil, err
		}
	}

	caPem, err := mtls.UnmarshalCAPEM(remoteState)
	if err != nil {
		return nil, nil, fmt.Errorf("while unmarshalling CA certificates: %w", err)
	}

	return member, caPem, nil
}

func (p *Plugin) Serve(ln net.Listener) error {
	var serverErr error

	errCh := make(chan error)
	logger := log.GetContextLogger(p.ctx)

	serverListener := cmux.NewListener(ln)

	go func() {
		logger.Info("server started", "listen_addr", ln.Addr().String())
		errCh <- p.server.Serve(serverListener)
	}()

	gossipMember, caPEM, err := p.initGossip()
	if err != nil {
		return err
	}
	beskarMetaCh := p.startGossipWatcher(gossipMember)

	if err := setServerTLSConfig(serverListener, caPEM); err != nil {
		_ = gossipMember.Shutdown()
		return err
	}

	ticker := time.NewTicker(time.Second * 5)

	select {
	case <-p.ctx.Done():
		return gossipMember.Shutdown()
	case <-ticker.C:
		_ = gossipMember.Shutdown()
		return fmt.Errorf("no beskar instance found")
	case beskarMeta := <-beskarMetaCh:
		ticker.Stop()
		if err := p.finalizeRepositoryParams(caPEM, beskarMeta); err != nil {
			_ = gossipMember.Shutdown()
			return err
		}
	}

	p.setRoute()

	if err := gossipMember.MarkAsReady(gossip.DefaultReadyTimeout); err != nil {
		_ = gossipMember.Shutdown()
		return err
	}

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server failure", "error", err.Error())
		}
		serverErr = err
	case <-p.ctx.Done():
		logger.Info("server shutdown")
	}

	_ = p.server.Shutdown(p.ctx)

	p.repositoryMutex.Lock()

	handlers := make(map[string]*repository.Handler)
	for name, handler := range p.repositories {
		handlers[name] = handler
	}

	p.repositoryMutex.Unlock()

	var wg sync.WaitGroup

	for name, handler := range handlers {
		wg.Add(1)

		go func(name string, handler *repository.Handler) {
			logger.Info("stopping repository handler", "repository", name)
			handler.Stop()
			logger.Info("repository handler stopped", "repository", name)
			wg.Done()
		}(name, handler)
	}

	wg.Wait()

	gossipErr := gossipMember.Shutdown()
	if serverErr == nil {
		serverErr = gossipErr
	}

	return serverErr
}

func (p *Plugin) repositoryHandler(repositoryName string) *repository.Handler {
	p.repositoryMutex.RLock()
	r, ok := p.repositories[repositoryName]
	p.repositoryMutex.RUnlock()

	if ok && r.Started() {
		return r
	}

	logger := log.GetContextLogger(p.ctx)
	logger = logger.With("repository", repositoryName)

	rh := repository.NewHandler(logger, repositoryName, p.repositoryParams)

	p.repositoryMutex.Lock()
	p.repositories[repositoryName] = rh
	p.repositoryMutex.Unlock()

	return rh
}

func (p *Plugin) hasRepositoryHander(repositoryName string) bool {
	p.repositoryMutex.RLock()
	_, ok := p.repositories[repositoryName]
	p.repositoryMutex.RUnlock()

	return ok
}

func (p *Plugin) finalizeRepositoryParams(caPEM *mtls.CAPEM, beskarMeta *gossip.BeskarMeta) error {
	tlsConfig, err := mtls.GenerateClientConfig(
		bytes.NewReader(caPEM.Cert),
		bytes.NewReader(caPEM.Key),
		time.Now().AddDate(10, 0, 0),
	)
	if err != nil {
		return fmt.Errorf("while generating beskar client mTLS configuration: %w", err)
	}
	//nolint:gosec
	tlsConfig.ServerName = fmt.Sprintf("%x", md5.Sum([]byte(beskarMeta.Hostname)))

	hostport := net.JoinHostPort(beskarMeta.Hostname, strconv.Itoa(int(beskarMeta.RegistryPort)))

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	transport.MaxIdleConnsPerHost = 16

	p.repositoryParams.NameOptions = []name.Option{
		name.WithDefaultRegistry(hostport),
	}
	p.repositoryParams.RemoteOptions = []remote.Option{
		remote.WithTransport(transport),
	}

	return nil
}

func setServerTLSConfig(ln *cmux.Listener, caPEM *mtls.CAPEM) error {
	localIPs, err := netutil.LocalIPs()
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	tlsConfig, err := mtls.GenerateServerConfig(
		bytes.NewReader(caPEM.Cert),
		bytes.NewReader(caPEM.Key),
		time.Now().AddDate(10, 0, 0),
		mtls.WithCertRequestIPs(localIPs...),
		mtls.WithCertRequestHostnames(hostname),
	)
	if err != nil {
		return err
	}

	ln.SetTLSConfig(tlsConfig)

	return nil
}
