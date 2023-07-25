// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

type AuthMethod struct {
	session *session.Session
}

type AuthMethodOption func(session *session.Session)

func WithCredentials(accessKeyID, secretKey, sessionToken string) AuthMethodOption {
	return func(session *session.Session) {
		session.Config.Credentials = newCredentials(accessKeyID, secretKey, sessionToken)
	}
}

func WithRegion(region string) AuthMethodOption {
	return func(session *session.Session) {
		session.Config.Region = aws.String(region)
	}
}

func WithDisableSSL(disable bool) AuthMethodOption {
	return func(session *session.Session) {
		session.Config.DisableSSL = aws.Bool(disable)
	}
}

func NewAuthMethod(endpoint string, options ...AuthMethodOption) (*AuthMethod, error) {
	sess, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	for _, opt := range options {
		opt(sess)
	}
	return &AuthMethod{
		session: sess,
	}, nil
}

func (am *AuthMethod) String() string {
	return "s3auth"
}

func (am *AuthMethod) Name() string {
	return am.String()
}

func (am *AuthMethod) Session() *session.Session {
	return am.session
}

func (am *AuthMethod) Endpoint() string {
	if am.session.Config.Endpoint == nil {
		return ""
	}
	return *am.session.Config.Endpoint
}

type basicProvider struct {
	value credentials.Value
}

func newCredentials(accessKeyID, secretKey, sessionToken string) *credentials.Credentials {
	return credentials.NewCredentials(&basicProvider{
		value: credentials.Value{
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretKey,
			SessionToken:    sessionToken,
			ProviderName:    "BasicProvider",
		},
	})
}

// Retrieve retrieves the keys from the environment.
func (bp *basicProvider) Retrieve() (credentials.Value, error) {
	return bp.value, nil
}

// IsExpired returns if the credentials have been retrieved.
func (basicProvider) IsExpired() bool {
	return false
}
