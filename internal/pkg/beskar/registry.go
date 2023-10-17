// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package beskar

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"reflect"
	"strconv"
	"syscall"
	"time"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry"
	"github.com/distribution/distribution/v3/registry/auth"
	"github.com/distribution/distribution/v3/version"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gorilla/mux"
	"github.com/hashicorp/memberlist"
	"github.com/mailgun/groupcache/v2"
	"github.com/opencontainers/go-digest"
	"go.ciq.dev/beskar/internal/pkg/cache"
	"go.ciq.dev/beskar/internal/pkg/cmux"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"go.ciq.dev/beskar/pkg/mtls"
	"go.ciq.dev/beskar/pkg/netutil"
	"go.ciq.dev/beskar/pkg/sighandler"

	// load distribution filesystem storage driver
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	// load distribution s3 storage driver
	_ "github.com/distribution/distribution/v3/registry/storage/driver/s3-aws"
	// load distribution azure storage driver
	_ "github.com/distribution/distribution/v3/registry/storage/driver/azure"
	// load distribution gcs storage driver
	_ "github.com/distribution/distribution/v3/registry/storage/driver/gcs"
)

type Registry struct {
	registry      distribution.Namespace
	beskarConfig  *config.BeskarConfig
	router        *mux.Router
	server        *http.Server
	member        *gossip.Member
	manifestCache *cache.GroupCache
	pluginManager *pluginManager
	errCh         chan error
	logger        dcontext.Logger
	wait          sighandler.WaitFunc
}

func New(beskarConfig *config.BeskarConfig) (context.Context, *Registry, error) {
	beskarRegistry := &Registry{
		beskarConfig: beskarConfig,
		errCh:        make(chan error, 1),
	}

	ctx, waitFunc := sighandler.New(beskarRegistry.errCh, syscall.SIGINT)
	beskarRegistry.wait = waitFunc

	ctx = dcontext.WithVersion(ctx, version.Version)

	beskarRegistry.router = mux.NewRouter()
	// for probes
	beskarRegistry.router.Handle("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	err := registerRegistryMiddleware(beskarRegistry, func(registry distribution.Namespace) *pluginManager {
		beskarRegistry.registry = registry
		beskarRegistry.pluginManager = newPluginManager(registry, beskarRegistry.router)
		return beskarRegistry.pluginManager
	})
	if err != nil {
		return nil, nil, err
	}

	if err := auth.Register("beskar", newAccessController(beskarRegistry.getHashedHostname())); err != nil {
		return nil, nil, err
	}

	registryServer, err := registry.NewRegistry(ctx, beskarConfig.Registry)
	if err != nil {
		return nil, nil, err
	}

	reflectServer := reflect.ValueOf(registryServer).Elem().FieldByName("server")
	if reflectServer.IsZero() || reflectServer.IsNil() {
		return nil, nil, fmt.Errorf("no http.Server found in registry struct")
	}
	reflectServer = reflect.NewAt(reflectServer.Type(), reflectServer.Addr().UnsafePointer()).Elem()
	server, ok := reflectServer.Interface().(*http.Server)
	if !ok {
		return nil, nil, fmt.Errorf("no http.Server found")
	}

	beskarRegistry.server = server
	beskarRegistry.logger = dcontext.GetLogger(ctx)

	if beskarConfig.Profiling {
		beskarRegistry.setProfiling()
	}

	return ctx, beskarRegistry, nil
}

func (br *Registry) setProfiling() {
	br.logger.Debug("Adding golang profiling endpoints")

	br.router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	br.router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	br.router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	br.router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	br.router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	br.router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
}

