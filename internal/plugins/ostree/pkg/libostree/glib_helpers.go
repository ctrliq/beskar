//nolint:goheader
// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package libostree

// #include "glib_helpers.go.h"
import "C"

import (
	"errors"
)

// GoError converts a C glib error to a Go error.
// The C error is freed after conversion.
func GoError(e *C.GError) error {
	defer C.g_error_free(e)

	if e == nil {
		return nil
	}
	return errors.New(C.GoString(C._g_error_get_message(e)))
}
