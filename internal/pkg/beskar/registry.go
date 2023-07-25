package beskar

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/reference"
	"github.com/distribution/distribution/v3/registry/auth"
	"github.com/distribution/distribution/v3/registry/handlers"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gorilla/mux"
	"github.com/hashicorp/memberlist"
	"github.com/mailgun/groupcache/v2"
	"github.com/opencontainers/go-digest"
	"go.ciq.dev/beskar/internal/pkg/cache"
	"go.ciq.dev/beskar/internal/pkg/config"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/pkg/mtls"
	"go.ciq.dev/beskar/pkg/netutil"

	// load distribution s3 storage driver
	_ "github.com/distribution/distribution/v3/registry/storage/driver/s3-aws"
)

type Registry struct {
	registry       distribution.Namespace
	beskarConfig   *config.BeskarConfig
	registryConfig *configuration.Configuration
	router         *mux.Router
	server         http.Server
	member         *gossip.Member
	manifestCache  *cache.GroupCache
	proxyPlugins   map[string]*proxyPlugin
}

func New(ctx context.Context, beskarConfig *config.BeskarConfig, registryConfig *configuration.Configuration, errCh chan error) (*Registry, error) {
	beskarRegistry := &Registry{
		beskarConfig:   beskarConfig,
		registryConfig: registryConfig,
		proxyPlugins:   make(map[string]*proxyPlugin),
	}

	// init gossip here
	member, err := gossip.Start(beskarConfig, nil, 30*time.Second)
	if err != nil {
		return nil, err
	}
	beskarRegistry.member = member

	remoteState, err := member.RemoteState()
	if err != nil {
		if len(member.Nodes()) == 0 {
			return nil, err
		}
		remoteState, err = member.LocalState()
		if err != nil {
			return nil, err
		}
	}

	caPem, err := mtls.UnmarshalCAPEM(remoteState)
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling CA certificates: %w", err)
	}

	cacheClientConfig, err := mtls.GenerateClientConfig(
		bytes.NewReader(caPem.Cert),
		bytes.NewReader(caPem.Key),
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
		bytes.NewReader(caPem.Cert),
		bytes.NewReader(caPem.Key),
		time.Now().AddDate(10, 0, 0),
		mtls.WithCertRequestIPs(localIPs...),
	)
	if err != nil {
		return nil, fmt.Errorf("while generating cache server mTLS certificates: %w", err)
	}

	cacheAddr := fmt.Sprintf("https://%s", beskarConfig.Cache.Addr)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = cacheClientConfig

	manifestCache := cache.NewCache(cacheAddr, &groupcache.HTTPPoolOptions{
		Transport: func(context.Context) http.RoundTripper {
			return transport
		},
	})
	beskarRegistry.manifestCache = manifestCache

	go beskarRegistry.startGossipWatcher()

	go func() {
		err = manifestCache.Start(cacheServerConfig)
		if err != nil {
			errCh <- err
		}
	}()

	cacheGroup, err := manifestCache.NewGroup("manifests", cache.DefaultCacheSize, cacheGetter{})
	if err != nil {
		return nil, err
	}

	registryCh, err := registerRegistryMiddleware(beskarRegistry, cacheGroup)
	if err != nil {
		return nil, err
	}

	if err := auth.Register("beskar", auth.InitFunc(newAccessController)); err != nil {
		return nil, err
	}

	app := handlers.NewApp(ctx, registryConfig)

	beskarRegistry.registry = <-registryCh

	beskarRegistry.router = mux.NewRouter()
	beskarRegistry.router.NotFoundHandler = app

	if err := initPlugins(ctx, beskarRegistry); err != nil {
		return nil, err
	}

	if beskarConfig.Profiling {
		beskarRegistry.setProfiling()
	}

	beskarRegistry.server = http.Server{
		Handler:           beskarRegistry.router,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 4 * time.Second,
	}

	return beskarRegistry, nil
}

func (br *Registry) setProfiling() {
	br.router.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	br.router.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	br.router.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	br.router.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	br.router.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	br.router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index)) // special handling for Gorilla mux
}

//nolint:unused // for later use
func (br *Registry) listBeskarTags(ctx context.Context) ([]string, error) {
	rn, err := reference.WithName("beskar")
	if err != nil {
		return nil, err
	}

	repo, err := br.registry.Repository(ctx, rn)
	if err != nil {
		return nil, err
	}

	tags, err := repo.Tags(ctx).All(ctx)
	if err != nil {
		//nolint:errorlint // error is not wrapped
		if _, ok := err.(distribution.ErrRepositoryUnknown); !ok {
			return nil, err
		}
	}

	return tags, nil
}

func (br *Registry) startGossipWatcher() {
	self := br.member.LocalNode()

	for event := range br.member.Watch() {
		switch event.EventType {
		case gossip.NodeJoin:
			node, ok := event.Arg.(*memberlist.Node)
			if !ok || self.Name == node.Name {
				continue
			}
			if self.Port != node.Port || !self.Addr.Equal(node.Addr) {
				meta := gossip.NewBeskarMeta()
				if err := meta.Decode(node.Meta); err == nil {
					peer := net.JoinHostPort(node.Addr.String(), strconv.Itoa(int(meta.CachePort)))
					br.manifestCache.AddPeer(fmt.Sprintf("https://%s", peer), node.Name)
				}
			}
		case gossip.NodeLeave:
			node, ok := event.Arg.(*memberlist.Node)
			if !ok || self.Name == node.Name {
				continue
			}
			if self.Port != node.Port || !self.Addr.Equal(node.Addr) {
				meta := gossip.NewBeskarMeta()
				if err := meta.Decode(node.Meta); err == nil {
					peer := net.JoinHostPort(node.Addr.String(), strconv.Itoa(int(meta.CachePort)))
					br.manifestCache.RemovePeer(fmt.Sprintf("https://%s", peer), node.Name)
				}
			}
		}
	}
}

func (br *Registry) Serve() error {
	ln, err := net.Listen("tcp", br.registryConfig.HTTP.Addr)
	if err != nil {
		return err
	}
	return br.server.Serve(ln)
}

func (br *Registry) Put(ctx context.Context, repository distribution.Repository, manifest distribution.Manifest, dgst digest.Digest) error {
	_, payload, err := manifest.Payload()
	if err != nil {
		return err
	}

	ociManifest, err := v1.ParseManifest(bytes.NewReader(payload))
	if err != nil {
		return err
	}

	mediatype := string(ociManifest.Config.MediaType)
	proxyPlugin, ok := br.proxyPlugins[mediatype]
	if !ok {
		return nil
	}

	return proxyPlugin.send(
		ctx,
		repository.Named().String(),
		mediatype,
		payload,
		dgst.String(),
	)
}

func (br *Registry) Delete(context.Context, digest.Digest) error {
	return nil
}
