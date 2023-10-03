// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/mailgun/groupcache/v2"
)

const (
	// 64 MiB cache size by default
	DefaultCacheSize = 1024 * 1024 * 64
)

type GroupCache struct {
	peerMutex sync.Mutex
	peers     map[string]string
	pool      *groupcache.HTTPPool
	groups    map[string]*groupcache.Group
	self      string
	server    *http.Server
}

func NewCache(self string, options *groupcache.HTTPPoolOptions) *GroupCache {
	if options == nil {
		options = &groupcache.HTTPPoolOptions{}
	}

	pool := groupcache.NewHTTPPoolOpts(self, options)
	pool.Set(self)

	return &GroupCache{
		peers: map[string]string{
			self: "",
		},
		pool:   pool,
		self:   self,
		groups: make(map[string]*groupcache.Group),
	}
}

func (gc *GroupCache) Start(tlsConfig *tls.Config) error {
	u, err := url.Parse(gc.self)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return err
	}
	ln = tls.NewListener(ln, tlsConfig)
	gc.server = &http.Server{
		Handler:           gc.pool,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return gc.server.Serve(ln)
}

func (gc *GroupCache) Stop(ctx context.Context) error {
	if gc == nil {
		return nil
	}
	return gc.server.Shutdown(ctx)
}

func (gc *GroupCache) setPeers() {
	peers := make([]string, 0, len(gc.peers))

	for peer := range gc.peers {
		peers = append(peers, peer)
	}

	sort.Strings(peers)

	gc.pool.Set(peers...)
}

func (gc *GroupCache) AddPeer(peer string, name string) {
	gc.peerMutex.Lock()
	gc.peers[peer] = name
	gc.setPeers()
	gc.peerMutex.Unlock()
}

func (gc *GroupCache) RemovePeer(peer string, name string) {
	gc.peerMutex.Lock()
	v, ok := gc.peers[peer]
	if ok && v == name {
		delete(gc.peers, peer)
		gc.setPeers()
	}
	gc.peerMutex.Unlock()
}

func (gc *GroupCache) NewGroup(name string, cacheBytes int64, getter groupcache.Getter) (*groupcache.Group, error) {
	if group, ok := gc.groups[name]; ok {
		return group, nil
	}

	if getter == nil {
		return nil, fmt.Errorf("getter is nil")
	}

	group := groupcache.NewGroup(name, cacheBytes, getter)
	gc.groups[name] = group

	return group, nil
}
