// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package libostree

// #cgo pkg-config: ostree-1 glib-2.0 gobject-2.0
// #include <stdlib.h>
// #include <glib.h>
// #include <glib-object.h>
// #include <gio/gio.h>
// #include <ostree.h>
import "C"

import (
	"runtime"
	"unsafe"
)

// RepoMode - The mode to use when creating a new repo.
// If an unknown mode is passed, RepoModeBare will be used silently.
//
// See https://ostreedev.github.io/ostree/formats/#the-archive-format
// See https://ostreedev.github.io/ostree/formats/#aside-bare-formats
type RepoMode string

func (r RepoMode) toC() C.OstreeRepoMode {
	switch r {
	case RepoModeBare:
		return C.OSTREE_REPO_MODE_BARE
	case RepoModeArchive:
		return C.OSTREE_REPO_MODE_ARCHIVE
	case RepoModeArchiveZ2:
		return C.OSTREE_REPO_MODE_ARCHIVE_Z2
	case RepoModeBareUser:
		return C.OSTREE_REPO_MODE_BARE_USER
	case RepoModeBareUserOnly:
		return C.OSTREE_REPO_MODE_BARE_USER_ONLY
	case RepoModeBareSplitXAttrs:
		return C.OSTREE_REPO_MODE_BARE_SPLIT_XATTRS
	default:
		return C.OSTREE_REPO_MODE_BARE
	}
}

const (
	// RepoModeBare - The default mode. Keeps all file metadata. May require elevated privileges.
	// The bare repository format is the simplest one. In this mode regular files are directly stored to disk, and all
	// metadata (e.g. uid/gid and xattrs) is reflected to the filesystem. It allows further direct access to content and
	// metadata, but it may require elevated privileges when writing objects to the repository.
	RepoModeBare RepoMode = "bare"

	// RepoModeArchive - The archive format. Best for small storage footprint. Mostly used for server-side repositories.
	// The archive format simply gzip-compresses each content object. Metadata objects are stored uncompressed. This
	// means that it’s easy to serve via static HTTP. Note: the repo config file still uses the historical term
	// archive-z2 as mode. But this essentially indicates the modern archive format.
	//
	// When you commit new content, you will see new .filez files appearing in `objects/`.
	RepoModeArchive RepoMode = "archive"

	// RepoModeArchiveZ2 - Functionally equivalent to RepoModeArchive. Only useful for backwards compatibility.
	RepoModeArchiveZ2 RepoMode = "archive-z2"

	// RepoModeBareUser - Like RepoModeBare but ignore incoming uid/gid and xattrs.
	// The bare-user format is a bit special in that the uid/gid and xattrs from the content are ignored. This is
	// primarily useful if you want to have the same OSTree-managed content that can be run on a host system or an
	// unprivileged container.
	RepoModeBareUser RepoMode = "bare-user"

	// RepoModeBareUserOnly - Like RepoModeBareUser. No metadata stored. Only useful for checkouts. Does not need xattrs.
	// Same as BARE_USER, but all metadata is not stored, so it can only be used for user checkouts. Does not need xattrs.
	RepoModeBareUserOnly RepoMode = "bare-user-only"

	// RepoModeBareSplitXAttrs - Like RepoModeBare but store xattrs in a separate file.
	// Similarly, the bare-split-xattrs format is a special mode where xattrs are stored as separate repository objects,
	// and not directly reflected to the filesystem. This is primarily useful when transporting xattrs through lossy
	// environments (e.g. tar streams and containerized environments). It also allows carrying security-sensitive xattrs
	// (e.g. SELinux labels) out-of-band without involving OS filesystem logic.
	RepoModeBareSplitXAttrs RepoMode = "bare-split-xattrs"
)

type Repo struct {
	native *C.OstreeRepo
}

func fromNative(cRepo *C.OstreeRepo) *Repo {
	repo := &Repo{
		native: cRepo,
	}

	// Let the GB trigger free the cRepo for us when repo is freed.
	runtime.SetFinalizer(repo, func(r *Repo) {
		C.free(unsafe.Pointer(r.native))
	})

	return repo
}

// Init initializes & opens a new ostree repository at the given path.
//
//	Create the underlying structure on disk for the repository, and call
//	ostree_repo_open() on the result, preparing it for use.
//
//	Since version 2016.8, this function will succeed on an existing
//	repository, and finish creating any necessary files in a partially
//	created repository.  However, this function cannot change the mode
//	of an existing repository, and will silently ignore an attempt to
//	do so.
//
//	Since 2017.9, "existing repository" is defined by the existence of an
//	`objects` subdirectory.
//
//	This function predates ostree_repo_create_at(). It is an error to call
//	this function on a repository initialized via ostree_repo_open_at().
func Init(path string, mode RepoMode) (repo *Repo, err error) {
	if path == "" {
		return nil, ErrInvalidPath
	}

	cPathStr := C.CString(path)
	defer C.free(unsafe.Pointer(cPathStr))
	cPath := C.g_file_new_for_path(cPathStr)
	defer C.g_object_unref(C.gpointer(cPath))

	// Create a *C.OstreeRepo from the path
	cRepo := C.ostree_repo_new(cPath)
	defer func() {
		if err != nil {
			C.free(unsafe.Pointer(cRepo))
		}
	}()

	var cErr *C.GError

	if r := C.ostree_repo_create(cRepo, mode.toC(), nil, &cErr); r == C.gboolean(0) {
		return nil, GoError(cErr)
	}
	return fromNative(cRepo), nil
}

