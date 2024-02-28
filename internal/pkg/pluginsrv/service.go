// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package pluginsrv

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/hashicorp/memberlist"
	"go.ciq.dev/beskar/internal/pkg/cmux"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/pkg/log"
	"go.ciq.dev/beskar/internal/pkg/repository"
	pluginv1 "go.ciq.dev/beskar/pkg/api/plugin/v1"
	"go.ciq.dev/beskar/pkg/mtls"
	"go.ciq.dev/beskar/pkg/netutil"
)

type Config struct {
	Router *chi.Mux
	Gossip gossip.Config
	Info   *pluginv1.Info
}

type Service[H repository.Handler] interface {
	// Start starts the service's HTTP server.
	Start(http.RoundTripper, *mtls.CAPEM, *gossip.BeskarMeta) error

	// Context returns the service's context.
	Context() context.Context

	// Config returns the service's configuration.
	Config() Config

	// RepositoryManager returns the service's repository manager.
	// For plugin's without a repository manager, this method should return nil.
	RepositoryManager() *repository.Manager[H]
}

func Serve[H repository.Handler](ln net.Listener, service Service[H]) (errFn error) {
	ctx := service.Context()

	errCh := make(chan error)
	logger := log.GetContextLogger(ctx)
	if logger == nil {
		return fmt.Errorf("no logger found in service context")
	}
	slog.SetDefault(logger)

	serverListener := cmux.NewListener(ln)

	serviceConfig := service.Config()

	httpContext := log.SetContextAttrs(ctx, slog.String("context", "http"))

	server := http.Server{
		Handler:           serviceConfig.Router,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return httpContext
		},
	}

	_, servicePort, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return err
	}

	go func() {
		logger.Info("server started", "listen_addr", ln.Addr().String())
		errCh <- server.Serve(serverListener)
	}()

	gossipMember, caPEM, err := initGossip(servicePort, serviceConfig.Gossip)
	if err != nil {
		return err
	}
	defer func() {
		gossipErr := gossipMember.Shutdown()
		if errFn == nil {
			errFn = gossipErr
		}
	}()

	beskarMetaCh := startGossipWatcher(ctx, gossipMember)

	if err := setServerTLSConfig(serverListener, caPEM); err != nil {
		return err
	}

	repoManager := service.RepositoryManager()
	if repoManager != nil {
		// Gracefully shutdown repository handlers
		defer func() {
			var wg sync.WaitGroup
			for name, handler := range repoManager.GetAll() {
				wg.Add(1)

				go func(name string, handler repository.Handler) {
					logger.Info("stopping repository handler", "repository", name)
					handler.Stop()
					logger.Info("repository handler stopped", "repository", name)
					wg.Done()
				}(name, handler)
			}
			wg.Wait()
		}()
	}

	ticker := time.NewTicker(time.Second * 5)

	select {
	case <-ctx.Done():
		return nil
	case <-ticker.C:
		return fmt.Errorf("no beskar instance found")
	case beskarMeta := <-beskarMetaCh:
		ticker.Stop()

		wh := webHandler[H]{
			pluginInfo: serviceConfig.Info,
			manager:    repoManager,
		}

		serviceConfig.Router.With(IsTLSMiddleware).HandleFunc("/event", wh.event)
		serviceConfig.Router.With(IsTLSMiddleware).HandleFunc("/info", wh.info)

		transport, err := getBeskarTransport(caPEM, beskarMeta)
		if err != nil {
			return err
		}
		if err := service.Start(transport, caPEM, beskarMeta); err != nil {
			return err
		}
	}

	if err := gossipMember.MarkAsReady(gossip.DefaultReadyTimeout); err != nil {
		return err
	}

	var serverErr error

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server failure", "error", err.Error())
		}
		serverErr = err
	case <-ctx.Done():
		logger.Info("server shutdown")
	}

	_ = server.Shutdown(ctx)

	return serverErr
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

func startGossipWatcher(ctx context.Context, member *gossip.Member) <-chan *gossip.BeskarMeta {
	var hostnameOnce sync.Once
	beskarMetaCh := make(chan *gossip.BeskarMeta, 1)

	logger := log.GetContextLogger(ctx)

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

func initGossip(port string, gossipConfig gossip.Config) (_ *gossip.Member, _ *mtls.CAPEM, errFn error) {
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

	member, err := gossip.Start(gossipConfig, meta, nil, 300*time.Second)
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

func getBeskarTransport(caPEM *mtls.CAPEM, beskarMeta *gossip.BeskarMeta) (http.RoundTripper, error) {
	beskarTLSConfig, err := mtls.GenerateClientConfig(
		bytes.NewReader(caPEM.Cert),
		bytes.NewReader(caPEM.Key),
		time.Now().AddDate(10, 0, 0),
	)
	if err != nil {
		return nil, fmt.Errorf("while generating beskar client mTLS configuration: %w", err)
	}
	//nolint:gosec
	s := md5.Sum([]byte(beskarMeta.Hostname))
	beskarTLSConfig.ServerName = hex.EncodeToString(s[:])

	beskarAddr := net.JoinHostPort(beskarMeta.Hostname, strconv.Itoa(int(beskarMeta.RegistryPort)))

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 0
	transport.MaxIdleConnsPerHost = 16
	transport.IdleConnTimeout = 10 * time.Second

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == beskarAddr {
			// disable insecure connection to beskar
			return nil, syscall.ECONNREFUSED
		}

		return net.Dial(network, addr)
	}

	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		var tlsConfig *tls.Config

		if addr == beskarAddr {
			tlsConfig = beskarTLSConfig
		}

		conn, err := tls.Dial(network, addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		return conn, conn.HandshakeContext(ctx)
	}

	return transport, nil
}
