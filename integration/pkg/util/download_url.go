package util

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

func DownloadFromURL(url string, timeout time.Duration) (io.ReadCloser, error) {
	httpClient := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}
