// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package gossip

import (
	"bytes"
	"encoding/gob"
)

type BeskarMeta struct {
	// Cache port.
	CachePort uint16
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
