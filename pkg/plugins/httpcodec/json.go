// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package httpcodec

import (
	"encoding/json"
	"io"

	"github.com/RussellLuo/kun/pkg/httpcodec"
	"github.com/RussellLuo/kun/pkg/werror"
	"go.ciq.dev/beskar/pkg/plugins/httperror"
)

var JSONCodec = httpcodec.NewDefaultCodecs(JSON{})

type JSON struct {
	httpcodec.JSON
}

func (j JSON) DecodeFailureResponse(body io.ReadCloser, out *error) error {
	var resp httpcodec.FailureResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return err
	}

	*out = &httperror.Error{
		Werror: &werror.Error{
			Code:    resp.Error.Code,
			Message: resp.Error.Message,
		},
	}

	return nil
}
