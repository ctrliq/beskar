// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package router

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/distribution/distribution/v3"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

var errCancelled = topdown.Error{Code: topdown.CancelErr}

type Result struct {
	Repository  string
	RedirectURL string
	Found       bool
}

type RegoOption = func(r *rego.Rego)

type RegoRouter struct {
	name    string
	options []RegoOption
	peq     rego.PreparedEvalQuery
}

type RegoRouterOption func(r *RegoRouter) error

func WithData(dataReader io.Reader) RegoRouterOption {
	return func(r *RegoRouter) error {
		var json map[string]interface{}

		data, err := io.ReadAll(dataReader)
		if err != nil {
			return err
		} else if err := util.UnmarshalJSON(data, &json); err != nil {
			return err
		}

		r.options = append(r.options, rego.Store(inmem.NewFromObject(json)))

		return nil
	}
}

func WithOption(option RegoOption) RegoRouterOption {
	return func(r *RegoRouter) error {
		r.options = append(r.options, option)
		return nil
	}
}

func New(name, module string, options ...RegoRouterOption) (_ *RegoRouter, err error) {
	router := &RegoRouter{
		name: name,
		options: []RegoOption{
			rego.Query("data.router.output"),
			rego.Module("router.rego", module),
			ociBlobDigestBuiltin,
			requestBodyBuiltin,
		},
	}

	for _, opt := range options {
		if err := opt(router); err != nil {
			return nil, fmt.Errorf("rego router option: %w", err)
		}
	}

	router.peq, err = rego.New(router.options...).PrepareForEval(context.Background())
	if err != nil {
		return nil, err
	}

	return router, nil
}

func (rr *RegoRouter) Decision(req *http.Request, registry distribution.Namespace) (*Result, error) {
	fctx := &funcContext{
		req:      req,
		registry: registry,
	}
	ctx := context.WithValue(req.Context(), &funcContextKey, fctx)

	rs, err := rr.peq.Eval(ctx, rego.EvalInput(map[string]string{
		"path":   req.URL.Path,
		"method": req.Method,
	}))
	if err != nil {
		if errors.Is(err, &errCancelled) && fctx.builtinErr != nil {
			return nil, fctx.builtinErr
		}
		return nil, err
	} else if len(rs) == 0 {
		return nil, fmt.Errorf("no output returned for %s routing decision", rr.name)
	} else if len(rs[0].Expressions) == 0 {
		return nil, fmt.Errorf("no output expression returned for %s routing decision", rr.name)
	}

	output := rs[0].Expressions[0].Value.(map[string]interface{})
	result := &Result{}

	if v, ok := output["repository"].(string); ok {
		result.Repository = v
	}
	if v, ok := output["redirect_url"].(string); ok {
		result.RedirectURL = v
	}
	if v, ok := output["found"].(bool); ok {
		result.Found = v
	}

	return result, nil
}
