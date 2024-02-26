// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"time"
)

const (
	DefaultSyncTimeout        = time.Hour
	DefaultSyncMaxWorkerCount = 10
)

type SyncConfig struct {
	Timeout        time.Duration `yaml:"timeout"`
	MaxWorkerCount int           `yaml:"max_worker_count"`
}

func (sc *SyncConfig) GetTimeout() time.Duration {
	if sc.Timeout <= 0 {
		return DefaultSyncTimeout
	}

	return sc.Timeout
}

func (sc *SyncConfig) GetMaxWorkerCount() int {
	if sc.MaxWorkerCount <= 0 {
		return DefaultSyncMaxWorkerCount
	}

	return sc.MaxWorkerCount
}
