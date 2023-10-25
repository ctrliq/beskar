// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package httperror

import "github.com/RussellLuo/kun/pkg/werror"

type Werror = *werror.Error

type Error struct {
	Werror
}

func (e *Error) Is(target error) bool {
	if te, ok := target.(*werror.Error); ok {
		if te.Code == "" {
			return e.Code == te.Message
		}
		return e.Code == te.Code
	}
	return false
}
