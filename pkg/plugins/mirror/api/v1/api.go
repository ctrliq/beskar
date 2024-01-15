// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package apiv1

import (
	"context"
	"regexp"
)

const (
	RepositoryRegex = "^(artifacts/mirror/[a-z0-9]+(?:[._-][a-z0-9]+)*)$"
	URLPath         = "/artifacts/mirror/api/v1"
)

var repositoryMatcher = regexp.MustCompile(RepositoryRegex)

func RepositoryMatch(repository string) bool {
	return repositoryMatcher.MatchString(repository)
}

type Page struct {
	Size  int
	Token string
}

// Repository properties/configuration.
type RepositoryProperties struct {
	// Configure the repository as a mirror.
	Mirror *bool `json:"mirror,omitempty"`
	// Mirror/Upstream URLs for mirroring.
	MirrorURLs []string `json:"mirror_urls,omitempty"`
}

// Repository logs.
type RepositoryLog struct {
	Level   string `json:"level"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// Repository files.
type RepositoryFile struct {
	Tag          string `json:"tag"`
	Name         string `json:"name"`
	Link         string `json:"link"`
	ModifiedTime string `json:"modified_time"`
	Mode         uint32 `json:"mode"`
	Size         uint64 `json:"size"`
}

// Mirror sync status.
type SyncStatus struct {
	Syncing     bool   `json:"syncing"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	TotalFiles  int    `json:"total_files"`
	SyncedFiles int    `json:"synced_files"`
	SyncError   string `json:"sync_error"`
}

// Mirror is used for managing mirror repositories.
// This is the API documentation of Mirror.
//
//kun:oas title=Mirror Repository Management API
//kun:oas version=1.0.0
//kun:oas basePath=/artifacts/mirror/api/v1
//kun:oas docsPath=/doc/swagger.yaml
//kun:oas tags=mirror
type Mirror interface {
	// Create a Mirror repository.
	//kun:op POST /repository
	//kun:success statusCode=200
	CreateRepository(ctx context.Context, repository string, properties *RepositoryProperties) (err error)

	// Delete a Mirror repository.
	//kun:op DELETE /repository
	//kun:success statusCode=200
	DeleteRepository(ctx context.Context, repository string, deleteFiles bool) (err error)

	// Update Mirror repository properties.
	//kun:op PUT /repository
	//kun:success statusCode=200
	UpdateRepository(ctx context.Context, repository string, properties *RepositoryProperties) (err error)

	// Get Mirror repository properties.
	//kun:op GET /repository
	//kun:success statusCode=200
	GetRepository(ctx context.Context, repository string) (properties *RepositoryProperties, err error)

	// Sync Mirror repository with an upstream repository.
	//kun:op GET /repository/sync
	//kun:success statusCode=200
	SyncRepository(ctx context.Context, repository string, wait bool) (err error)

	// Get Mirror repository sync status.
	//kun:op GET /repository/sync:status
	//kun:success statusCode=200
	GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *SyncStatus, err error)

	// List Mirror repository logs.
	//kun:op GET /repository/logs
	//kun:success statusCode=200
	ListRepositoryLogs(ctx context.Context, repository string, page *Page) (logs []RepositoryLog, err error)

	// List files for a Mirror repository.
	//kun:op GET /repository/file:list
	//kun:success statusCode=200
	ListRepositoryFiles(ctx context.Context, repository string, page *Page) (repositoryFiles []*RepositoryFile, err error)

	// Get file for a Mirror repository.
	//kun:op GET /repository/file
	//kun:success statusCode=200
	GetRepositoryFile(ctx context.Context, repository, file string) (repositoryFile *RepositoryFile, err error)
}
