// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package beskar

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/gorilla/mux"
	"github.com/hashicorp/memberlist"
	"go.ciq.dev/beskar/internal/pkg/gossip"
	"go.ciq.dev/beskar/internal/pkg/router"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	pluginv1 "go.ciq.dev/beskar/pkg/api/plugin/v1"
	"go.ciq.dev/beskar/pkg/rv"
	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/proto"
)

type nodeInfo struct {
	pluginName string
	nodeName   string
}

type pluginManager struct {
	pluginsMutex sync.RWMutex
	plugins      map[string]*plugin
	router       *mux.Router
	registry     distribution.Namespace
	reverseProxy *httputil.ReverseProxy
	nodesInfo    map[string]nodeInfo
	httpClient   *http.Client
}

func newPluginManager(registry distribution.Namespace, router *mux.Router, tlsConfig *tls.Config) *pluginManager {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	reverseProxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			target := *pr.In.URL
			target.Path = ""
			target.Scheme = "https"
			pr.SetURL(&target)
			hostport, ok := getReverseProxyHostport(pr.In.Context())
			if ok {
				pr.Out.Host = hostport
				pr.Out.URL.Host = hostport
			} else {
				pr.Out.Host = "127.0.0.1"
				pr.Out.URL.Host = "127.0.0.1"
			}
		},
	}

	return &pluginManager{
		plugins:      make(map[string]*plugin),
		registry:     registry,
		reverseProxy: reverseProxy,
		router:       router,
		nodesInfo:    make(map[string]nodeInfo),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   time.Minute,
		},
	}
}

func (pm *pluginManager) register(node *memberlist.Node, meta *gossip.BeskarMeta) error {
	hostport := net.JoinHostPort(node.Addr.String(), strconv.Itoa(int(meta.ServicePort)))
	info, err := pm.getPluginInfo(hostport)
	if err != nil {
		return err
	}

	pm.pluginsMutex.Lock()
	defer pm.pluginsMutex.Unlock()

	pl, ok := pm.plugins[info.Name]
	if !ok {
		mediaTypes := make(map[string]struct{})
		for _, mediaType := range info.Mediatypes {
			mediaTypes[mediaType] = struct{}{}
		}

		pl = &plugin{
			nodeHash:     rv.NewNodeHash(nil),
			version:      info.Version,
			name:         info.Name,
			mediaTypes:   mediaTypes,
			registry:     pm.registry,
			httpClient:   pm.httpClient,
			reverseProxy: pm.reverseProxy,
		}

		if err := pl.initRouter(info); err != nil {
			return err
		}

		pm.router.PathPrefix(info.Prefix).Handler(pl)

		pm.plugins[info.Name] = pl
	} else if semver.Compare(pl.version, info.Version) == -1 {
		// plugin update
		mediaTypes := make(map[string]struct{})
		for _, mediaType := range info.Mediatypes {
			mediaTypes[mediaType] = struct{}{}
		}
		pl.version = info.Version
		pl.mediaTypes = mediaTypes

		if err := pl.initRouter(info); err != nil {
			return err
		}
	}

	pm.nodesInfo[hostport] = nodeInfo{
		pluginName: info.Name,
		nodeName:   node.Name,
	}

	pl.nodeHash.Add(meta.Hostname, hostport)

	return nil
}

func (pm *pluginManager) unregister(node *memberlist.Node, meta *gossip.BeskarMeta) {
	hostport := net.JoinHostPort(node.Addr.String(), strconv.Itoa(int(meta.ServicePort)))

	pm.pluginsMutex.Lock()
	defer pm.pluginsMutex.Unlock()

	nodeInfo := pm.nodesInfo[hostport]

	pl, ok := pm.plugins[nodeInfo.pluginName]
	if ok && nodeInfo.nodeName == node.Name {
		pl.nodeHash.Remove(meta.Hostname)
		delete(pm.nodesInfo, hostport)
	}
}

