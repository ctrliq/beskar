// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package rv

import (
	"sort"
	"sync"

	"github.com/twmb/murmur3"
)

type Node struct {
	hostname string
	hostport string
	seed     uint64
}

func newNode(hostname, hostport string) *Node {
	return &Node{
		hostname: hostname,
		hostport: hostport,
		seed:     murmur3.StringSum64(hostname),
	}
}

func (n Node) hash(key string) uint64 {
	return murmur3.SeedStringSum64(n.seed, key)
}

func (n Node) Hostname() string {
	return n.hostname
}

func (n Node) Hostport() string {
	return n.hostport
}

type NodeKeyChangeFunc func(oldNode, newNode *Node)

type OnUpdateFunc func() (chan []string, NodeKeyChangeFunc)

type NodeHash struct {
	nodexMutex   sync.RWMutex
	nodes        []*Node
	onUpdateFunc OnUpdateFunc
}

func NewNodeHash(onUpdateFunc OnUpdateFunc) *NodeHash {
	return &NodeHash{
		nodes:        make([]*Node, 0),
		onUpdateFunc: onUpdateFunc,
	}
}

func (h *NodeHash) Add(hostname, hostport string) {
	h.nodexMutex.Lock()

	for _, node := range h.nodes {
		if node.hostname == hostname {
			h.nodexMutex.Unlock()
			return
		}
	}

	h.nodes = append(h.nodes, newNode(hostname, hostport))

	sort.SliceStable(h.nodes, func(i, j int) bool {
		return h.nodes[i].seed < h.nodes[j].seed
	})

	if h.onUpdateFunc != nil {
		keyCh, keyChangeFn := h.onUpdateFunc()

		for keys := range keyCh {
			for _, key := range keys {
				go func(key string) {
					node1, node2 := h.getNodes(key)
					if node1.hostport == hostport {
						keyChangeFn(node2, node1)
					}
				}(key)
			}
		}
	}

	h.nodexMutex.Unlock()
}

func (h *NodeHash) Remove(hostname string) {
	h.nodexMutex.Lock()

	var node *Node
	var nodeIndex int

	for idx, n := range h.nodes {
		if n.hostname == hostname {
			node = n
			nodeIndex = idx
			break
		}
	}

	if node == nil {
		h.nodexMutex.Unlock()
		return
	}

	if h.onUpdateFunc != nil {
		keyCh, keyChangeFn := h.onUpdateFunc()

		for keys := range keyCh {
			for _, key := range keys {
				go func(key string) {
					node1, node2 := h.getNodes(key)
					if node1.hostport == node.hostport {
						keyChangeFn(node1, node2)
					}
				}(key)
			}
		}
	}

	h.nodes[nodeIndex] = h.nodes[len(h.nodes)-1]
	h.nodes = h.nodes[:len(h.nodes)-1]

	sort.SliceStable(h.nodes, func(i, j int) bool {
		return h.nodes[i].seed < h.nodes[j].seed
	})

	h.nodexMutex.Unlock()
}

func (h *NodeHash) Get(key string) *Node {
	var node *Node

	h.nodexMutex.RLock()
	defer h.nodexMutex.RUnlock()

	maxHash := uint64(0)

	for _, n := range h.nodes {
		if hash := n.hash(key); hash > maxHash {
			maxHash = hash
			node = n
		}
	}

	return node
}

func (h *NodeHash) getNodes(key string) (*Node, *Node) {
	var node1, node2 *Node

	h.nodexMutex.RLock()
	defer h.nodexMutex.RUnlock()

	maxHash := uint64(0)

	for _, n := range h.nodes {
		if hash := n.hash(key); hash > maxHash {
			maxHash = hash
			node2, node1 = node1, n
		}
	}

	return node1, node2
}
