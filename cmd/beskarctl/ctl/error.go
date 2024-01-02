// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ctl

import "fmt"

type Err string

func (e Err) Error() string {
	return string(e)
}

func Errf(str string, a ...any) Err {
	return Err(fmt.Sprintf(str, a...))
}
