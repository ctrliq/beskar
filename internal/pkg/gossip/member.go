// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package gossip

import (
	"fmt"
	"io"
	"time"

	"github.com/hashicorp/memberlist"
)

// Member represents a member part of a gossip cluster.
type Member struct {
	ml        *memberlist.Memberlist
	eventChan chan MemberEvent
	nd        *nodeDelegate
}

const (
	// DefaultLeaveTimeout is the time to wait during while leaving cluster
	DefaultLeaveTimeout = 5 * time.Second
	// DefaultReadyTimeout is the time to wait during ready status update
	DefaultReadyTimeout = 2 * time.Second
)

// NewMember creates and/or participates to a gossip cluster.
func NewMember(name string, peers []string, memberOpt ...MemberOption) (*Member, error) {
	cfg := memberlist.DefaultLANConfig()
	cfg.BindPort = 0
	cfg.Name = name

	eventChan := make(chan MemberEvent, 128)
	nd := &nodeDelegate{
		eventChan: eventChan,
	}
	cfg.Delegate = nd
	cfg.Events = nd

	cfg.LogOutput = io.Discard
	cfg.Keyring, _ = memberlist.NewKeyring(nil, nil)

	for _, opt := range memberOpt {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	// create memberlist network
	ml, err := memberlist.Create(cfg)
	if err != nil {
		return nil, err
	}

	member := &Member{
		ml:        ml,
		eventChan: eventChan,
		nd:        nd,
	}
	if len(peers) > 0 {
		peerJoined, err := member.join(peers)
		if err != nil {
			return nil, err
		} else if peerJoined == 0 {
			return nil, fmt.Errorf("no peer has been joined")
		}
	}

	return member, nil
}

// Watch returns an event channel to react to various notifications
// coming from gossip protocol (join, leave, update ...).
func (member *Member) Watch() <-chan MemberEvent {
	return member.eventChan
}

// Join joins a peer in the cluster.
func (member *Member) join(peers []string) (int, error) {
	if len(peers) == 0 {
		return 0, fmt.Errorf("at least one master peer address is required to join cluster")
	}

	count, err := member.ml.Join(peers)
	if err != nil {
		_ = member.ml.Shutdown()
		return 0, err
	}

	return count, err
}

// Shutdown leaves the cluster.
func (member *Member) Shutdown() error {
	if member == nil {
		return nil
	} else if member.ml == nil {
		return fmt.Errorf("no cluster joined")
	}
	if member.ml.NumMembers() > 0 {
		if err := member.ml.Leave(DefaultLeaveTimeout); err != nil {
			return err
		}
		return member.ml.Shutdown()
	}
	return nil
}

// Nodes returns all nodes participating to the cluster.
func (member *Member) Nodes() []*memberlist.Node {
	return member.ml.Members()
}

// LocalNode returns the current node information.
func (member *Member) LocalNode() *memberlist.Node {
	return member.ml.LocalNode()
}

// Send senda message to a particular node.
func (member *Member) Send(node *memberlist.Node, msg []byte) error {
	return member.ml.SendReliable(node, msg)
}

// MarkAsReady updates node metadata ready status and advertise nodes about change.
func (member *Member) MarkAsReady(timeout time.Duration) error {
	meta := NewBeskarMeta()
	if err := meta.Decode(member.nd.meta); err != nil {
		return fmt.Errorf("while decoding metadata")
	}
	meta.Ready = true
	mb, err := meta.Encode()
	if err != nil {
		return err
	}
	member.nd.meta = mb
	return member.ml.UpdateNode(timeout)
}

// PeerState returns the state of the peer used to join the cluster if any.
func (member *Member) RemoteState() ([]byte, error) {
	if member.nd.remoteState != nil {
		return member.nd.remoteState, nil
	}
	return nil, fmt.Errorf("no remote state received")
}

// LocalState returns the state of the node if any.
func (member *Member) LocalState() ([]byte, error) {
	if member.nd.localState != nil {
		return member.nd.localState, nil
	}
	return nil, fmt.Errorf("no local state set")
}
