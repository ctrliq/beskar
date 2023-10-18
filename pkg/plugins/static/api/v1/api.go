// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package apiv1

import (
	"context"
)

type Page struct {
	Size  int
	Token string
}

// Repository logs.
type RepositoryLog struct {
	Level   string `json:"level"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// Repository files.
type RepositoryFile struct {
	Tag        string `json:"tag"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	UploadTime string `json:"upload_time"`
	Size       uint64 `json:"size"`
}

// Static is used for managing static file repositories.
// This is the API documentation of Static.
//
//kun:oas title=Static file Repository Management API
//kun:oas version=1.0.0
//kun:oas basePath=/artifacts/static/api/v1
//kun:oas docsPath=/doc/swagger.yaml
//kun:oas tags=static
type Static interface {
	// Delete a static repository.
	//kun:op DELETE /repository
	//kun:success statusCode=200
	DeleteRepository(ctx context.Context, repository string) (err error)

	// List static repository logs.
	//kun:op GET /repository/logs
	//kun:success statusCode=200
	ListRepositoryLogs(ctx context.Context, repository string, page *Page) (logs []RepositoryLog, err error)

	// Get file information by tag from static repository.
	//kun:op GET /repository/file:bytag
	//kun:success statusCode=200
	GetRepositoryFileByTag(ctx context.Context, repository string, tag string) (repositoryFile *RepositoryFile, err error)

	// Get file information by name from static repository.
	//kun:op GET /repository/file:byname
	//kun:success statusCode=200
	GetRepositoryFileByName(ctx context.Context, repository string, name string) (repositoryFile *RepositoryFile, err error)

	// Remove file from static repository.
	//kun:op DELETE /repository/file
	//kun:success statusCode=200
	RemoveRepositoryFile(ctx context.Context, repository string, tag string) (err error)

	// List files for a static repository.
	//kun:op GET /repository/file:list
	//kun:success statusCode=200
	ListRepositoryFiles(ctx context.Context, repository string, page *Page) (repositoryFiles []*RepositoryFile, err error)
}
