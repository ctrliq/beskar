// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirror

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yummeta"
	"go.ciq.dev/beskar/pkg/decompress"

	"golang.org/x/crypto/openpgp"                  //nolint:staticcheck
	pgperrors "golang.org/x/crypto/openpgp/errors" //nolint:staticcheck
)

type Syncer struct {
	mirrorURLs  []*url.URL
	transport   *http.Transport
	downloadDir string
	httpProxy   *url.URL
	httpsProxy  *url.URL
	err         error
	repomdRoot  *yummeta.RepoMdRoot
	keyring     openpgp.KeyRing

	repomdXML          []byte
	repomdXMLSignature []byte
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

func WithKeyring(keyring openpgp.KeyRing) SyncerOption {
	return func(s *Syncer) {
		s.keyring = keyring
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

func (s *Syncer) DownloadMetadata(ctx context.Context, extraMetadataFilterFn ExtraMetadataFilter) <-chan int {
	metadataCh := make(chan int)

	go func() {
		defer close(metadataCh)

		if err := s.initRepomdXML(ctx); err != nil {
			s.err = err
			return
		}

		for idx, data := range s.repomdRoot.Data {
			if extraMetadataFilterFn(data.Type, data.Checksum.Value) {
				metadataCh <- idx
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

func (s *Syncer) RepomdXML() io.Reader {
	return bytes.NewReader(s.repomdXML)
}

func (s *Syncer) RepomdXMLSignature() io.Reader {
	if s.repomdXMLSignature == nil {
		return nil
	}
	return bytes.NewReader(s.repomdXMLSignature)
}

func (s *Syncer) initRepomdXML(ctx context.Context) error {
	if s.repomdRoot != nil {
		return nil
	}

	buf := new(bytes.Buffer)

	rc, err := s.FileReader(ctx, "repodata/repomd.xml")
	if err != nil {
		return err
	}

	_, err = io.Copy(buf, rc)
	_ = rc.Close()

	if err != nil {
		return err
	}

	s.repomdXML = make([]byte, buf.Len())
	copy(s.repomdXML, buf.Bytes())
	buf.Reset()

	if s.keyring != nil {
		rc, err = s.FileReader(ctx, "repodata/repomd.xml.asc")
		if err == nil {
			_, err = io.Copy(buf, rc)
			_ = rc.Close()

			if err != nil {
				return err
			}

			s.repomdXMLSignature = make([]byte, buf.Len())
			copy(s.repomdXMLSignature, buf.Bytes())

			signer, err := openpgp.CheckArmoredDetachedSignature(
				s.keyring,
				s.RepomdXML(),
				s.RepomdXMLSignature(),
			)

			if errors.Is(err, pgperrors.ErrUnknownIssuer) {
				return fmt.Errorf("repomd.xml.asc: GPG signature validation failed")
			} else if err != nil {
				return fmt.Errorf("repomd.xml.asc: %w", err)
			} else if len(signer.Identities) == 0 {
				return fmt.Errorf("repomd.xml.asc: no identity found in public key")
			}
		}
	}

	s.repomdRoot, err = yummeta.ParseRepomd(s.RepomdXML())
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
