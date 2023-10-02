// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirror

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yummeta"
	"go.ciq.dev/beskar/pkg/decompress"
)

type Syncer struct {
	mirrorURLs  []*url.URL
	transport   *http.Transport
	downloadDir string
	httpProxy   *url.URL
	httpsProxy  *url.URL
	err         error
	repomdRoot  *yummeta.RepoMdRoot
}

type SyncerOption func(*Syncer)

func WithTLSConfig(tlsConfig *tls.Config) SyncerOption {
	return func(s *Syncer) {
		s.transport.TLSClientConfig = tlsConfig
	}
}

func WithProxyURL(proxy *url.URL, httpsProxy bool) SyncerOption {
	return func(s *Syncer) {
		if httpsProxy {
			s.httpsProxy = proxy
		} else {
			s.httpProxy = proxy
		}
	}
}

func NewSyncer(downloadDir string, mirrorURLs []*url.URL, options ...SyncerOption) *Syncer {
	syncer := &Syncer{
		mirrorURLs:  mirrorURLs,
		downloadDir: downloadDir,
		transport:   http.DefaultTransport.(*http.Transport).Clone(),
	}

	for _, opt := range options {
		opt(syncer)
	}

	if syncer.httpProxy != nil || syncer.httpsProxy != nil {
		syncer.transport.Proxy = syncer.getProxy
	}

	return syncer
}

func (s *Syncer) getProxy(req *http.Request) (*url.URL, error) {
	if req.URL.Scheme == "https" {
		return s.httpsProxy, nil
	}
	return s.httpProxy, nil
}

type PackageFilter func(id string) bool

func (s *Syncer) DownloadPackages(ctx context.Context, packageFilterFn PackageFilter) (<-chan string, int) {
	packageCh := make(chan string)
	totalPackagesCh := make(chan int)

	go func() {
		defer close(packageCh)
		defer close(totalPackagesCh)

		if err := s.initRepomdXML(ctx); err != nil {
			s.err = err
			return
		}

		dataHref := ""
		for _, data := range s.repomdRoot.Data {
			if data.Type != string(yummeta.PrimaryDataType) {
				continue
			}
			dataHref = data.Location.Href
			break
		}

		rc, err := s.FileReader(ctx, dataHref)
		if err != nil {
			s.err = err
			return
		}

		primaryPath := filepath.Join(s.downloadDir, filepath.Base(dataHref))
		primaryFile, err := os.Create(primaryPath)
		if err != nil {
			_ = rc.Close()
			s.err = err
			return
		}
		defer os.Remove(primaryPath)

		_, err = io.Copy(primaryFile, rc)
		_ = rc.Close()
		if err != nil {
			_ = primaryFile.Close()
			s.err = err
			return
		} else if err := primaryFile.Close(); err != nil {
			s.err = err
			return
		}

		primaryDecompressedFile, err := decompress.File(primaryPath)
		if err != nil {
			s.err = fmt.Errorf("while opening %s: %w", primaryPath, err)
			return
		}
		defer primaryDecompressedFile.Close()

		first := true

		err = yummeta.WalkPrimaryPackages(primaryDecompressedFile, func(pp yummeta.PrimaryPackage, totalPackages int) error {
			if first {
				totalPackagesCh <- totalPackages
				first = !first
			}
			if packageFilterFn(pp.ID) {
				packageCh <- pp.Href
			}
			return nil
		})
		if err != nil {
			s.err = err
		}
	}()

	return packageCh, <-totalPackagesCh
}

type ExtraMetadataFilter func(dataType string, checksum string) bool

func (s *Syncer) DownloadExtraMetadata(ctx context.Context, extraMetadataFilterFn ExtraMetadataFilter) <-chan int {
	metadataCh := make(chan int)

	go func() {
		defer close(metadataCh)

		if err := s.initRepomdXML(ctx); err != nil {
			s.err = err
			return
		}

		for idx, data := range s.repomdRoot.Data {
			switch yummeta.DataType(data.Type) {
			case yummeta.FilelistsDataType, yummeta.FilelistsDatabaseDataType:
			case yummeta.PrimaryDataType, yummeta.PrimaryDatabaseDataType:
			case yummeta.OtherDataType, yummeta.OtherDatabaseDataType:
			default:
				if extraMetadataFilterFn(data.Type, data.Checksum.Value) {
					metadataCh <- idx
				}
			}
		}
	}()

	return metadataCh
}

func (s *Syncer) RepomdData(index int) *yummeta.RepoMdData {
	if index > len(s.repomdRoot.Data)-1 {
		return nil
	}
	return s.repomdRoot.Data[index]
}

func (s *Syncer) initRepomdXML(ctx context.Context) error {
	if s.repomdRoot != nil {
		return nil
	}

	rc, err := s.FileReader(ctx, "repodata/repomd.xml")
	if err != nil {
		return err
	}
	defer rc.Close()

	s.repomdRoot, err = yummeta.ParseRepomd(rc)
	if err != nil {
		return err
	}

	return nil
}

func (s *Syncer) FileReader(ctx context.Context, path string) (io.ReadCloser, error) {
	client := &http.Client{Transport: s.transport}

	for idx, mirrorURL := range s.mirrorURLs {
		mirror := *mirrorURL
		mirror.Path = filepath.Join(mirror.Path, path)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, mirror.String(), nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			if len(s.mirrorURLs)-1 == idx {
				return nil, err
			}
		} else if resp.StatusCode != http.StatusOK {
			if len(s.mirrorURLs)-1 == idx {
				return nil, fmt.Errorf("unknown %d http status returned for mirror %s", resp.StatusCode, mirror.String())
			}
		} else {
			return resp.Body, nil
		}
	}

	return nil, fmt.Errorf("all upstream mirrors were tried")
}

func (s *Syncer) Err() error {
	return s.err
}
