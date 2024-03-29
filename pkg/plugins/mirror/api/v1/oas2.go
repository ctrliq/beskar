// Code generated by kun; DO NOT EDIT.
// github.com/RussellLuo/kun

package apiv1

import (
	"reflect"

	"github.com/RussellLuo/kun/pkg/oas2"
)

var (
	base = `swagger: "2.0"
info:
  title: "Mirror Repository Management API"
  version: "1.0.0"
  description: "Mirror is used for managing mirror repositories.\nThis is the API documentation of Mirror.\n//"
  license:
    name: "MIT"
host: "example.com"
basePath: "/artifacts/mirror/api/v1"
schemes:
  - "https"
consumes:
  - "application/json"
produces:
  - "application/json"
`

	paths = `
paths:
  /repository:
    post:
      description: "Create a Mirror repository."
      operationId: "CreateRepository"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/CreateRepositoryRequestBody"
      %s
    delete:
      description: "Delete a Mirror repository."
      operationId: "DeleteRepository"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/DeleteRepositoryRequestBody"
      %s
    get:
      description: "Get Mirror repository properties."
      operationId: "GetRepository"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/GetRepositoryRequestBody"
      %s
    put:
      description: "Update Mirror repository properties."
      operationId: "UpdateRepository"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/UpdateRepositoryRequestBody"
      %s
  /repository/file:
    delete:
      description: "Delete file for a Mirror repository."
      operationId: "DeleteRepositoryFile"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/DeleteRepositoryFileRequestBody"
      %s
    get:
      description: "Get file for a Mirror repository."
      operationId: "GetRepositoryFile"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/GetRepositoryFileRequestBody"
      %s
  /repository/file:mode:
    delete:
      description: "Delete files by mode for a Mirror repository."
      operationId: "DeleteRepositoryFilesByMode"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/DeleteRepositoryFilesByModeRequestBody"
      %s
  /repository/generate:web:
    get:
      description: "Generate Mirror web pages ."
      operationId: "GenerateRepository"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/GenerateRepositoryRequestBody"
      %s
  /repository/file:count:
    get:
      description: "Get file count for a Mirror repository."
      operationId: "GetRepositoryFileCount"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/GetRepositoryFileCountRequestBody"
      %s
  /repository/sync:plan:
    get:
      description: "Get Mirror repository sync plan."
      operationId: "GetRepositorySyncPlan"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/GetRepositorySyncPlanRequestBody"
      %s
  /repository/sync:status:
    get:
      description: "Get Mirror repository sync status."
      operationId: "GetRepositorySyncStatus"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/GetRepositorySyncStatusRequestBody"
      %s
  /repository/file:list:
    get:
      description: "List files for a Mirror repository."
      operationId: "ListRepositoryFiles"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/ListRepositoryFilesRequestBody"
      %s
  /repository/logs:
    get:
      description: "List Mirror repository logs."
      operationId: "ListRepositoryLogs"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/ListRepositoryLogsRequestBody"
      %s
  /repository/sync:
    get:
      description: "Sync Mirror repository with an upstream repository."
      operationId: "SyncRepository"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/SyncRepositoryRequestBody"
      %s
  /repository/sync:config:
    get:
      description: "Sync Mirror repository with an upstream repository using a specified config."
      operationId: "SyncRepositoryWithConfig"
      tags:
        - mirror
      parameters:
        - name: body
          in: body
          schema:
            $ref: "#/definitions/SyncRepositoryWithConfigRequestBody"
      %s
`
)

func getResponses(schema oas2.Schema) []oas2.OASResponses {
	return []oas2.OASResponses{
		oas2.GetOASResponses(schema, "CreateRepository", 200, &CreateRepositoryResponse{}),
		oas2.GetOASResponses(schema, "DeleteRepository", 200, &DeleteRepositoryResponse{}),
		oas2.GetOASResponses(schema, "GetRepository", 200, &GetRepositoryResponse{}),
		oas2.GetOASResponses(schema, "UpdateRepository", 200, &UpdateRepositoryResponse{}),
		oas2.GetOASResponses(schema, "DeleteRepositoryFile", 200, &DeleteRepositoryFileResponse{}),
		oas2.GetOASResponses(schema, "GetRepositoryFile", 200, &GetRepositoryFileResponse{}),
		oas2.GetOASResponses(schema, "DeleteRepositoryFilesByMode", 200, &DeleteRepositoryFilesByModeResponse{}),
		oas2.GetOASResponses(schema, "GenerateRepository", 200, &GenerateRepositoryResponse{}),
		oas2.GetOASResponses(schema, "GetRepositoryFileCount", 200, &GetRepositoryFileCountResponse{}),
		oas2.GetOASResponses(schema, "GetRepositorySyncPlan", 200, &GetRepositorySyncPlanResponse{}),
		oas2.GetOASResponses(schema, "GetRepositorySyncStatus", 200, &GetRepositorySyncStatusResponse{}),
		oas2.GetOASResponses(schema, "ListRepositoryFiles", 200, &ListRepositoryFilesResponse{}),
		oas2.GetOASResponses(schema, "ListRepositoryLogs", 200, &ListRepositoryLogsResponse{}),
		oas2.GetOASResponses(schema, "SyncRepository", 200, &SyncRepositoryResponse{}),
		oas2.GetOASResponses(schema, "SyncRepositoryWithConfig", 200, &SyncRepositoryWithConfigResponse{}),
	}
}

