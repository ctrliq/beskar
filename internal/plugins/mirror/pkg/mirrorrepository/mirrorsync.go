// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasmirror"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/mirror/api/v1"
	"go.ciq.dev/go-rsync/rsync"
)

type MirrorSyncerPlan struct {
	AddRemoteFiles   []*mirrordb.RepositoryFile
	DeleteLocalFiles []*mirrordb.RepositoryFile
}

type MirrorSyncer struct {
	h                  *Handler
	config             mirrorConfig
	configID           uint64
	parallelism        int
	upstreamRepository string
}

func NewMirrorSyncer(h *Handler, config mirrorConfig, configID uint64, parallelism int) (*MirrorSyncer, error) {
	repository := path.Base(config.URL.Path)

	return &MirrorSyncer{
		h:                  h,
		config:             config,
		configID:           configID,
		parallelism:        parallelism,
		upstreamRepository: path.Join("artifacts/mirror", repository),
	}, nil
}

func (s *MirrorSyncer) Plan() (*MirrorSyncerPlan, error) {
	// Fetch remote files
	remoteAPIFiles, err := s.ListRepositoryFiles()
	if err != nil {
		s.h.logger.Error("Failed to list remote files", "error", err)
		return nil, err
	}

	// Convert to db file structure
	remoteFiles := make([]*mirrordb.RepositoryFile, 0, len(remoteAPIFiles))
	for _, f := range remoteAPIFiles {
		remoteFiles = append(remoteFiles, toRepositoryFileDB(f))
	}

	// Fetch local files
	localFiles, err := s.h.listRepositoryFilesByConfigID(context.Background(), s.configID)
	if err != nil {
		s.h.logger.Error("Failed to list local files", "error", err)
		return nil, err
	}

	add, del := diff(localFiles, remoteFiles)

	return &MirrorSyncerPlan{
		AddRemoteFiles:   add,
		DeleteLocalFiles: del,
	}, nil
}

func diff(local, remote []*mirrordb.RepositoryFile) (add, del []*mirrordb.RepositoryFile) {
	mLocal := make(map[string]*mirrordb.RepositoryFile, len(local))
	for _, l := range local {
		mLocal[l.Name] = l
	}

	// Find items in remote that are not in local
	for _, r := range remote {
		if _, found := mLocal[r.Name]; !found {
			add = append(add, r)
		}
	}

	mRemote := make(map[string]*mirrordb.RepositoryFile, len(remote))
	for _, r := range remote {
		mRemote[r.Name] = r
	}

	// Find items in local that are not in remote
	for _, l := range local {
		if _, found := mRemote[l.Name]; !found {
			del = append(del, l)
		}
	}

	return add, del
}

func (s *MirrorSyncer) filePush(remoteFile *mirrordb.RepositoryFile) error {
	fileReference := filepath.Clean(s.h.generateFileReference(strings.ToLower(remoteFile.Name)))

	// Generate GET URL
	u := &url.URL{
		Scheme: s.config.URL.Scheme,
		Host:   s.config.URL.Host,
		User:   s.config.URL.User,
		Path:   path.Join(s.config.URL.Path, remoteFile.Name),
	}

	// Fetch file from remote
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		s.h.logger.Error("Failed to create request", "file", remoteFile.Name, "error", err)
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.h.logger.Error("Failed to fetch file", "file", remoteFile.Name, "error", err)
		return err
	}

	f, err := os.CreateTemp(s.h.downloadDir(), "")
	if err != nil {
		s.h.logger.Error("Failed to create temp file", "file", remoteFile.Name, "error", err)
		resp.Body.Close()
		return err
	}
	defer os.Remove(f.Name())

	s.h.logger.Debug("Downloading", "file", remoteFile.Name, "temp", f.Name())
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		s.h.logger.Error("Failed to download file", "file", remoteFile.Name, "error", err)
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	// Commit content to storage
	if err := f.Sync(); err != nil {
		return err
	}

	cb := backoff.WithMaxRetries(
		backoff.NewConstantBackOff(5*time.Second),
		3,
	)
	err = backoff.Retry(func() error {
		// Seek to start of file
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}

		// Push file to storage
		repoPath := filepath.Join(s.h.Repository, filepath.Dir(remoteFile.Name))
		s.h.logger.Debug("Pushing", "file", remoteFile.Name, "repo", repoPath)
		pusher, err := orasmirror.NewStaticFileStreamPusher(f, strings.ToLower(filepath.Base(remoteFile.Name)), strings.ToLower(repoPath), s.h.Params.NameOptions...)
		if err != nil {
			s.h.logger.Error("Failed to create pusher", "file", remoteFile.Name, "error", err)
			return err
		}

		err = oras.Push(pusher, s.h.Params.RemoteOptions...)
		if err != nil {
			s.h.logger.Error("Failed to push file", "file", remoteFile.Name, "error", err)
			return err
		}

		return nil
	}, cb)
	if err != nil {
		return err
	}

	// Add entry to DB
	//nolint:gosec
	sum := md5.Sum([]byte(fileReference))
	tag := hex.EncodeToString(sum[:])

	err = s.h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
		Tag:           tag,
		Name:          remoteFile.Name,
		Reference:     fileReference,
		Parent:        filepath.Dir(remoteFile.Name),
		Link:          "",
		LinkReference: "",
		ModifiedTime:  remoteFile.ModifiedTime,
		Mode:          remoteFile.Mode,
		Size:          remoteFile.Size,
		ConfigID:      s.configID,
	})
	if err != nil {
		s.h.logger.Error("Failed to add file to repository database", "file", remoteFile.Name, "error", err)
		return err
	}

	return nil
}

