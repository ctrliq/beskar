// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasmirror"
	"go.ciq.dev/go-rsync/rsync"
)

type PlanSyncer struct {
	h           *Handler
	config      mirrorConfig
	configID    uint64
	parallelism int

	plan *rsync.SyncPlan

	client *http.Client
}

func NewPlanSyncer(h *Handler, config mirrorConfig, configID uint64, parallelism int, plan *rsync.SyncPlan) *PlanSyncer {
	return &PlanSyncer{
		h:           h,
		config:      config,
		configID:    configID,
		parallelism: parallelism,
		plan:        plan,
		client: &http.Client{
			Transport: &http.Transport{
				// Disable compression handling so compressed
				// files are preserved as they are when synchronized.
				DisableCompression: true,
			},
		},
	}
}

func (s *PlanSyncer) filePush(remoteFile rsync.FileInfo) error {
	fileReference := filepath.Clean(s.h.generateFileReference(strings.ToLower(string(remoteFile.Path))))

	// Fetch file from remote
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.config.HTTPURL.String()+"/"+string(remoteFile.Path), nil)
	if err != nil {
		s.h.logger.Error("Failed to create request", "file", string(remoteFile.Path), "error", err)
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.h.logger.Error("Failed to fetch file", "file", string(remoteFile.Path), "error", err)
		return err
	}

	f, err := os.CreateTemp(s.h.downloadDir(), "")
	if err != nil {
		s.h.logger.Error("Failed to create temp file", "file", string(remoteFile.Path), "error", err)
		resp.Body.Close()
		return err
	}
	defer os.Remove(f.Name())

	s.h.logger.Debug("Downloading", "file", string(remoteFile.Path), "temp", f.Name())
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		s.h.logger.Error("Failed to download file", "file", string(remoteFile.Path), "error", err)
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
		repoPath := filepath.Join(s.h.Repository, filepath.Dir(string(remoteFile.Path)))
		s.h.logger.Debug("Pushing", "file", string(remoteFile.Path), "repo", repoPath)
		pusher, err := orasmirror.NewStaticFileStreamPusher(f, strings.ToLower(filepath.Base(string(remoteFile.Path))), strings.ToLower(repoPath), s.h.Params.NameOptions...)
		if err != nil {
			s.h.logger.Error("Failed to create pusher", "file", string(remoteFile.Path), "error", err)
			return err
		}

		err = oras.Push(pusher, s.h.Params.RemoteOptions...)
		if err != nil {
			s.h.logger.Error("Failed to push file", "file", string(remoteFile.Path), "error", err)
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

	name := filepath.Clean(string(remoteFile.Path))

	err = s.h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
		Tag:           tag,
		Name:          name,
		Reference:     fileReference,
		Parent:        filepath.Dir(name),
		Link:          "",
		LinkReference: "",
		ModifiedTime:  int64(remoteFile.Mtime),
		Mode:          uint32(remoteFile.Mode),
		Size:          uint64(remoteFile.Size),
		ConfigID:      s.configID,
	})
	if err != nil {
		s.h.logger.Error("Failed to add file to repository database", "file", string(remoteFile.Path), "error", err)
		return err
	}

	return nil
}

func (s *PlanSyncer) fileWorker(c chan rsync.FileInfo, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	for remoteFile := range c {
		if err := s.filePush(remoteFile); err != nil {
			s.h.logger.Error("Failed to push file", "file", string(remoteFile.Path), "error", err)
		}
	}
}

func (s *PlanSyncer) Sync() error {
	// Create push channel and wait group
	pushChan := make(chan rsync.FileInfo)
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
	for _, i := range s.plan.AddRemoteFiles {
		// Get file from remote file list
		remoteFile := s.plan.RemoteFiles[i]

		s.h.logger.Debug("Processing", "file", string(remoteFile.Path))

		fileReference := filepath.Clean(s.h.generateFileReference(strings.ToLower(string(remoteFile.Path))))

		if remoteFile.Mode.IsREG() {
			// Process file in worker pool
			pushChan <- remoteFile
		} else if remoteFile.Mode.IsDIR() {
			// Add entry to DB
			//nolint:gosec
			sum := md5.Sum([]byte(fileReference))
			tag := hex.EncodeToString(sum[:])

			name := filepath.Clean(string(remoteFile.Path))

			err := s.h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
				Tag:           tag,
				Name:          name,
				Reference:     fileReference,
				Parent:        filepath.Dir(name),
				Link:          "",
				LinkReference: "",
				ModifiedTime:  int64(remoteFile.Mtime),
				Mode:          uint32(remoteFile.Mode),
				Size:          uint64(remoteFile.Size),
				ConfigID:      s.configID,
			})
			if err != nil {
				return err
			}
		} else if remoteFile.Mode.IsLNK() {
			// Lookup link content in map
			linkContent, ok := s.plan.Symlinks[string(remoteFile.Path)]
			if !ok {
				return fmt.Errorf("link content not found for %s", remoteFile.Path)
			}

			s.h.logger.Debug("Processing Link", "content", string(linkContent))

			intermediate := filepath.Clean(s.h.generateFileReference(strings.ToLower(string(linkContent))))
			target := strings.TrimPrefix(intermediate, s.h.Repository)
			linkReference := filepath.Join(filepath.Dir(fileReference), target)

			name := filepath.Clean(string(remoteFile.Path))

			// Add entry to DB
			//nolint:gosec
			sum := md5.Sum([]byte(name))
			tag := hex.EncodeToString(sum[:])

			err := s.h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
				Tag:           tag,
				Name:          name,
				Reference:     name,
				Parent:        filepath.Dir(name),
				Link:          string(linkContent),
				LinkReference: linkReference,
				ModifiedTime:  int64(remoteFile.Mtime),
				Mode:          uint32(remoteFile.Mode),
				Size:          uint64(remoteFile.Size),
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
	for _, i := range s.plan.DeleteLocalFiles {
		localFile := s.plan.LocalFiles[i]
		s.h.logger.Debug("Removing", "file", string(localFile.Path))

		fileReference := filepath.Clean(s.h.generateFileReference(strings.ToLower(string(localFile.Path))))

		// Remove entry from DB
		err := s.h.removeFileFromRepositoryDatabase(context.Background(), fileReference)
		if err != nil {
			s.h.logger.Error("Failed to remove file from repository database", "file", string(localFile.Path), "error", err)
			return err
		}
	}

	return nil
}
