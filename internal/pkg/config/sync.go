// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	DefaultSyncTimeout        = time.Hour
	DefaultSyncMaxWorkerCount = 10
)

type SyncConfig struct {
	Timeout        *durationpb.Duration `yaml:"timeout"`
	MaxWorkerCount int                  `yaml:"max_worker_count"`
}

func (sc *SyncConfig) GetTimeout() time.Duration {
	if sc.Timeout == nil {
		return DefaultSyncTimeout
	}

	if !sc.Timeout.IsValid() || sc.Timeout.GetSeconds() <= 0 {
		return DefaultSyncTimeout
	}

	return sc.Timeout.AsDuration()
}

func (sc *SyncConfig) GetMaxWorkerCount() int {
	if sc.MaxWorkerCount <= 0 {
		return DefaultSyncMaxWorkerCount
	}

	return sc.MaxWorkerCount
}