func (s *MirrorSyncer) fileWorker(c chan *mirrordb.RepositoryFile, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	for remoteFile := range c {
		if err := s.filePush(remoteFile); err != nil {
			s.h.logger.Error("Failed to push file", "file", remoteFile.Name, "error", err)
		}
	}
}

func (s *MirrorSyncer) Sync() error {
	// Generate plan
	plan, err := s.Plan()
	if err != nil {
		s.h.logger.Error("Failed to generate sync plan", "error", err)
		return err
	}

	// Create push channel and wait group
	pushChan := make(chan *mirrordb.RepositoryFile)
	wg := new(sync.WaitGroup)

	// Ensure download directory exists
	if err := os.MkdirAll(s.h.downloadDir(), 0o755); err != nil {
		return err
	}

	// Start worker pool
	for i := 0; i < s.parallelism; i++ {
		go s.fileWorker(pushChan, wg)
	}

	// Fetch/Update remote files
	for _, remoteFile := range plan.AddRemoteFiles {
		s.h.logger.Debug("Processing", "file", remoteFile.Name)

		fileReference := filepath.Clean(s.h.generateFileReference(strings.ToLower(remoteFile.Name)))

		if rsync.FileMode(remoteFile.Mode).IsREG() {
			// Process file in worker pool
			pushChan <- remoteFile
		} else if rsync.FileMode(remoteFile.Mode).IsDIR() {
			// Add entry to DB
			//nolint:gosec
			sum := md5.Sum([]byte(fileReference))
			tag := hex.EncodeToString(sum[:])

			err := s.h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
				Tag:           tag,
				Name:          remoteFile.Name,
				Reference:     fileReference,
				Parent:        filepath.Dir(remoteFile.Name),
				Link:          "",
				LinkReference: "",
				ModifiedTime:  remoteFile.ModifiedTime,
				Mode:          remoteFile.Mode,
				Size:          remoteFile.Size,
				ConfigID:      s.configID,
			})
			if err != nil {
				return err
			}
		} else if rsync.FileMode(remoteFile.Mode).IsLNK() {
			s.h.logger.Debug("Processing Link", "content", remoteFile.Link)

			link := s.h.generateFileReference(strings.ToLower(remoteFile.Link))

			// Add entry to DB
			//nolint:gosec
			sum := md5.Sum([]byte(remoteFile.Name))
			tag := hex.EncodeToString(sum[:])

			err := s.h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
				Tag:           tag,
				Name:          remoteFile.Name,
				Reference:     remoteFile.Name,
				Parent:        filepath.Dir(remoteFile.Name),
				Link:          remoteFile.Link,
				LinkReference: link,
				ModifiedTime:  remoteFile.ModifiedTime,
				Mode:          remoteFile.Mode,
				Size:          remoteFile.Size,
				ConfigID:      s.configID,
			})
			if err != nil {
				return err
			}
		}
	}

	close(pushChan)

	// Wait for all files to be processed
	wg.Wait()

	// Remove local files
	for _, localFile := range plan.DeleteLocalFiles {
		s.h.logger.Debug("Removing", "file", localFile.Name)

		fileReference := filepath.Clean(s.h.generateFileReference(strings.ToLower(localFile.Name)))

		// Remove entry from DB
		err := s.h.removeFileFromRepositoryDatabase(context.Background(), fileReference)
		if err != nil {
			s.h.logger.Error("Failed to remove file from repository database", "file", localFile.Name, "error", err)
			return err
		}
	}

	return nil
}

// Custom list method since generated client doesn't supply user credentials in the URL.
func (s *MirrorSyncer) ListRepositoryFiles() ([]*apiv1.RepositoryFile, error) {
	var u *url.URL

	if s.config.HTTPURL != nil {
		u = s.config.HTTPURL
	} else {
		path := "/repository/file:list"
		u = &url.URL{
			Scheme: s.config.URL.Scheme,
			Host:   s.config.URL.Host,
			User:   s.config.URL.User,
			Path:   apiv1.URLPath + path,
		}
	}

	reqBody := struct {
		Repository string `json:"repository"`
	}{
		Repository: s.upstreamRepository,
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody := struct {
		RepositoryFiles []*apiv1.RepositoryFile `json:"repository_files"`
	}{
		RepositoryFiles: make([]*apiv1.RepositoryFile, 0),
	}

	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return nil, err
	}

	return respBody.RepositoryFiles, nil
}