func (pm *pluginManager) getPluginInfo(hostport string) (*pluginv1.Info, error) {
	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 5 * time.Second
	eb.MaxInterval = 500 * time.Millisecond

	info := new(pluginv1.Info)
	pluginURL := url.URL{
		Scheme: "https",
		Host:   hostport,
		Path:   "/info",
	}

	err := backoff.Retry(func() error {
		req, err := http.NewRequest(http.MethodGet, pluginURL.String(), nil)
		if err != nil {
			return err
		}
		req = req.WithContext(context.Background())
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := pm.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("plugin backend has returned an unknown status %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error during body read: %w", err)
		} else if err := proto.Unmarshal(data, info); err != nil {
			return fmt.Errorf("while unmarshalling plugin info data: %w", err)
		}

		return nil
	}, eb)
	if err != nil {
		return nil, fmt.Errorf("while getting plugin info from %s: %w", hostport, err)
	}

	return info, nil
}

func (pm *pluginManager) getPlugin(mediaType string) (*plugin, bool) {
	pm.pluginsMutex.Lock()
	defer pm.pluginsMutex.Unlock()

	for _, plugin := range pm.plugins {
		if _, ok := plugin.mediaTypes[mediaType]; ok {
			return plugin, true
		}
	}

	return nil, false
}

type plugin struct {
	nodeHash     *rv.NodeHash
	name         string
	version      string
	registry     distribution.Namespace
	mediaTypes   map[string]struct{}
	httpClient   *http.Client
	reverseProxy *httputil.ReverseProxy
	router       atomic.Pointer[router.RegoRouter]
}

func (p *plugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.RemoteAddr

	result, err := p.router.Load().Decision(r, p.registry)
	if err != nil {
		dcontext.GetLogger(r.Context()).Errorf("%s router decision error: %s", p.name, err)
		fmt.Println("WTF!!!!")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if !result.Found {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if result.RedirectURL != "" {
		http.Redirect(w, r, result.RedirectURL, http.StatusMovedPermanently)
		return
	} else if result.Repository != "" {
		key = result.Repository
	}

	node := p.nodeHash.Get(key)
	if node == nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	p.reverseProxy.ServeHTTP(w, setReverseProxyHostport(r, node.Hostport()))
}

func (p *plugin) sendEvent(ctx context.Context, event *eventv1.EventPayload, node *rv.Node) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return err
	}

	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 5 * time.Second
	eb.MaxInterval = 500 * time.Millisecond

	return backoff.Retry(func() error {
		destNode := node
		if destNode == nil {
			destNode = p.nodeHash.Get(event.Repository)
		}

		pluginURL := url.URL{
			Scheme: "https",
			Host:   destNode.Hostport(),
			Path:   "/event",
		}

		req, err := http.NewRequest(http.MethodPost, pluginURL.String(), bytes.NewReader(data))
		if err != nil {
			return err
		}
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("plugin backend has returned an unknown status %d", resp.StatusCode)
		}

		return nil
	}, backoff.WithContext(eb, ctx))
}

func (p *plugin) initRouter(info *pluginv1.Info) error {
	var routerOptions []router.RegoRouterOption

	if info.Router == nil {
		return nil
	}
	if len(info.Router.Data) > 0 {
		routerOptions = append(routerOptions, router.WithData(bytes.NewReader(info.Router.Data)))
	}
	rr, err := router.New(info.Name, string(info.Router.Rego), routerOptions...)
	if err != nil {
		return err
	}
	p.router.Store(rr)

	return nil
}

var reverseProxyHostportKey uint8

func setReverseProxyHostport(r *http.Request, hostport string) *http.Request {
	ctx := context.WithValue(r.Context(), &reverseProxyHostportKey, hostport)
	return r.WithContext(ctx)
}

func getReverseProxyHostport(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(&reverseProxyHostportKey).(string)
	return v, ok
}

func initPlugins(ctx context.Context) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	execPath := filepath.Dir(self)

	executable := filepath.Join(execPath, "beskar-yum")

	if _, err := os.Stat(executable); err == nil {
		cmd := exec.CommandContext(ctx, executable, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Cancel = func() error {
			return cmd.Process.Signal(syscall.SIGTERM)
		}
		cmd.WaitDelay = 5 * time.Second
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() {
			_ = cmd.Wait()
		}()
	}

	return nil
}
