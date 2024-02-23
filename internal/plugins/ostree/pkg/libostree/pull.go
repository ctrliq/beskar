//nolint:goheader
// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package libostree

// #include <ostree.h>
// #include "pull.go.h"
import "C"

import (
	"context"
	"unsafe"
)

// Pull pulls refs from the named remote.
// Returns an error if the refs could not be fetched.
func (r *Repo) Pull(ctx context.Context, remote string, opts ...Option) error {
	cremote := C.CString(remote)
	defer C.free(unsafe.Pointer(cremote))

	options := toGVariant(opts...)
	defer C.g_variant_unref(options)

	var cErr *C.GError

	cCancel := C.g_cancellable_new()
	go func() {
		//nolint:gosimple
		for {
			select {
			case <-ctx.Done():
				C.g_cancellable_cancel(cCancel)
				return
			}
		}
	}()

	// Pull refs from remote
	if C.ostree_repo_pull_with_options(
		r.native,
		cremote,
		options,
		nil,
		cCancel,
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

	// TrustedHTTP - Don't verify checksums of objects HTTP repositories (Since: 2017.12)
	TrustedHTTP

	// None - No special options for pull
	None = 0
)

// Flags adds the given flags to the pull options.
func Flags(flags FlagSet) Option {
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		key := C.CString("flags")
		deferFree(unsafe.Pointer(key))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		cRefs := C.MakeRefArray(C.int(len(refs)))
		deferFree(unsafe.Pointer(cRefs))
		for i := 0; i < len(refs); i++ {
			cRef := C.CString(refs[i])
			deferFree(unsafe.Pointer(cRef))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		key := C.CString("gpg-verify-summary")
		deferFree(unsafe.Pointer(key))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		// 0 is the default depth so there is no need to add it to the builder.
		if depth == 0 {
			return
		}
		key := C.CString("depth")
		deferFree(unsafe.Pointer(key))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		key := C.CString("disable-static-deltas")
		deferFree(unsafe.Pointer(key))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		key := C.CString("require-static-deltas")
		deferFree(unsafe.Pointer(key))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		key := C.CString("dry-run")
		deferFree(unsafe.Pointer(key))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		// "" is the default so there is no need to add it to the builder.
		if appendUserAgent == "" {
			return
		}

		key := C.CString("append-user-agent")
		deferFree(unsafe.Pointer(key))
		cAppendUserAgent := C.CString(appendUserAgent)
		deferFree(unsafe.Pointer(cAppendUserAgent))
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
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		key := C.CString("n-network-retries")
		deferFree(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_int32(C.gint32(n))),
		)
	}
}

// MaxOutstandingFetcherRequests sets the max-outstanding-fetcher-requests option to the given value in the pull options.
// The max amount of concurrent connections allowed.
func MaxOutstandingFetcherRequests(n uint32) Option {
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		key := C.CString("max-outstanding-fetcher-requests")
		deferFree(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_new_uint32(C.guint32(n))),
		)
	}
}

// HTTPHeaders sets the http-headers option to the given value in the pull options.
// Additional HTTP headers to send with requests.
func HTTPHeaders(headers map[string]string) Option {
	return func(builder *C.GVariantBuilder, deferFree deferredFreeFn) {
		// Array of string tuples
		typeStr := C.CString("a(ss)")
		defer C.free(unsafe.Pointer(typeStr))
		variantType := C.g_variant_type_new(typeStr)

		// NOTE THE USE OF A NESTED BUILDER HERE - BE CAREFUL!
		// The builder is freed by g_variant_builder_end below.
		// See https://docs.gtk.org/glib/method.VariantBuilder.init.html
		var hdrBuilder C.GVariantBuilder
		C.g_variant_builder_init(&hdrBuilder, variantType)

		// Add headers to hdrBuilder (not builder)
		for key, value := range headers {
			gVariantBuilderAddStringTuple(
				&hdrBuilder,
				C.CString(key),
				C.CString(value),
			)
		}

		key := C.CString("http-headers")
		deferFree(unsafe.Pointer(key))
		gVariantBuilderAddVariant(
			builder,
			key,
			C.g_variant_new_variant(C.g_variant_builder_end(&hdrBuilder)),
		)
	}
}
