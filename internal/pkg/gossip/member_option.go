// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package gossip

import (
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/memberlist"
)

// MemberOption defines a member configuration function.
type MemberOption func(*memberlist.Config) error

// WithSecretKey sets the secret key used to encrypt/decrypt communications.
func WithSecretKey(key []byte) MemberOption {
	return func(cfg *memberlist.Config) error {
		return cfg.Keyring.AddKey(key)
	}
}

// WithNodeMeta sets meta data associated to a node.
func WithNodeMeta(meta []byte) MemberOption {
	return func(cfg *memberlist.Config) error {
		nd, ok := cfg.Delegate.(*nodeDelegate)
		if !ok {
			return fmt.Errorf("no node delegate found")
		} else if len(meta) > memberlist.MetaMaxSize {
			return fmt.Errorf("meta data size exceed limit of %d bytes", memberlist.MetaMaxSize)
		}
		nd.meta = meta
		return nil
	}
}

func WithLocalState(state []byte) MemberOption {
	return func(cfg *memberlist.Config) error {
		nd, ok := cfg.Delegate.(*nodeDelegate)
		if !ok {
			return fmt.Errorf("no node delegate found")
		}
		nd.localState = state
		return nil
	}
}

// WithBindAddress sets the bind address to listen on.
func WithBindAddress(addr string) MemberOption {
	return func(cfg *memberlist.Config) error {
		ip, port, err := net.SplitHostPort(addr)
		if err != nil {
			return err
		}
		bindPort, err := strconv.Atoi(port)
		if err != nil {
			return err
		}
		cfg.BindAddr = ip
		cfg.BindPort = bindPort
		return nil
	}
}