func (br *Registry) startGossipWatcher() {
	self := br.member.LocalNode()

	for event := range br.member.Watch() {
		switch event.EventType {
		case gossip.NodeUpdate:
			node, ok := event.Arg.(*memberlist.Node)
			if !ok || self.Name == node.Name {
				continue
			}

			if self.Port != node.Port || !self.Addr.Equal(node.Addr) {
				meta := gossip.NewBeskarMeta()
				if err := meta.Decode(node.Meta); err == nil && meta.Ready {
					switch meta.InstanceType {
					case gossip.BeskarInstance:
						peer := net.JoinHostPort(node.Addr.String(), strconv.Itoa(int(meta.ServicePort)))
						br.manifestCache.AddPeer(fmt.Sprintf("https://%s", peer), node.Name)
						br.logger.Debugf("Added groupcache peer %s", peer)
					case gossip.PluginInstance:
						br.logger.Infof("Register plugin")
						if err := br.pluginManager.register(node, meta); err != nil {
							br.logger.Errorf("plugin register error: %s", err)
						}
					}
				}
			}
		case gossip.NodeJoin:
			node, ok := event.Arg.(*memberlist.Node)
			if !ok || self.Name == node.Name {
				continue
			}

			br.logger.Debugf("Added node %s to cluster", node.Addr)

			if self.Port != node.Port || !self.Addr.Equal(node.Addr) {
				meta := gossip.NewBeskarMeta()
				if err := meta.Decode(node.Meta); err == nil && meta.Ready {
					switch meta.InstanceType {
					case gossip.BeskarInstance:
						peer := net.JoinHostPort(node.Addr.String(), strconv.Itoa(int(meta.ServicePort)))
						br.manifestCache.AddPeer(fmt.Sprintf("https://%s", peer), node.Name)
						br.logger.Debugf("Added groupcache peer %s", peer)
					case gossip.PluginInstance:
						br.logger.Infof("Register plugin")
						if err := br.pluginManager.register(node, meta); err != nil {
							br.logger.Errorf("plugin register error: %s", err)
						}
					}
				}
			}
		case gossip.NodeLeave:
			node, ok := event.Arg.(*memberlist.Node)
			if !ok || self.Name == node.Name {
				continue
			}

			br.logger.Debugf("Removed node %s from cluster", node.Addr)

			if self.Port != node.Port || !self.Addr.Equal(node.Addr) {
				meta := gossip.NewBeskarMeta()
				if err := meta.Decode(node.Meta); err == nil && meta.Ready {
					switch meta.InstanceType {
					case gossip.BeskarInstance:
						peer := net.JoinHostPort(node.Addr.String(), strconv.Itoa(int(meta.ServicePort)))
						br.manifestCache.RemovePeer(fmt.Sprintf("https://%s", peer), node.Name)
						br.logger.Debugf("Removed groupcache peer %s", peer)
					case gossip.PluginInstance:
						br.logger.Infof("Unregister plugin")
						br.pluginManager.unregister(node, meta)
					}
				}
			}
		}
	}
}

func (br *Registry) initGossip() (_ *mtls.CAPEM, errFn error) {
	br.logger.Info("Initializing gossip")

	_, port, err := net.SplitHostPort(br.beskarConfig.Cache.Addr)
	if err != nil {
		return nil, err
	}
	cachePort, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, err
	}

	_, port, err = net.SplitHostPort(br.beskarConfig.Registry.HTTP.Addr)
	if err != nil {
		return nil, err
	}
	registryPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, err
	}

	meta := gossip.NewBeskarMeta()
	meta.ServicePort = uint16(cachePort)
	meta.RegistryPort = uint16(registryPort)
	meta.InstanceType = gossip.BeskarInstance
	meta.Hostname = br.beskarConfig.Hostname

	br.member, err = gossip.Start(br.beskarConfig.Gossip, meta, nil, 300*time.Second)
	if err != nil {
		return nil, err
	}
	defer func() {
		if errFn != nil {
			_ = br.member.Shutdown()
		}
	}()

	remoteState, err := br.member.RemoteState()
	if err != nil {
		if len(br.member.Nodes()) == 0 {
			return nil, err
		}
		remoteState, err = br.member.LocalState()
		if err != nil {
			return nil, err
		}
	}

	caPem, err := mtls.UnmarshalCAPEM(remoteState)
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling CA certificates: %w", err)
	}

	return caPem, nil
}

func (br *Registry) getHashedHostname() string {
	//nolint:gosec
	return fmt.Sprintf("%x", md5.Sum([]byte(br.beskarConfig.Hostname)))
}

func (br *Registry) initMTLS(ln net.Listener, caPEM *mtls.CAPEM, tlsConfig *tls.Config) error {
	localIPs, err := netutil.LocalIPs()
	if err != nil {
		return err
	}

	hashedHostname := br.getHashedHostname()

	registryServerConfig, err := mtls.GenerateServerConfig(
		bytes.NewReader(caPEM.Cert),
		bytes.NewReader(caPEM.Key),
		time.Now().AddDate(10, 0, 0),
		mtls.WithCertRequestIPs(localIPs...),
		mtls.WithCertRequestHostnames(
			br.beskarConfig.Hostname,
			hashedHostname,
		),
	)
	if err != nil {
		return err
	}

	if tlsConfig == nil {
		tlsConfig = registryServerConfig
	} else {
		tlsConfig.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			if hello.ServerName == hashedHostname {
				return registryServerConfig, nil
			}
			return nil, nil
		}
	}

	if cln, ok := ln.(*cmux.Listener); ok {
		cln.SetTLSConfig(tlsConfig)
	}

	return nil
}

