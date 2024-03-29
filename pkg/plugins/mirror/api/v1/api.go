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

// Mirror content configurations.
type MirrorConfig struct {
	URL         string   `json:"url"`
	HTTPURL     string   `json:"http_url"`
	Destination string   `json:"destination"`
	Exclusions  []string `json:"exclusions"`
}

// Web content configuration.
type WebConfig struct {
	Prefix string `json:"prefix"`
}

// Repository properties/configuration.
type RepositoryProperties struct {
	// Configure the repository as a mirror.
	Mirror *bool `json:"mirror,omitempty"`
	// Mirror content configurations.
	MirrorConfigs []MirrorConfig `json:"mirror_configs,omitempty"`
	// Web content configuration.
	WebConfig *WebConfig `json:"web_config,omitempty"`
}

// Repository synchronization plan.
type RepositorySyncPlan struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

// Repository logs.
type RepositoryLog struct {
	Level   string `json:"level"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// Repository files.
type RepositoryFile struct {
	Tag           string `json:"tag"`
	Name          string `json:"name"`
	Reference     string `json:"reference"`
	Parent        string `json:"parent"`
	Link          string `json:"link"`
	LinkReference string `json:"link_reference"`
	ModifiedTime  string `json:"modified_time"`
	Mode          uint32 `json:"mode"`
	Size          uint64 `json:"size"`
	ConfigID      uint64 `json:"config_id"`
}

// Mirror sync status.
type SyncStatus struct {
	Syncing   bool   `json:"syncing"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	SyncError string `json:"sync_error"`
}

// Mirror is used for managing mirror repositories.
// This is the API documentation of Mirror.
//
//kun:oas title=Mirror Repository Management API
//kun:oas version=1.0.0
//kun:oas basePath=/artifacts/mirror/api/v1
//kun:oas docsPath=/doc/swagger.yaml
//kun:oas tags=mirror
type Mirror interface { //nolint:interfacebloat
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

	// Sync Mirror repository with an upstream repository using a specified config.
	//kun:op GET /repository/sync:config
	//kun:success statusCode=200
	SyncRepositoryWithConfig(ctx context.Context, repository string, mirrorConfigs []MirrorConfig, webConfig *WebConfig, wait bool) (err error)

	// Generate Mirror web pages .
	//kun:op GET /repository/generate:web
	//kun:success statusCode=200
	GenerateRepository(ctx context.Context, repository string) (err error)

	// Get Mirror repository sync status.
	//kun:op GET /repository/sync:status
	//kun:success statusCode=200
	GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *SyncStatus, err error)

	// Get Mirror repository sync plan.
	//kun:op GET /repository/sync:plan
	//kun:success statusCode=200
	GetRepositorySyncPlan(ctx context.Context, repository string) (syncPlan *RepositorySyncPlan, err error)

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

	// Get file count for a Mirror repository.
	//kun:op GET /repository/file:count
	//kun:success statusCode=200
	GetRepositoryFileCount(ctx context.Context, repository string) (count int, err error)

	// Delete file for a Mirror repository.
	//kun:op DELETE /repository/file
	//kun:success statusCode=200
	DeleteRepositoryFile(ctx context.Context, repository, file string) (err error)

	// Delete files by mode for a Mirror repository.
	//kun:op DELETE /repository/file:mode
	//kun:success statusCode=200
	DeleteRepositoryFilesByMode(ctx context.Context, repository string, mode uint32) (err error)
}
