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

func (c *HTTPClient) AddRemote(ctx context.Context, repository string, properties *OSTreeRemoteProperties) (err error) {
	codec := c.codecs.EncodeDecoder("AddRemote")

	path := "/repository/remote:add"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string                  `json:"repository"`
		Properties *OSTreeRemoteProperties `json:"properties"`
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

func (c *HTTPClient) CreateRepository(ctx context.Context, repository string, properties *OSTreeRepositoryProperties) (err error) {
	codec := c.codecs.EncodeDecoder("CreateRepository")

	path := "/repository"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string                      `json:"repository"`
		Properties *OSTreeRepositoryProperties `json:"properties"`
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

func (c *HTTPClient) SyncRepository(ctx context.Context, repository string, request *OSTreeRepositorySyncRequest) (err error) {
	codec := c.codecs.EncodeDecoder("SyncRepository")

	path := "/repository/sync"
	u := &url.URL{
		Scheme: c.scheme,
		Host:   c.host,
		Path:   c.pathPrefix + path,
	}

	reqBody := struct {
		Repository string                       `json:"repository"`
		Request    *OSTreeRepositorySyncRequest `json:"request"`
	}{
		Repository: repository,
		Request:    request,
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
