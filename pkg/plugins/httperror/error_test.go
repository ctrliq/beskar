// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package httperror

import (
	"net/http"
	"testing"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
)

func TestHTTPStatusCode(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name: "ErrInvalidArgument",
			err: &Error{
				Werror: gcode.FromCodeMessage(gcode.ErrInvalidArgument.Error(), "testing").(*werror.Error),
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "ErrPermissionDenied",
			err: &Error{
				Werror: gcode.FromCodeMessage(gcode.ErrPermissionDenied.Error(), "testing").(*werror.Error),
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "Unknown",
			err: &Error{
				Werror: gcode.FromCodeMessage("unknown", "testing").(*werror.Error),
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gcode.HTTPStatusCode(tt.err) != tt.expectedStatus {
				t.Fatal("err")
			}
		})
	}
}