func (br *Registry) initManifestCache(caPEM *mtls.CAPEM) (_ *groupcache.Group, errFn error) {
	cacheClientConfig, err := mtls.GenerateClientConfig(
		bytes.NewReader(caPEM.Cert),
		bytes.NewReader(caPEM.Key),
		time.Now().AddDate(10, 0, 0),
	)
	if err != nil {
		return nil, fmt.Errorf("while generating cache client mTLS certificates: %w", err)
	}

	localIPs, err := netutil.LocalIPs()
	if err != nil {
		return nil, err
	}

	cacheServerConfig, err := mtls.GenerateServerConfig(
		bytes.NewReader(caPEM.Cert),
		bytes.NewReader(caPEM.Key),
		time.Now().AddDate(10, 0, 0),
		mtls.WithCertRequestIPs(localIPs...),
	)
	if err != nil {
		return nil, fmt.Errorf("while generating cache server mTLS certificates: %w", err)
	}

	_, port, err := net.SplitHostPort(br.beskarConfig.Cache.Addr)
	if err != nil {
		return nil, err
	}
	cacheAddr := fmt.Sprintf("https://%s", net.JoinHostPort(br.member.LocalNode().Addr.String(), port))

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = cacheClientConfig
	transport.MaxIdleConnsPerHost = 16
	transport.ResponseHeaderTimeout = 2 * time.Second

	br.manifestCache = cache.NewCache(cacheAddr, &groupcache.HTTPPoolOptions{
		Transport: func(context.Context) http.RoundTripper {
			return transport
		},
	})

	go func() {
		err = br.manifestCache.Start(cacheServerConfig)
		if err != nil {
			br.errCh <- err
		}
	}()

	return br.manifestCache.NewGroup("manifests", cache.DefaultCacheSize, cacheGetter{
		registry: br.registry,
	})
}

func (br *Registry) Serve(ctx context.Context, ln net.Listener) (errFn error) {
	br.logger.Info("Starting beskar server")

	tlsConfig, err := br.getTLSConfig(ctx)
	if err != nil {
		return err
	} else if tlsConfig != nil {
		dcontext.GetLogger(ctx).Infof("listening on %v, tls", ln.Addr())
		ln = tls.NewListener(ln, tlsConfig)
	} else {
		dcontext.GetLogger(ctx).Infof("listening on %v", ln.Addr())
		ln = cmux.NewListener(ln)
	}

	go func() {
		br.errCh <- br.server.Serve(ln)
	}()

	caPEM, err := br.initGossip()
	if err != nil {
		return err
	}
	defer func() {
		gossipErr := br.member.Shutdown()
		if errFn == nil {
			errFn = gossipErr
		}
	}()

	if err := br.initMTLS(ln, caPEM, tlsConfig); err != nil {
		return err
	}

	pluginClientConfig, err := mtls.GenerateClientConfig(
		bytes.NewReader(caPEM.Cert),
		bytes.NewReader(caPEM.Key),
		time.Now().AddDate(10, 0, 0),
	)
	if err != nil {
		return fmt.Errorf("while generating plugin client mTLS certificates: %w", err)
	}

	br.pluginManager.setClientTLSConfig(pluginClientConfig)

	manifestCache := &manifestCache{}

	br.router.NotFoundHandler = br.server.Handler
	br.server.Handler = br.router

	br.server.BaseContext = func(net.Listener) context.Context {
		return context.WithValue(ctx, &manifestCacheKey, manifestCache)
	}

	manifestCache.Group, err = br.initManifestCache(caPEM)
	if err != nil {
		return err
	}
	defer func() {
		manifestCacheErr := br.manifestCache.Stop(ctx)
		if errFn == nil {
			errFn = manifestCacheErr
		}
	}()

	go br.startGossipWatcher()

	if err := br.member.MarkAsReady(gossip.DefaultReadyTimeout); err != nil {
		return err
	}

	waitPlugins, err := loadPlugins(ctx)
	if err != nil {
		return err
	}

	err = br.wait(true)

	br.logger.Info("Stopping beskar server")

	waitPlugins()

	if err == nil && br.beskarConfig.Registry.HTTP.DrainTimeout > 0 {
		c, cancel := context.WithTimeout(context.Background(), br.beskarConfig.Registry.HTTP.DrainTimeout)
		err = br.server.Shutdown(c)
		cancel()
	}

	return err
}

func (br *Registry) Put(ctx context.Context, repository distribution.Repository, dgst digest.Digest, mediaType string, payload []byte) error {
	return br.sendEvent(
		ctx,
		&eventv1.EventPayload{
			Repository: repository.Named().String(),
			Digest:     dgst.String(),
			Mediatype:  mediaType,
			Payload:    payload,
			Action:     eventv1.Action_ACTION_PUT,
		},
	)
}

func (br *Registry) Delete(ctx context.Context, repository distribution.Repository, dgst digest.Digest, mediaType string, payload []byte) error {
	return br.sendEvent(
		ctx,
		&eventv1.EventPayload{
			Repository: repository.Named().String(),
			Digest:     dgst.String(),
			Mediatype:  mediaType,
			Payload:    payload,
			Action:     eventv1.Action_ACTION_DELETE,
		},
	)
}

func (br *Registry) sendEvent(ctx context.Context, event *eventv1.EventPayload) error {
	matches := artifactsMatch.FindStringSubmatch(event.Repository)
	if len(matches) < 2 {
		return nil
	}

	switch event.Mediatype {
	case "application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json":
		ociManifest, err := v1.ParseManifest(bytes.NewReader(event.Payload))
		if err != nil {
			return err
		}
		event.Mediatype = string(ociManifest.Config.MediaType)
		plugin, ok := br.pluginManager.getPlugin(event.Mediatype)
		if !ok {
			return nil
		}

		br.logger.Debugf("Sending manifest %s event to plugin", event.Repository)

		return plugin.sendEvent(ctx, event, nil)
	default:
	}

	return nil
}
