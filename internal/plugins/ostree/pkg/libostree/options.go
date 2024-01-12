package libostree

// #cgo pkg-config: ostree-1 glib-2.0 gobject-2.0
// #include <stdlib.h>
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include <ostree.h>
// #include "options.go.h"
import "C"
import "unsafe"

// Option defines an option for pulling ostree repos.
// It is used to build a *C.GVariant via a *C.GVariantBuilder.
// free is an optional function that frees the memory allocated by the option. free may be called more than once.
type Option func(builder *C.GVariantBuilder, free freeFunc)
type freeFunc func(...unsafe.Pointer)

// ToGVariant converts the given Options to a GVariant using a GVaraintBuilder.
func toGVariant(opts ...Option) *C.GVariant {

	typeStr := (*C.gchar)(C.CString("a{sv}"))
	defer C.free(unsafe.Pointer(typeStr))

	variantType := C.g_variant_type_new(typeStr)

	// The builder is freed by g_variant_builder_end below.
	// See https://docs.gtk.org/glib/method.VariantBuilder.init.html
	var builder C.GVariantBuilder
	C.g_variant_builder_init(&builder, variantType)

	// Collect pointers to free later
	var toFree []unsafe.Pointer
	freeFn := func(ptrs ...unsafe.Pointer) {
		toFree = append(toFree, ptrs...)
	}

	for _, opt := range opts {
		opt(&builder, freeFn)
	}
	defer func() {
		for i := 0; i < len(toFree); i++ {
			C.free(toFree[i])
		}
	}()

	variant := C.g_variant_builder_end(&builder)
	return C.g_variant_ref_sink(variant)
}

func gVariantBuilderAddVariant(builder *C.GVariantBuilder, key *C.gchar, variant *C.GVariant) {
	C.g_variant_builder_add_variant(builder, key, variant)
}

// NoGPGVerify sets the gpg-verify option to false in the pull options.
func NoGPGVerify() Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		key := C.CString("gpg-verify")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_boolean(C.gboolean(0))),
		)
	}
}