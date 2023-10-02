// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package sighandler

import (
	"context"
	"os"
	"os/signal"
)

type WaitFunc func(bool) error

func New(errCh chan error, signals ...os.Signal) (context.Context, WaitFunc) {
	quit := make(chan os.Signal, 1)

	ctx, cancel := context.WithCancel(context.Background())

	// setup channel to get notified on SIGTERM signal
	signal.Notify(quit, signals...)

	return ctx, func(returnOnCancel bool) error {
		for {
			select {
			case <-ctx.Done():
				if returnOnCancel {
					return nil
				}
			case <-quit:
				cancel()
			case err := <-errCh:
				cancel()
				return err
			}
		}
	}
}