// Open opens an ostree repository at the given path.
func Open(path string) (*Repo, error) {
	if path == "" {
		return nil, ErrInvalidPath
	}

	cPathStr := C.CString(path)
	defer C.free(unsafe.Pointer(cPathStr))
	cPath := C.g_file_new_for_path(cPathStr)
	defer C.g_object_unref(C.gpointer(cPath))

	// Create a *C.OstreeRepo from the path
	cRepo := C.ostree_repo_new(cPath)

	var cErr *C.GError

	if r := C.ostree_repo_open(cRepo, nil, &cErr); r == C.gboolean(0) {
		return nil, GoError(cErr)
	}

	return fromNative(cRepo), nil
}

// AddRemote adds a remote to the repository.
func (r *Repo) AddRemote(name, url string, opts ...Option) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	options := toGVariant(opts...)
	defer C.g_variant_unref(options)

	var cErr *C.GError

	/*
		gboolean
		ostree_repo_remote_add(OstreeRepo *self,
		                       const char *name,
		                       const char *url,
		                       GVariant *options,
		                       GCancellable *cancellable,
		                       GError **error)
	*/
	if C.ostree_repo_remote_add(
		r.native,
		cName,
		cURL,
		options,
		nil,
		&cErr,
	) == C.gboolean(0) {
		return GoError(cErr)
	}

	return nil
}

// DeleteRemote deletes a remote from the repository.
func (r *Repo) DeleteRemote(name string) error {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var cErr *C.GError
	if C.ostree_repo_remote_delete(
		r.native,
		cName,
		nil,
		&cErr,
	) == C.gboolean(0) {
		return GoError(cErr)
	}

	return nil
}

// ReloadRemoteConfig reloads the remote configuration.
func (r *Repo) ReloadRemoteConfig() error {
	var cErr *C.GError

	if C.ostree_repo_reload_config(
		r.native,
		nil,
		&cErr,
	) == C.gboolean(0) {
		return GoError(cErr)
	}

	return nil
}

// ListRemotes lists the remotes in the repository.
func (r *Repo) ListRemotes() []string {
	var n C.guint
	remotes := C.ostree_repo_remote_list(
		r.native,
		&n,
	)

	var ret []string
	for {
		if *remotes == nil {
			break
		}
		ret = append(ret, C.GoString(*remotes))
		remotes = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(remotes)) + unsafe.Sizeof(uintptr(0))))
	}

	return ret
}

type ListRefsExtFlags int

const (
	ListRefsExtFlags_Aliases = 1 << iota
	ListRefsExtFlags_ExcludeRemotes
	ListRefsExtFlags_ExcludeMirrors
	ListRefsExtFlags_None ListRefsExtFlags = 0
)

type Ref struct {
	Name     string
	Checksum string
}

func (r *Repo) ListRefsExt(flags ListRefsExtFlags, prefix ...string) ([]Ref, error) {
	var cPrefix *C.char
	if len(prefix) > 0 {
		cPrefix = C.CString(prefix[0])
		defer C.free(unsafe.Pointer(cPrefix))
	}

	cFlags := (C.OstreeRepoListRefsExtFlags)(C.int(flags))

	var cErr *C.GError
	var outAllRefs *C.GHashTable
	if C.ostree_repo_list_refs_ext(
		r.native,
		cPrefix,
		&outAllRefs,
		cFlags,
		nil,
		&cErr,
	) == C.gboolean(0) {
		return nil, GoError(cErr)
	}

	// iter is freed when g_hash_table_iter_next returns false
	var iter C.GHashTableIter
	C.g_hash_table_iter_init(&iter, outAllRefs)

	var cRef, cChecksum C.gpointer
	var ret []Ref
	for C.g_hash_table_iter_next(&iter, &cRef, &cChecksum) == C.gboolean(1) {
		if cRef == nil {
			break
		}

		ref := (*C.OstreeCollectionRef)(unsafe.Pointer(&cRef))

		ret = append(ret, Ref{
			Name:     C.GoString((*C.char)((*C.gchar)(ref.ref_name))),
			Checksum: C.GoString((*C.char)(cChecksum)),
		})
	}

	return ret, nil
}

func (r *Repo) RegenerateSummary() error {
	var cErr *C.GError
	if C.ostree_repo_regenerate_summary(
		r.native,
		nil,
		nil,
		&cErr,
	) == C.gboolean(0) {
		return GoError(cErr)
	}

	return nil
}
