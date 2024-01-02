// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostree

import (
	"context"
	"errors"
	"fmt"

	apiv1 "go.ciq.dev/beskar/pkg/plugins/ostree/api/v1"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
)

type apiService struct{}

func newAPIService() *apiService {
	return &apiService{}
}

func (o *apiService) MirrorRepository(_ context.Context, repository string, properties *apiv1.OSTreeRepositoryProperties) (err error) {
	fmt.Printf("Repo: %s,\nProperties: %v\n", repository, properties)
	return werror.Wrap(gcode.ErrNotImplemented, errors.New("repository mirroring not yet supported"))
}
