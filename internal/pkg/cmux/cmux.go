// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package cmux

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"syscall"
	"time"
)

type Conn struct {
	net.Conn
	buffered bool
	b        [1]byte
	readErr  error
}

func newConn(ctx context.Context, nc net.Conn) (*Conn, bool) {
	c := &Conn{
		Conn:     nc,
		buffered: true,
	}

	readFn := func() chan error {
		errCh := make(chan error, 1)
		defer close(errCh)

		if n, err := c.Conn.Read(c.b[:1]); err != nil {
			errCh <- err
		} else if n == 0 {
			errCh <- io.ErrUnexpectedEOF
		} else {
			errCh <- nil
		}

		return errCh
	}

	select {
	case <-ctx.Done():
		_ = nc.Close()
		c.readErr = context.Cause(ctx)
	case err := <-readFn():
		c.readErr = err
	}

	return c, c.b[0] == 0x16
}

func (c *Conn) Read(p []byte) (int, error) {
	if c.buffered {
		c.buffered = !c.buffered

		if len(p) == 0 {
			return 0, io.ErrShortBuffer
		} else if c.readErr != nil {
			return 0, c.readErr
		}

		p[0] = c.b[0]
		n, err := c.Conn.Read(p[1:])
		if err != nil {
			return 0, err
		}

		return n + 1, nil
	}

	return c.Conn.Read(p)
}

type Listener struct {
	net.Listener
	acceptCh  chan any
	closed    atomic.Bool
	tlsConfig atomic.Pointer[tls.Config]
}

func NewListener(ln net.Listener) *Listener {
	listener := &Listener{
		Listener: ln,
		acceptCh: make(chan any),
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					listener.closed.Store(true)
					cancel()
					close(listener.acceptCh)
					break
				}
				listener.acceptCh <- err
				continue
			}
			go listener.acceptConn(ctx, conn)
		}
	}()

	return listener
}

func (l *Listener) acceptConn(ctx context.Context, c net.Conn) {
	ctx, cancel := context.WithTimeoutCause(ctx, 100*time.Millisecond, syscall.ETIMEDOUT)
	nc, isTLS := newConn(ctx, c)
	cancel()

	if isTLS {
		if tlsConfig := l.tlsConfig.Load(); tlsConfig != nil {
			c = tls.Server(nc, tlsConfig)
		}
	} else {
		c = nc
	}

	if !l.closed.Load() {
		l.acceptCh <- c
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	in, ok := <-l.acceptCh
	if !ok {
		return nil, net.ErrClosed
	}

	if conn, ok := in.(net.Conn); ok {
		return conn, nil
	} else if err, ok := in.(error); ok {
		return nil, err
	}

	return nil, fmt.Errorf("wrong type received from accept channel")
}

func (l *Listener) SetTLSConfig(tlsConfig *tls.Config) {
	l.tlsConfig.Store(tlsConfig)
}
