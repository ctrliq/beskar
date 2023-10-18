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

func (c *HTTPClient) DeleteRepository(ctx context.Context, repository string) (err error) {
	codec := c.codecs.EncodeDecoder("DeleteRepository")

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

func (c *HTTPClient) GetRepositoryFileByName(ctx context.Context, repository string, name string) (repositoryFile *RepositoryFile, err error) {
	codec := c.codecs.EncodeDecoder("GetRepositoryFileByName")

	path := "/repository/file:byname"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		Name       string `json:"name"`
	}{
		Repository: repository,
		Name:       name,
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

	respBody := &GetRepositoryFileByNameResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.RepositoryFile, nil
}

func (c *HTTPClient) GetRepositoryFileByTag(ctx context.Context, repository string, tag string) (repositoryFile *RepositoryFile, err error) {
	codec := c.codecs.EncodeDecoder("GetRepositoryFileByTag")

	path := "/repository/file:bytag"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
	}{
		Repository: repository,
		Tag:        tag,
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

	respBody := &GetRepositoryFileByTagResponse{}
	err = codec.DecodeSuccessResponse(_resp.Body, respBody.Body())
	if err != nil {
		return nil, err
	}
	return respBody.RepositoryFile, nil
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

func (c *HTTPClient) RemoveRepositoryFile(ctx context.Context, repository string, tag string) (err error) {
	codec := c.codecs.EncodeDecoder("RemoveRepositoryFile")

	path := "/repository/file"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
	}{
		Repository: repository,
		Tag:        tag,
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
