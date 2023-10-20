// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package apiv1

import (
	"context"
	"regexp"
)

const RepositoryRegex = "^(artifacts/yum/[a-z0-9]+(?:[/._-][a-z0-9]+)*)$"

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
	// GPG Public Key to check package signatures.
	GPGKey []byte `json:"gpg_key,omitempty"`
}

// Repository logs.
type RepositoryLog struct {
	Level   string `json:"level"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// Repository packages.
type RepositoryPackage struct {
	Tag          string `json:"tag"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	UploadTime   string `json:"upload_time"`
	BuildTime    string `json:"build_time"`
	Size         uint64 `json:"size"`
	Architecture string `json:"architecture"`
	SourceRPM    string `json:"source_rpm"`
	Version      string `json:"version"`
	Release      string `json:"release"`
	Groups       string `json:"groups"`
	License      string `json:"license"`
	Vendor       string `json:"vendor"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	Verified     bool   `json:"verified"`
	GPGSignature string `json:"gpg_signature"`
}

// Mirror sync status.
type SyncStatus struct {
	Syncing        bool   `json:"syncing"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	TotalPackages  int    `json:"total_packages"`
	SyncedPackages int    `json:"synced_packages"`
	SyncError      string `json:"sync_error"`
}

// YUM is used for managing YUM repositories.
// This is the API documentation of YUM.
//
//kun:oas title=Yum Repository Management API
//kun:oas version=1.0.0
//kun:oas basePath=/artifacts/yum/api/v1
//kun:oas docsPath=/doc/swagger.yaml
//kun:oas tags=yum
type YUM interface {
	// Create a YUM repository.
	//kun:op POST /repository
	//kun:success statusCode=200
	CreateRepository(ctx context.Context, repository string, properties *RepositoryProperties) (err error)

	// Delete a YUM repository.
	//kun:op DELETE /repository
	//kun:success statusCode=200
	DeleteRepository(ctx context.Context, repository string) (err error)

	// Update YUM repository properties.
	//kun:op PUT /repository
	//kun:success statusCode=200
	UpdateRepository(ctx context.Context, repository string, properties *RepositoryProperties) (err error)

	// Get YUM repository properties.
	//kun:op GET /repository
	//kun:success statusCode=200
	GetRepository(ctx context.Context, repository string) (properties *RepositoryProperties, err error)

	// Sync YUM repository with an upstream repository.
	//kun:op GET /repository/sync
	//kun:success statusCode=200
	SyncRepository(ctx context.Context, repository string) (err error)

	// Get YUM repository sync status.
	//kun:op GET /repository/sync:status
	//kun:success statusCode=200
	GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *SyncStatus, err error)

	// List YUM repository logs.
	//kun:op GET /repository/logs
	//kun:success statusCode=200
	ListRepositoryLogs(ctx context.Context, repository string, page *Page) (logs []RepositoryLog, err error)

	// Get RPM package from YUM repository.
	//kun:op GET /repository/package
	//kun:success statusCode=200
	GetRepositoryPackage(ctx context.Context, repository string, id string) (repositoryPackages *RepositoryPackage, err error)

	// Remove RPM package from YUM repository.
	//kun:op DELETE /repository/package
	//kun:success statusCode=200
	RemoveRepositoryPackage(ctx context.Context, repository string, id string) (err error)

	// List RPM packages for a YUM repository.
	//kun:op GET /repository/package:list
	//kun:success statusCode=200
	ListRepositoryPackages(ctx context.Context, repository string, page *Page) (repositoryPackages []*RepositoryPackage, err error)
}
