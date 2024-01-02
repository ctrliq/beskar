package backoff

import (
	"time"

	"github.com/cenkalti/backoff/v4"
)

func Retry(fn backoff.Operation) error {
	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 20 * time.Second
	eb.MaxInterval = 2 * time.Second
	return backoff.Retry(fn, eb)
}
