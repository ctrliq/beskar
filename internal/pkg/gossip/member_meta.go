// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package gossip

import (
	"bytes"
	"encoding/gob"
)

type InstanceType uint8

const (
	BeskarInstance InstanceType = iota + 1
	PluginInstance
)

type BeskarMeta struct {
	// Beskar or plugin instance.
	InstanceType InstanceType
	// Ready state
	Ready bool
	// Groupcache service port for beskar instances or HTTP service for plugin instances.
	ServicePort uint16
	// Registry service port for beskar instances.
	RegistryPort uint16
	// Hostname, provided as part of metadata as gossip is randomizing node name,
	// for beskar instances it will return the configured hostname, for plugins it returns
	// the node hostname
	Hostname string
}

func NewBeskarMeta() *BeskarMeta {
	return &BeskarMeta{}
}

func (bm *BeskarMeta) Encode() ([]byte, error) {
	b := new(bytes.Buffer)
	if err := gob.NewEncoder(b).Encode(bm); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Decode decodes gob format meta data and returns a NodeMeta
// corresponding structure.
func (bm *BeskarMeta) Decode(buf []byte) error {
	return gob.NewDecoder(bytes.NewReader(buf)).Decode(bm)
}
