// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package libostree

type Err string

func (e Err) Error() string {
	return string(e)
}

const (
	ErrInvalidPath = Err("invalid path")
)
