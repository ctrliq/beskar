// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package apiv1

import (
	"context"
	"regexp"
)

const (
	RepositoryRegex = "^(artifacts/ostree/[a-z0-9]+(?:[/._-][a-z0-9]+)*)$"
	URLPath         = "/artifacts/ostree/api/v1"
)

var repositoryMatcher = regexp.MustCompile(RepositoryRegex)

func RepositoryMatch(repository string) bool {
	return repositoryMatcher.MatchString(repository)
}

type Page struct {
	Size  int
	Token string
}

type OSTreeRepositoryProperties struct {
	// Remotes - The remote repositories to mirror.
	Remotes []OSTreeRemoteProperties `json:"remotes"`
}

type OSTreeRemoteProperties struct {
	// Name - The name of the remote repository.
	Name string `json:"name"`

	// RemoteURL - The http url of the remote repository.
	RemoteURL string `json:"remote_url"`

	// GPGVerify - Whether to verify the GPG signature of the repository.
	NoGPGVerify bool `json:"no_gpg_verify"`
}

type OSTreeRepositorySyncRequest struct {
	// Remote - The name of the remote to sync.
	Remote string `json:"remote"`

	// Refs - The branches/refs to mirror. Leave empty to mirror all branches/refs.
	Refs []string `json:"refs"`

	// Depth - The depth of the mirror. Defaults is 0, -1 means infinite.
	Depth int `json:"depth"`
}

// Mirror sync status.
type SyncStatus struct {
	Syncing   bool   `json:"syncing"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	SyncError string `json:"sync_error"`

	// TODO: Implement these
	// The data for these is present when performing a pull via the ostree cli, so it is in the libostree code base.
	// SyncedMetadata int `json:"synced_metadata"`
	// SyncedObjects  int `json:"synced_objects"`
}

// OSTree is used for managing ostree repositories.
// This is the API documentation of OSTree.
//
//kun:oas title=OSTree Repository Management API
//kun:oas version=1.0.0
//kun:oas basePath=/artifacts/ostree/api/v1
//kun:oas docsPath=/doc/swagger.yaml
//kun:oas tags=ostree
type OSTree interface {
	// Create an OSTree repository.
	//kun:op POST /repository
	//kun:success statusCode=200
	CreateRepository(ctx context.Context, repository string, properties *OSTreeRepositoryProperties) (err error)

	// Delete a OSTree repository.
	//kun:op DELETE /repository
	//kun:success statusCode=202
	DeleteRepository(ctx context.Context, repository string) (err error)

	// Add a new remote to the OSTree repository.
	//kun:op POST /repository/remote
	//kun:success statusCode=200
	AddRemote(ctx context.Context, repository string, properties *OSTreeRemoteProperties) (err error)

	// Sync an ostree repository with one of the configured remotes.
	//kun:op POST /repository/sync
	//kun:success statusCode=202
	SyncRepository(ctx context.Context, repository string, properties *OSTreeRepositorySyncRequest) (err error)

	// Get OSTree repository sync status.
	//kun:op GET /repository/sync
	//kun:success statusCode=200
	GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *SyncStatus, err error)
}
