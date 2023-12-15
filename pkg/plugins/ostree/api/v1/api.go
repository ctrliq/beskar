// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
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

// OSTree is used for managing ostree repositories.
// This is the API documentation of OSTree.
//
//kun:oas title=OSTree Repository Management API
//kun:oas version=1.0.0
//kun:oas basePath=/artifacts/ostree/api/v1
//kun:oas docsPath=/doc/swagger.yaml
//kun:oas tags=ostree
type OSTree interface {
	// Mirror an ostree repository.
	//kun:op POST /repository/mirror
	//kun:success statusCode=200
	MirrorRepository(ctx context.Context, repository string, depth int) (err error)
}
