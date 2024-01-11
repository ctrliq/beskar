package ostree

// #cgo pkg-config: ostree-1 glib-2.0 gobject-2.0
// #include <stdlib.h>
// #include <glib.h>
// #include <ostree.h>
// #include "pull.go.h"
import "C"
import (
	"context"
	"unsafe"
)

// Pull pulls refs from the named remote.
// Returns an error if the refs could not be fetched.
func (r *Repo) Pull(_ context.Context, remote string, opts ...Option) error {
	cremote := C.CString(remote)
	defer C.free(unsafe.Pointer(cremote))

	options := toGVariant(opts...)
	defer C.g_variant_unref(options)

	var cErr *C.GError

	// Pull refs from remote
	// TODO: Implement cancellable so that we can cancel the pull if needed.
	if C.ostree_repo_pull_with_options(
		r.native,
		cremote,
		options,
		nil,
		nil,
		&cErr,
	) == C.gboolean(0) {
		return GoError(cErr)
	}

	return nil
}

type FlagSet int

const (
	// Mirror - Write out refs suitable for mirrors and fetch all refs if none requested
	Mirror = 1 << iota

	// CommitOnly - Fetch only the commit metadata
	CommitOnly

	// Untrusted - Do verify checksums of local (filesystem-accessible) repositories (defaults on for HTTP)
	Untrusted

	// BaseUserOnlyFiles - Since 2017.7.  Reject writes of content objects with modes outside of 0775.
	BaseUserOnlyFiles

	// TrustedHttp - Don't verify checksums of objects HTTP repositories (Since: 2017.12)
	TrustedHttp

	// None - No special options for pull
	None = 0
)

// Flags adds the given flags to the pull options.
func Flags(flags FlagSet) Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		key := C.CString("flags")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_int32(C.gint32(flags))),
		)
	}
}

// Refs adds the given refs to the pull options.
// When pulling refs from a remote, only the specified refs will be pulled.
func Refs(refs ...string) Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		cRefs := C.MakeRefArray(C.int(len(refs)))
		free(unsafe.Pointer(cRefs))
		for i := 0; i < len(refs); i++ {
			cRef := C.CString(refs[i])
			free(unsafe.Pointer(cRef))
			C.AppendRef(cRefs, C.int(i), cRef)
		}
		C.g_variant_builder_add_refs(
			builder,
			cRefs,
		)
	}
}

// NoGPGVerifySummary sets the gpg-verify-summary option to false in the pull options.
func NoGPGVerifySummary() Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		key := C.CString("gpg-verify-summary")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_boolean(C.gboolean(0))),
		)
	}
}

// Depth sets the depth option to the given value in the pull options.
// How far in the history to traverse; default is 0, -1 means infinite
func Depth(depth int) Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		// 0 is the default depth so there is no need to add it to the builder.
		if depth != 0 {
			return
		}
		key := C.CString("depth")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_int32(C.gint32(depth))),
		)
	}
}

// DisableStaticDelta sets the disable-static-deltas option to true in the pull options.
// Do not use static deltas.
func DisableStaticDelta() Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		key := C.CString("disable-static-deltas")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_boolean(C.gboolean(1))),
		)
	}
}

// RequireStaticDelta sets the require-static-deltas option to true in the pull options.
// Require static deltas.
func RequireStaticDelta() Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		key := C.CString("require-static-deltas")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_boolean(C.gboolean(1))),
		)
	}
}

// DryRun sets the dry-run option to true in the pull options.
// Only print information on what will be downloaded (requires static deltas).
func DryRun() Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		key := C.CString("dry-run")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_boolean(C.gboolean(1))),
		)
	}
}

// AppendUserAgent sets the append-user-agent option to the given value in the pull options.
// Additional string to append to the user agent.
func AppendUserAgent(appendUserAgent string) Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		// "" is the default so there is no need to add it to the builder.
		if appendUserAgent == "" {
			return
		}

		key := C.CString("append-user-agent")
		free(unsafe.Pointer(key))
		cAppendUserAgent := C.CString(appendUserAgent)
		free(unsafe.Pointer(cAppendUserAgent))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_string(cAppendUserAgent)),
		)
	}
}

// NetworkRetries sets the n-network-retries option to the given value in the pull options.
// Number of times to retry each download on receiving.
func NetworkRetries(n int) Option {
	return func(builder *C.GVariantBuilder, free freeFunc) {
		key := C.CString("n-network-retries")
		free(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_int32(C.gint32(n))),
		)
	}
}
