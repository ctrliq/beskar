// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package gossip

import (
	"github.com/hashicorp/memberlist"
)

const (
	// NodeJoin represents an event about a node join.
	NodeJoin memberlist.NodeEventType = iota << 1
	// NodeLeave represents an event about a node leave.
	NodeLeave
	// NodeUpdate represents an event about a node update.
	NodeUpdate
	// NodeMessage represents an event about a user message.
	NodeMessage
	// NodeError represents an event about a node error.
	NodeError
)

// MemberEvent
type MemberEvent struct {
	EventType memberlist.NodeEventType
	Arg       interface{}
}

// nodeDelegate regroups some hooks.
type nodeDelegate struct {
	meta        []byte
	eventChan   chan MemberEvent
	localState  []byte
	remoteState []byte
}

// NotifyMsg is called when a user-data message is received.
func (nd *nodeDelegate) NotifyMsg(b []byte) {
	nd.eventChan <- MemberEvent{
		EventType: NodeMessage,
		Arg:       b,
	}
}

// NotifyJoin is invoked when a node is detected to have joined.
func (nd *nodeDelegate) NotifyJoin(node *memberlist.Node) {
	nd.eventChan <- MemberEvent{
		EventType: NodeJoin,
		Arg:       node,
	}
}

// NotifyLeave is invoked when a node is detected to have left.
func (nd *nodeDelegate) NotifyLeave(node *memberlist.Node) {
	nd.eventChan <- MemberEvent{
		EventType: NodeLeave,
		Arg:       node,
	}
}

// NotifyUpdate is invoked when a node is detected to have
// updated, usually involving the meta data.
func (nd *nodeDelegate) NotifyUpdate(node *memberlist.Node) {
	nd.eventChan <- MemberEvent{
		EventType: NodeUpdate,
		Arg:       node,
	}
}

// NodeMeta is used to retrieve meta-data about the current node
// when broadcasting an alive message.
func (nd *nodeDelegate) NodeMeta(_ int) []byte { return nd.meta }

// GetBroadcasts is called when user data messages can be broadcast.
func (nd *nodeDelegate) GetBroadcasts(_, _ int) [][]byte { return nil }

// LocalState is used for a TCP Push/Pull.
func (nd *nodeDelegate) LocalState(join bool) []byte {
	if join && nd.localState != nil {
		return nd.localState
	}
	return nil
}

// MergeRemoteState is invoked after a TCP Push/Pull.
func (nd *nodeDelegate) MergeRemoteState(buf []byte, join bool) {
	if join && nd.remoteState == nil {
		if nd.localState == nil {
			nd.localState = buf
		}
		nd.remoteState = buf
	}
}
