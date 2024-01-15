// Code generated by kun; DO NOT EDIT.
// github.com/RussellLuo/kun

package apiv1

import (
	"context"

	"github.com/RussellLuo/kun/pkg/httpoption"
	"github.com/RussellLuo/validating/v3"
	"github.com/go-kit/kit/endpoint"
)

type CreateRepositoryRequest struct {
	Repository string                `json:"repository"`
	Properties *RepositoryProperties `json:"properties"`
}

// ValidateCreateRepositoryRequest creates a validator for CreateRepositoryRequest.
func ValidateCreateRepositoryRequest(newSchema func(*CreateRepositoryRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*CreateRepositoryRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type CreateRepositoryResponse struct {
	Err error `json:"-"`
}

func (r *CreateRepositoryResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *CreateRepositoryResponse) Failed() error { return r.Err }

// MakeEndpointOfCreateRepository creates the endpoint for s.CreateRepository.
func MakeEndpointOfCreateRepository(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*CreateRepositoryRequest)
		err := s.CreateRepository(
			ctx,
			req.Repository,
			req.Properties,
		)
		return &CreateRepositoryResponse{
			Err: err,
		}, nil
	}
}

type DeleteRepositoryRequest struct {
	Repository  string `json:"repository"`
	DeleteFiles bool   `json:"delete_files"`
}

// ValidateDeleteRepositoryRequest creates a validator for DeleteRepositoryRequest.
func ValidateDeleteRepositoryRequest(newSchema func(*DeleteRepositoryRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*DeleteRepositoryRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type DeleteRepositoryResponse struct {
	Err error `json:"-"`
}

func (r *DeleteRepositoryResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *DeleteRepositoryResponse) Failed() error { return r.Err }

// MakeEndpointOfDeleteRepository creates the endpoint for s.DeleteRepository.
func MakeEndpointOfDeleteRepository(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*DeleteRepositoryRequest)
		err := s.DeleteRepository(
			ctx,
			req.Repository,
			req.DeleteFiles,
		)
		return &DeleteRepositoryResponse{
			Err: err,
		}, nil
	}
}

type GetRepositoryRequest struct {
	Repository string `json:"repository"`
}

// ValidateGetRepositoryRequest creates a validator for GetRepositoryRequest.
func ValidateGetRepositoryRequest(newSchema func(*GetRepositoryRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*GetRepositoryRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type GetRepositoryResponse struct {
	Properties *RepositoryProperties `json:"properties"`
	Err        error                 `json:"-"`
}

func (r *GetRepositoryResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *GetRepositoryResponse) Failed() error { return r.Err }

// MakeEndpointOfGetRepository creates the endpoint for s.GetRepository.
func MakeEndpointOfGetRepository(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*GetRepositoryRequest)
		properties, err := s.GetRepository(
			ctx,
			req.Repository,
		)
		return &GetRepositoryResponse{
			Properties: properties,
			Err:        err,
		}, nil
	}
}

type GetRepositoryFileRequest struct {
	Repository string `json:"repository"`
	File       string `json:"file"`
}

// ValidateGetRepositoryFileRequest creates a validator for GetRepositoryFileRequest.
func ValidateGetRepositoryFileRequest(newSchema func(*GetRepositoryFileRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*GetRepositoryFileRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type GetRepositoryFileResponse struct {
	RepositoryFile *RepositoryFile `json:"repository_file"`
	Err            error           `json:"-"`
}

func (r *GetRepositoryFileResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *GetRepositoryFileResponse) Failed() error { return r.Err }

// MakeEndpointOfGetRepositoryFile creates the endpoint for s.GetRepositoryFile.
func MakeEndpointOfGetRepositoryFile(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*GetRepositoryFileRequest)
		repositoryFile, err := s.GetRepositoryFile(
			ctx,
			req.Repository,
			req.File,
		)
		return &GetRepositoryFileResponse{
			RepositoryFile: repositoryFile,
			Err:            err,
		}, nil
	}
}

type GetRepositorySyncStatusRequest struct {
	Repository string `json:"repository"`
}

// ValidateGetRepositorySyncStatusRequest creates a validator for GetRepositorySyncStatusRequest.
func ValidateGetRepositorySyncStatusRequest(newSchema func(*GetRepositorySyncStatusRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*GetRepositorySyncStatusRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type GetRepositorySyncStatusResponse struct {
	SyncStatus *SyncStatus `json:"sync_status"`
	Err        error       `json:"-"`
}

func (r *GetRepositorySyncStatusResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *GetRepositorySyncStatusResponse) Failed() error { return r.Err }

// MakeEndpointOfGetRepositorySyncStatus creates the endpoint for s.GetRepositorySyncStatus.
func MakeEndpointOfGetRepositorySyncStatus(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*GetRepositorySyncStatusRequest)
		syncStatus, err := s.GetRepositorySyncStatus(
			ctx,
			req.Repository,
		)
		return &GetRepositorySyncStatusResponse{
			SyncStatus: syncStatus,
			Err:        err,
		}, nil
	}
}

