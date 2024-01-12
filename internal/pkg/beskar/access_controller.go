// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package beskar

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/distribution/distribution/v3/registry/auth"
	"golang.org/x/crypto/bcrypt"
)

// accessController provides a simple implementation of auth.AccessController
// that simply checks for a non-empty Authorization header. It is useful for
// demonstration and testing.
type accessController struct {
	hashPassword   []byte
	hashedHostname string
}

var _ auth.AccessController = &accessController{}

func newAccessController(hashedHostname string) auth.InitFunc {
	return func(options map[string]interface{}) (auth.AccessController, error) {
		account, ok := options["account"]
		if !ok {
			return nil, fmt.Errorf("account with hashed password is missing: htpasswd bcrypt format expected")
		}
		htpasswdEntry, ok := account.(string)
		if !ok || htpasswdEntry == "" {
			return nil, fmt.Errorf("account with hashed password is missing or badly formatted: htpasswd bcrypt format expected")
		}

		idx := strings.Index(htpasswdEntry, ":")
		if idx == -1 || idx >= len(htpasswdEntry) {
			return nil, fmt.Errorf("account with hashed password is missing or badly formatted: htpasswd bcrypt format expected")
		}

		return &accessController{
			hashPassword:   []byte(htpasswdEntry[idx+1:]),
			hashedHostname: hashedHostname,
		}, nil
	}
}

// Authorized simply checks for the existence of the authorization header,
// responding with a bearer challenge if it doesn't exist.
func (ac *accessController) Authorized(req *http.Request, accessRecords ...auth.Access) (*auth.Grant, error) {
	requireAuthentication := false

	for _, record := range accessRecords {
		// enforce authentication for:
		// - catalog
		// - push/delete
		// - beskar internal repository (reserved for future use)
		if record.Type == "registry" {
			requireAuthentication = true
			break
		} else if record.Action == "push" || record.Action == "delete" {
			requireAuthentication = true
			break
		} else if record.Name == "beskar" || strings.HasPrefix(record.Name, "beskar/") {
			requireAuthentication = true
			break
		}
	}

	if !requireAuthentication {
		return &auth.Grant{}, nil
	} else if req.TLS != nil {
		isPlugin, ok := req.Context().Value(&serverPluginContextKey).(bool)
		if ok && isPlugin {
			return &auth.Grant{}, nil
		}
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		return nil, &challenge{
			err: auth.ErrInvalidCredential,
		}
	}

	if err := bcrypt.CompareHashAndPassword(ac.hashPassword, []byte(password)); err != nil {
		return nil, auth.ErrAuthenticationFailure
	}

	return &auth.Grant{User: auth.UserInfo{Name: username}}, nil
}

type challenge struct {
	err error
}

var _ auth.Challenge = challenge{}

// SetHeaders sets a simple bearer challenge on the response.
func (ch challenge) SetHeaders(_ *http.Request, w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Basic realm=beskar")
}

func (ch challenge) Error() string {
	return fmt.Sprintf("basic authentication challenge for realm beskar: %s", ch.err)
}
