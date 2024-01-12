package libostree

// #cgo pkg-config: glib-2.0 gobject-2.0
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include <stdlib.h>
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
	return errors.New(C.GoString((*C.char)(C._g_error_get_message(e))))
}
