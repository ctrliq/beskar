// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirror

import (
	"context"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

type Semaphore struct {
	weighted atomic.Pointer[semaphore.Weighted]
	enabled  atomic.Bool
}

func NewWeighted(n int64) *Semaphore {
	s := &Semaphore{}
	s.weighted.Store(semaphore.NewWeighted(n))
	return s
}

func (s *Semaphore) Acquire(ctx context.Context, n int64, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.enabled.Store(true)

	if err := s.weighted.Load().Acquire(ctx, n); err != nil {
		s.enabled.Store(false)
		s.weighted.Store(semaphore.NewWeighted(n))
		return err
	}

	return nil
}

func (s *Semaphore) Release(n int64) {
	if s.enabled.Load() {
		s.weighted.Load().Release(n)
	}
}
