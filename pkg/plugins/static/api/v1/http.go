// Code generated by kun; DO NOT EDIT.
// github.com/RussellLuo/kun

package apiv1

import (
	"context"
	"net/http"

	"github.com/RussellLuo/kun/pkg/httpcodec"
	"github.com/RussellLuo/kun/pkg/httpoption"
	"github.com/RussellLuo/kun/pkg/oas2"
	"github.com/go-chi/chi"
	kithttp "github.com/go-kit/kit/transport/http"
)

func NewHTTPRouter(svc Static, codecs httpcodec.Codecs, opts ...httpoption.Option) chi.Router {
	r := chi.NewRouter()
	options := httpoption.NewOptions(opts...)

	r.Method("GET", "/doc/swagger.yaml", oas2.Handler(OASv2APIDoc, options.ResponseSchema()))

	var codec httpcodec.Codec
	var validator httpoption.Validator
	var kitOptions []kithttp.ServerOption

	codec = codecs.EncodeDecoder("DeleteRepository")
	validator = options.RequestValidator("DeleteRepository")
	r.Method(
		"DELETE", "/repository",
		kithttp.NewServer(
			MakeEndpointOfDeleteRepository(svc),
			decodeDeleteRepositoryRequest(codec, validator),
			httpcodec.MakeResponseEncoder(codec, 200),
			append(kitOptions,
				kithttp.ServerErrorEncoder(httpcodec.MakeErrorEncoder(codec)),
			)...,
		),
	)

	codec = codecs.EncodeDecoder("GetRepositoryFileByName")
	validator = options.RequestValidator("GetRepositoryFileByName")
	r.Method(
		"GET", "/repository/file:byname",
		kithttp.NewServer(
			MakeEndpointOfGetRepositoryFileByName(svc),
			decodeGetRepositoryFileByNameRequest(codec, validator),
			httpcodec.MakeResponseEncoder(codec, 200),
			append(kitOptions,
				kithttp.ServerErrorEncoder(httpcodec.MakeErrorEncoder(codec)),
			)...,
		),
	)

	codec = codecs.EncodeDecoder("GetRepositoryFileByTag")
	validator = options.RequestValidator("GetRepositoryFileByTag")
	r.Method(
		"GET", "/repository/file:bytag",
		kithttp.NewServer(
			MakeEndpointOfGetRepositoryFileByTag(svc),
			decodeGetRepositoryFileByTagRequest(codec, validator),
			httpcodec.MakeResponseEncoder(codec, 200),
			append(kitOptions,
				kithttp.ServerErrorEncoder(httpcodec.MakeErrorEncoder(codec)),
			)...,
		),
	)

	codec = codecs.EncodeDecoder("ListRepositoryFiles")
	validator = options.RequestValidator("ListRepositoryFiles")
	r.Method(
		"GET", "/repository/file:list",
		kithttp.NewServer(
			MakeEndpointOfListRepositoryFiles(svc),
			decodeListRepositoryFilesRequest(codec, validator),
			httpcodec.MakeResponseEncoder(codec, 200),
			append(kitOptions,
				kithttp.ServerErrorEncoder(httpcodec.MakeErrorEncoder(codec)),
			)...,
		),
	)

	codec = codecs.EncodeDecoder("ListRepositoryLogs")
	validator = options.RequestValidator("ListRepositoryLogs")
	r.Method(
		"GET", "/repository/logs",
		kithttp.NewServer(
			MakeEndpointOfListRepositoryLogs(svc),
			decodeListRepositoryLogsRequest(codec, validator),
			httpcodec.MakeResponseEncoder(codec, 200),
			append(kitOptions,
				kithttp.ServerErrorEncoder(httpcodec.MakeErrorEncoder(codec)),
			)...,
		),
	)

	codec = codecs.EncodeDecoder("RemoveRepositoryFile")
	validator = options.RequestValidator("RemoveRepositoryFile")
	r.Method(
		"DELETE", "/repository/file",
		kithttp.NewServer(
			MakeEndpointOfRemoveRepositoryFile(svc),
			decodeRemoveRepositoryFileRequest(codec, validator),
			httpcodec.MakeResponseEncoder(codec, 200),
			append(kitOptions,
				kithttp.ServerErrorEncoder(httpcodec.MakeErrorEncoder(codec)),
			)...,
		),
	)

	return r
}

func decodeDeleteRepositoryRequest(codec httpcodec.Codec, validator httpoption.Validator) kithttp.DecodeRequestFunc {
	return func(_ context.Context, r *http.Request) (interface{}, error) {
		var _req DeleteRepositoryRequest

		if err := codec.DecodeRequestBody(r, &_req); err != nil {
			return nil, err
		}

		if err := validator.Validate(&_req); err != nil {
			return nil, err
		}

		return &_req, nil
	}
}

func decodeGetRepositoryFileByNameRequest(codec httpcodec.Codec, validator httpoption.Validator) kithttp.DecodeRequestFunc {
	return func(_ context.Context, r *http.Request) (interface{}, error) {
		var _req GetRepositoryFileByNameRequest

		if err := codec.DecodeRequestBody(r, &_req); err != nil {
			return nil, err
		}

		if err := validator.Validate(&_req); err != nil {
			return nil, err
		}

		return &_req, nil
	}
}

func decodeGetRepositoryFileByTagRequest(codec httpcodec.Codec, validator httpoption.Validator) kithttp.DecodeRequestFunc {
	return func(_ context.Context, r *http.Request) (interface{}, error) {
		var _req GetRepositoryFileByTagRequest

		if err := codec.DecodeRequestBody(r, &_req); err != nil {
			return nil, err
		}

		if err := validator.Validate(&_req); err != nil {
			return nil, err
		}

		return &_req, nil
	}
}

func decodeListRepositoryFilesRequest(codec httpcodec.Codec, validator httpoption.Validator) kithttp.DecodeRequestFunc {
	return func(_ context.Context, r *http.Request) (interface{}, error) {
		var _req ListRepositoryFilesRequest

		if err := codec.DecodeRequestBody(r, &_req); err != nil {
			return nil, err
		}

		if err := validator.Validate(&_req); err != nil {
			return nil, err
		}

		return &_req, nil
	}
}

func decodeListRepositoryLogsRequest(codec httpcodec.Codec, validator httpoption.Validator) kithttp.DecodeRequestFunc {
	return func(_ context.Context, r *http.Request) (interface{}, error) {
		var _req ListRepositoryLogsRequest

		if err := codec.DecodeRequestBody(r, &_req); err != nil {
			return nil, err
		}

		if err := validator.Validate(&_req); err != nil {
			return nil, err
		}

		return &_req, nil
	}
}

func decodeRemoveRepositoryFileRequest(codec httpcodec.Codec, validator httpoption.Validator) kithttp.DecodeRequestFunc {
	return func(_ context.Context, r *http.Request) (interface{}, error) {
		var _req RemoveRepositoryFileRequest

		if err := codec.DecodeRequestBody(r, &_req); err != nil {
			return nil, err
		}

		if err := validator.Validate(&_req); err != nil {
			return nil, err
		}

		return &_req, nil
	}
}
