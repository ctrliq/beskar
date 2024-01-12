// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package gossip

type Config struct {
	Addr  string   `yaml:"addr"`
	Key   string   `yaml:"key"`
	Peers []string `yaml:"peers"`
}