type ListRepositoryFilesRequest struct {
	Repository string `json:"repository"`
	Page       *Page  `json:"page"`
}

// ValidateListRepositoryFilesRequest creates a validator for ListRepositoryFilesRequest.
func ValidateListRepositoryFilesRequest(newSchema func(*ListRepositoryFilesRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*ListRepositoryFilesRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type ListRepositoryFilesResponse struct {
	RepositoryFiles []*RepositoryFile `json:"repository_files"`
	Err             error             `json:"-"`
}

func (r *ListRepositoryFilesResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *ListRepositoryFilesResponse) Failed() error { return r.Err }

// MakeEndpointOfListRepositoryFiles creates the endpoint for s.ListRepositoryFiles.
func MakeEndpointOfListRepositoryFiles(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*ListRepositoryFilesRequest)
		repositoryFiles, err := s.ListRepositoryFiles(
			ctx,
			req.Repository,
			req.Page,
		)
		return &ListRepositoryFilesResponse{
			RepositoryFiles: repositoryFiles,
			Err:             err,
		}, nil
	}
}

type ListRepositoryLogsRequest struct {
	Repository string `json:"repository"`
	Page       *Page  `json:"page"`
}

// ValidateListRepositoryLogsRequest creates a validator for ListRepositoryLogsRequest.
func ValidateListRepositoryLogsRequest(newSchema func(*ListRepositoryLogsRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*ListRepositoryLogsRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type ListRepositoryLogsResponse struct {
	Logs []RepositoryLog `json:"logs"`
	Err  error           `json:"-"`
}

func (r *ListRepositoryLogsResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *ListRepositoryLogsResponse) Failed() error { return r.Err }

// MakeEndpointOfListRepositoryLogs creates the endpoint for s.ListRepositoryLogs.
func MakeEndpointOfListRepositoryLogs(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*ListRepositoryLogsRequest)
		logs, err := s.ListRepositoryLogs(
			ctx,
			req.Repository,
			req.Page,
		)
		return &ListRepositoryLogsResponse{
			Logs: logs,
			Err:  err,
		}, nil
	}
}

type SyncRepositoryRequest struct {
	Repository string `json:"repository"`
	Wait       bool   `json:"wait"`
}

// ValidateSyncRepositoryRequest creates a validator for SyncRepositoryRequest.
func ValidateSyncRepositoryRequest(newSchema func(*SyncRepositoryRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*SyncRepositoryRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type SyncRepositoryResponse struct {
	Err error `json:"-"`
}

func (r *SyncRepositoryResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *SyncRepositoryResponse) Failed() error { return r.Err }

// MakeEndpointOfSyncRepository creates the endpoint for s.SyncRepository.
func MakeEndpointOfSyncRepository(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*SyncRepositoryRequest)
		err := s.SyncRepository(
			ctx,
			req.Repository,
			req.Wait,
		)
		return &SyncRepositoryResponse{
			Err: err,
		}, nil
	}
}

type UpdateRepositoryRequest struct {
	Repository string                `json:"repository"`
	Properties *RepositoryProperties `json:"properties"`
}

// ValidateUpdateRepositoryRequest creates a validator for UpdateRepositoryRequest.
func ValidateUpdateRepositoryRequest(newSchema func(*UpdateRepositoryRequest) validating.Schema) httpoption.Validator {
	return httpoption.FuncValidator(func(value interface{}) error {
		req := value.(*UpdateRepositoryRequest)
		return httpoption.Validate(newSchema(req))
	})
}

type UpdateRepositoryResponse struct {
	Err error `json:"-"`
}

func (r *UpdateRepositoryResponse) Body() interface{} { return r }

// Failed implements endpoint.Failer.
func (r *UpdateRepositoryResponse) Failed() error { return r.Err }

// MakeEndpointOfUpdateRepository creates the endpoint for s.UpdateRepository.
func MakeEndpointOfUpdateRepository(s Mirror) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(*UpdateRepositoryRequest)
		err := s.UpdateRepository(
			ctx,
			req.Repository,
			req.Properties,
		)
		return &UpdateRepositoryResponse{
			Err: err,
		}, nil
	}
}
