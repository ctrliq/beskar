// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package cmux

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync/atomic"
)

type conn struct {
	net.Conn
	buffered bool
	b        [1]byte
	eof      bool
}

func newConn(nc net.Conn) (*conn, error) {
	c := &conn{
		Conn: nc,
	}
	if n, err := c.Conn.Read(c.b[:1]); err != nil {
		if errors.Is(err, io.EOF) {
			c.eof = true
			return c, nil
		}
		return nil, err
	} else if n == 0 {
		return nil, io.ErrUnexpectedEOF
	} else if n != 1 {
		return nil, io.ErrShortBuffer
	}
	c.buffered = true
	return c, nil
}

func (c *conn) isTLS() bool {
	return c.b[0] == 0x16
}

func (c *conn) Read(p []byte) (int, error) {
	if c.buffered {
		if len(p) > 0 {
			c.buffered = !c.buffered
			p[0] = c.b[0]
			n, err := c.Conn.Read(p[1:])
			if err != nil {
				return 0, err
			}
			return n + 1, nil
		}
	} else if c.eof {
		return 0, io.EOF
	}
	return c.Conn.Read(p)
}

type Listener struct {
	net.Listener
	tlsConfig atomic.Pointer[tls.Config]
}

func NewListener(ln net.Listener) *Listener {
	return &Listener{
		Listener: ln,
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	nc, err := newConn(c)
	if err != nil {
		return nil, err
	} else if nc.isTLS() {
		if tlsConfig := l.tlsConfig.Load(); tlsConfig != nil {
			return tls.Server(nc, tlsConfig), nil
		}
	}
	return nc, nil
}

func (l *Listener) SetTLSConfig(tlsConfig *tls.Config) {
	l.tlsConfig.Store(tlsConfig)
}