func getDefinitions(schema oas2.Schema) map[string]oas2.Definition {
	defs := make(map[string]oas2.Definition)

	oas2.AddDefinition(defs, "CreateRepositoryRequestBody", reflect.ValueOf(&struct {
		Repository string                `json:"repository"`
		Properties *RepositoryProperties `json:"properties"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "CreateRepository", 200, (&CreateRepositoryResponse{}).Body())

	oas2.AddDefinition(defs, "DeleteRepositoryRequestBody", reflect.ValueOf(&struct {
		Repository  string `json:"repository"`
		DeleteFiles bool   `json:"delete_files"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "DeleteRepository", 200, (&DeleteRepositoryResponse{}).Body())

	oas2.AddDefinition(defs, "DeleteRepositoryFileRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
		File       string `json:"file"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "DeleteRepositoryFile", 200, (&DeleteRepositoryFileResponse{}).Body())

	oas2.AddDefinition(defs, "DeleteRepositoryFilesByModeRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
		Mode       uint32 `json:"mode"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "DeleteRepositoryFilesByMode", 200, (&DeleteRepositoryFilesByModeResponse{}).Body())

	oas2.AddDefinition(defs, "GenerateRepositoryRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "GenerateRepository", 200, (&GenerateRepositoryResponse{}).Body())

	oas2.AddDefinition(defs, "GetRepositoryRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "GetRepository", 200, (&GetRepositoryResponse{}).Body())

	oas2.AddDefinition(defs, "GetRepositoryFileRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
		File       string `json:"file"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "GetRepositoryFile", 200, (&GetRepositoryFileResponse{}).Body())

	oas2.AddDefinition(defs, "GetRepositoryFileCountRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "GetRepositoryFileCount", 200, (&GetRepositoryFileCountResponse{}).Body())

	oas2.AddDefinition(defs, "GetRepositorySyncPlanRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "GetRepositorySyncPlan", 200, (&GetRepositorySyncPlanResponse{}).Body())

	oas2.AddDefinition(defs, "GetRepositorySyncStatusRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "GetRepositorySyncStatus", 200, (&GetRepositorySyncStatusResponse{}).Body())

	oas2.AddDefinition(defs, "ListRepositoryFilesRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
		Page       *Page  `json:"page"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "ListRepositoryFiles", 200, (&ListRepositoryFilesResponse{}).Body())

	oas2.AddDefinition(defs, "ListRepositoryLogsRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
		Page       *Page  `json:"page"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "ListRepositoryLogs", 200, (&ListRepositoryLogsResponse{}).Body())

	oas2.AddDefinition(defs, "SyncRepositoryRequestBody", reflect.ValueOf(&struct {
		Repository string `json:"repository"`
		Wait       bool   `json:"wait"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "SyncRepository", 200, (&SyncRepositoryResponse{}).Body())

	oas2.AddDefinition(defs, "SyncRepositoryWithConfigRequestBody", reflect.ValueOf(&struct {
		Repository    string         `json:"repository"`
		MirrorConfigs []MirrorConfig `json:"mirror_configs"`
		WebConfig     *WebConfig     `json:"web_config"`
		Wait          bool           `json:"wait"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "SyncRepositoryWithConfig", 200, (&SyncRepositoryWithConfigResponse{}).Body())

	oas2.AddDefinition(defs, "UpdateRepositoryRequestBody", reflect.ValueOf(&struct {
		Repository string                `json:"repository"`
		Properties *RepositoryProperties `json:"properties"`
	}{}))
	oas2.AddResponseDefinitions(defs, schema, "UpdateRepository", 200, (&UpdateRepositoryResponse{}).Body())

	return defs
}

func OASv2APIDoc(schema oas2.Schema) string {
	resps := getResponses(schema)
	paths := oas2.GenPaths(resps, paths)

	defs := getDefinitions(schema)
	definitions := oas2.GenDefinitions(defs)

	return base + paths + definitions
}
