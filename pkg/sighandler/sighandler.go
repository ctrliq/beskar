package sighandler

import (
	"context"
	"os"
	"os/signal"
)

type WaitFunc func() error

func New(errCh chan error, signals ...os.Signal) (context.Context, WaitFunc) {
	quit := make(chan os.Signal, 1)

	ctx, cancel := context.WithCancel(context.Background())

	// setup channel to get notified on SIGTERM signal
	signal.Notify(quit, signals...)

	return ctx, func() error {
		select {
		case <-ctx.Done():
		case <-quit:
			cancel()
		case err := <-errCh:
			cancel()
			return err
		}
		return nil
	}
}
