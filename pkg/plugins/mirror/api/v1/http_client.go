// Code generated by kun; DO NOT EDIT.
// github.com/RussellLuo/kun

package apiv1

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/RussellLuo/kun/pkg/httpcodec"
)

type HTTPClient struct {
	codecs     httpcodec.Codecs
	httpClient *http.Client
	scheme     string
	host       string
	pathPrefix string
}

func NewHTTPClient(codecs httpcodec.Codecs, httpClient *http.Client, baseURL string) (*HTTPClient, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &HTTPClient{
		codecs:     codecs,
		httpClient: httpClient,
		scheme:     u.Scheme,
		host:       u.Host,
		pathPrefix: strings.TrimSuffix(u.Path, "/"),
	}, nil
}

func (c *HTTPClient) CreateRepository(ctx context.Context, repository string, properties *RepositoryProperties) (err error) {
	codec := c.codecs.EncodeDecoder("CreateRepository")

	path := "/repository"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string                `json:"repository"`
		Properties *RepositoryProperties `json:"properties"`
	}{
		Repository: repository,
		Properties: properties,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "POST", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}

func (c *HTTPClient) DeleteRepository(ctx context.Context, repository string, deleteFiles bool) (err error) {
	codec := c.codecs.EncodeDecoder("DeleteRepository")

	path := "/repository"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository  string `json:"repository"`
		DeleteFiles bool   `json:"delete_files"`
	}{
		Repository:  repository,
		DeleteFiles: deleteFiles,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "DELETE", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}

func (c *HTTPClient) DeleteRepositoryFile(ctx context.Context, repository string, file string) (err error) {
	codec := c.codecs.EncodeDecoder("DeleteRepositoryFile")

	path := "/repository/file"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		File       string `json:"file"`
	}{
		Repository: repository,
		File:       file,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "DELETE", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}

func (c *HTTPClient) DeleteRepositoryFilesByMode(ctx context.Context, repository string, mode uint32) (err error) {
	codec := c.codecs.EncodeDecoder("DeleteRepositoryFilesByMode")

	path := "/repository/file:mode"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		Mode       uint32 `json:"mode"`
	}{
		Repository: repository,
		Mode:       mode,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "DELETE", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}

func (c *HTTPClient) GenerateRepository(ctx context.Context, repository string) (err error) {
	codec := c.codecs.EncodeDecoder("GenerateRepository")

	path := "/repository/generate:web"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
	}{
		Repository: repository,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}

func (c *HTTPClient) GetRepository(ctx context.Context, repository string) (properties *RepositoryProperties, err error) {
	codec := c.codecs.EncodeDecoder("GetRepository")

	path := "/repository"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
	}{
		Repository: repository,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return nil, err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return nil, err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return nil, err
	}

	respBody := &GetRepositoryResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.Properties, nil
}

func (c *HTTPClient) GetRepositoryFile(ctx context.Context, repository string, file string) (repositoryFile *RepositoryFile, err error) {
	codec := c.codecs.EncodeDecoder("GetRepositoryFile")

	path := "/repository/file"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		File       string `json:"file"`
	}{
		Repository: repository,
		File:       file,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return nil, err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return nil, err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return nil, err
	}

	respBody := &GetRepositoryFileResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.RepositoryFile, nil
}

func (c *HTTPClient) GetRepositoryFileCount(ctx context.Context, repository string) (count int, err error) {
	codec := c.codecs.EncodeDecoder("GetRepositoryFileCount")

	path := "/repository/file:count"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
	}{
		Repository: repository,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return 0, err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return 0, err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return 0, err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return 0, err
	}

	respBody := &GetRepositoryFileCountResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return 0, err
	}
	return respBody.Count, nil
}

func (c *HTTPClient) GetRepositorySyncPlan(ctx context.Context, repository string) (syncPlan *RepositorySyncPlan, err error) {
	codec := c.codecs.EncodeDecoder("GetRepositorySyncPlan")

	path := "/repository/sync:plan"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
	}{
		Repository: repository,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return nil, err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return nil, err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return nil, err
	}

	respBody := &GetRepositorySyncPlanResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.SyncPlan, nil
}

func (c *HTTPClient) GetRepositorySyncStatus(ctx context.Context, repository string) (syncStatus *SyncStatus, err error) {
	codec := c.codecs.EncodeDecoder("GetRepositorySyncStatus")

	path := "/repository/sync:status"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
	}{
		Repository: repository,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return nil, err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return nil, err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return nil, err
	}

	respBody := &GetRepositorySyncStatusResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.SyncStatus, nil
}

func (c *HTTPClient) ListRepositoryFiles(ctx context.Context, repository string, page *Page) (repositoryFiles []*RepositoryFile, err error) {
	codec := c.codecs.EncodeDecoder("ListRepositoryFiles")

	path := "/repository/file:list"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		Page       *Page  `json:"page"`
	}{
		Repository: repository,
		Page:       page,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return nil, err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return nil, err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return nil, err
	}

	respBody := &ListRepositoryFilesResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.RepositoryFiles, nil
}

func (c *HTTPClient) ListRepositoryLogs(ctx context.Context, repository string, page *Page) (logs []RepositoryLog, err error) {
	codec := c.codecs.EncodeDecoder("ListRepositoryLogs")

	path := "/repository/logs"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		Page       *Page  `json:"page"`
	}{
		Repository: repository,
		Page:       page,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return nil, err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return nil, err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return nil, err
	}

	respBody := &ListRepositoryLogsResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.Logs, nil
}

func (c *HTTPClient) SyncRepository(ctx context.Context, repository string, wait bool) (err error) {
	codec := c.codecs.EncodeDecoder("SyncRepository")

	path := "/repository/sync"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		Wait       bool   `json:"wait"`
	}{
		Repository: repository,
		Wait:       wait,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}

func (c *HTTPClient) SyncRepositoryWithConfig(ctx context.Context, repository string, mirrorConfigs []MirrorConfig, webConfig *WebConfig, wait bool) (err error) {
	codec := c.codecs.EncodeDecoder("SyncRepositoryWithConfig")

	path := "/repository/sync:config"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository    string         `json:"repository"`
		MirrorConfigs []MirrorConfig `json:"mirror_configs"`
		WebConfig     *WebConfig     `json:"web_config"`
		Wait          bool           `json:"wait"`
	}{
		Repository:    repository,
		MirrorConfigs: mirrorConfigs,
		WebConfig:     webConfig,
		Wait:          wait,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "GET", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}

func (c *HTTPClient) UpdateRepository(ctx context.Context, repository string, properties *RepositoryProperties) (err error) {
	codec := c.codecs.EncodeDecoder("UpdateRepository")

	path := "/repository"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string                `json:"repository"`
		Properties *RepositoryProperties `json:"properties"`
	}{
		Repository: repository,
		Properties: properties,
	}
	reqBodyReader, headers, err := codec.EncodeRequestBody(&reqBody)
	if err != nil {
		return err
	}

	_req, err := http.NewRequestWithContext(ctx, "PUT", u.String(), reqBodyReader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		_req.Header.Set(k, v)
	}

	_resp, err := c.httpClient.Do(_req)
	if err != nil {
		return err
	}
	defer _resp.Body.Close()

	if _resp.StatusCode < http.StatusOK || _resp.StatusCode > http.StatusNoContent {
		var respErr error
		err := codec.DecodeFailureResponse(_resp.Body, &respErr)
		if err == nil {
			err = respErr
		}
		return err
	}

	return nil
}
