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
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/reference"
	"github.com/distribution/distribution/v3/registry"
	"github.com/distribution/distribution/v3/registry/auth"
	"github.com/distribution/distribution/v3/version"
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
	server        *registry.Registry
	member        *gossip.Member
	manifestCache *cache.GroupCache
	proxyPlugins  map[string]*proxyPlugin
	errCh         chan error
}

func New(beskarConfig *config.BeskarConfig) (context.Context, *Registry, error) {
	ctx := dcontext.WithVersion(dcontext.Background(), version.Version)

	beskarRegistry := &Registry{
		beskarConfig: beskarConfig,
		proxyPlugins: make(map[string]*proxyPlugin),
		errCh:        make(chan error, 1),
	}

	member, err := gossip.Start(beskarConfig, nil, 30*time.Second)
	if err != nil {
		return nil, nil, err
	}
	beskarRegistry.member = member

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

	cacheClientConfig, err := mtls.GenerateClientConfig(
		bytes.NewReader(caPem.Cert),
		bytes.NewReader(caPem.Key),
		time.Now().AddDate(10, 0, 0),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("while generating cache client mTLS certificates: %w", err)
	}

	localIPs, err := netutil.LocalIPs()
	if err != nil {
		return nil, nil, err
	}

	cacheServerConfig, err := mtls.GenerateServerConfig(
		bytes.NewReader(caPem.Cert),
		bytes.NewReader(caPem.Key),
		time.Now().AddDate(10, 0, 0),
		mtls.WithCertRequestIPs(localIPs...),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("while generating cache server mTLS certificates: %w", err)
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
			beskarRegistry.errCh <- err
		}
	}()

	cacheGroup, err := manifestCache.NewGroup("manifests", cache.DefaultCacheSize, cacheGetter{})
	if err != nil {
		return nil, nil, err
	}

	registryCh, err := registerRegistryMiddleware(beskarRegistry, cacheGroup)
	if err != nil {
		return nil, nil, err
	}

	if err := auth.Register("beskar", auth.InitFunc(newAccessController)); err != nil {
		return nil, nil, err
	}

	beskarRegistry.router = mux.NewRouter()

	registry.RegisterHandler(func(config *configuration.Configuration, handler http.Handler) http.Handler {
		beskarRegistry.router.NotFoundHandler = handler
		return beskarRegistry.router
	})

	beskarRegistry.server, err = registry.NewRegistry(ctx, beskarConfig.Registry.Configuration)
	if err != nil {
		return nil, nil, err
	}

	beskarRegistry.registry = <-registryCh

	if err := initPlugins(ctx, beskarRegistry); err != nil {
		return nil, nil, err
	}

	if beskarConfig.Profiling {
		beskarRegistry.setProfiling()
	}

	return ctx, beskarRegistry, nil
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

func (br *Registry) Serve(ctx context.Context) error {
	go func() {
		err := br.server.ListenAndServe()
		gossipErr := br.member.Shutdown()
		if err == nil {
			err = gossipErr
		}
		manifestCacheErr := br.manifestCache.Stop(ctx)
		if err == nil {
			err = manifestCacheErr
		}
		br.errCh <- err
	}()
	return <-br.errCh
}

func (br *Registry) Put(ctx context.Context, repository distribution.Repository, dgst digest.Digest, mediaType string, payload []byte) error {
	switch mediaType {
	case "application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json":
		ociManifest, err := v1.ParseManifest(bytes.NewReader(payload))
		if err != nil {
			return err
		}
		mediaType = string(ociManifest.Config.MediaType)
		proxyPlugin, ok := br.proxyPlugins[mediaType]
		if !ok {
			return nil
		}

		return proxyPlugin.send(
			ctx,
			repository.Named().String(),
			mediaType,
			payload,
			dgst.String(),
		)
	default:
	}

	return nil
}

func (br *Registry) Delete(context.Context, digest.Digest) error {
	return nil
}
