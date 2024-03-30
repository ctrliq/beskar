// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasmirror"
	"go.ciq.dev/go-rsync/rsync"
)

type Storage struct {
	h        *Handler
	config   mirrorConfig
	configID uint64
	pushChan chan pushMessage
}

type pushMessage struct {
	filePath string
	repoPath string
}

func NewStorage(h *Handler, config mirrorConfig, configID uint64) *Storage {
	pushChan := make(chan pushMessage)

	for i := 0; i < h.Params.Sync.MaxWorkerCount; i++ {
		go func() {
			for msg := range pushChan {
				repoPath := h.Repository
				fileName := msg.filePath

				dir, file := filepath.Split(fileName)
				if dir != "" {
					repoPath = h.Repository + "/" + filepath.Clean(dir)
					fileName = file
				}

				f, err := os.Open(msg.filePath)
				if err != nil {
					h.logger.Error("Failed to open file", "error", err)
					continue
				}

				h.logger.Debug("Pushing", "file", fileName, "repo", repoPath)
				pusher, err := orasmirror.NewStaticFileStreamPusher(f, strings.ToLower(fileName), strings.ToLower(msg.repoPath), h.Params.NameOptions...)
				if err != nil {
					h.logger.Error("Failed to create pusher", "error", err)
					f.Close()
					continue
				}

				err = oras.Push(pusher, h.Params.RemoteOptions...)
				if err != nil {
					f.Close()
					h.logger.Error("Failed to push file", "error", err)
				}

				f.Close()
				os.Remove(msg.filePath)
			}
		}()
	}

	return &Storage{
		h:        h,
		config:   config,
		configID: configID,
		pushChan: pushChan,
	}
}

func (s *Storage) Put(filePath string, content io.Reader, fileSize int64, metadata rsync.FileMetadata) (written int64, err error) {
	repoPath := s.h.Repository
	fileName := filePath

	// Only handle regular file content
	if metadata.Mode.IsREG() {
		dir, _ := filepath.Split(fileName)
		if dir != "" {
			repoPath = filepath.Join(s.h.Repository, dir)
		}

		err := os.MkdirAll(filepath.Dir(filepath.Join(s.h.downloadDir(), filePath)), 0o755)
		if err != nil {
			return 0, err
		}

		err = copyTo(content, filepath.Join(s.h.downloadDir(), filePath))
		if err != nil {
			return 0, err
		}

		s.pushChan <- pushMessage{
			filePath: filepath.Join(s.h.downloadDir(), filePath),
			repoPath: repoPath,
		}
	}

	fileReference := filepath.Clean(s.h.generateFileReference(strings.ToLower(filePath)))

	name := filepath.Clean(filePath)

	var link, linkReference string
	if metadata.Mode.IsLNK() {
		c, err := io.ReadAll(content)
		if err != nil {
			return 0, err
		}

		link = string(c)

		intermediate := filepath.Clean(s.h.generateFileReference(strings.ToLower(string(c))))
		target := strings.TrimPrefix(intermediate, s.h.Repository)
		linkReference = filepath.Join(filepath.Dir(fileReference), target)

		// Set reference on links to something unique, but not used
		fileReference = name
	}

	//nolint:gosec
	sum := md5.Sum([]byte(fileReference))
	tag := hex.EncodeToString(sum[:])

	err = s.h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
		Tag:           tag,
		Name:          name,
		Reference:     fileReference,
		Parent:        filepath.Dir(name),
		Link:          link,
		LinkReference: linkReference,
		ModifiedTime:  int64(metadata.Mtime),
		Mode:          uint32(metadata.Mode),
		Size:          uint64(fileSize),
		ConfigID:      s.configID,
	})
	if err != nil {
		return 0, err
	}

	return fileSize, nil
}

func (s *Storage) Delete(filePath string, _ rsync.FileMode) error {
	fileReference := s.h.generateFileReference(filePath)

	err := s.h.removeFileFromRepositoryDatabase(context.Background(), fileReference)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) List() (rsync.FileList, error) {
	repositoryFiles, err := s.h.listRepositoryFilesByConfigID(context.Background(), s.configID)
	if err != nil {
		return nil, err
	}

	var fileList rsync.FileList
	for _, repositoryFile := range repositoryFiles {
		path := filepath.Clean(repositoryFile.Name)
		path = filepath.Clean(strings.TrimPrefix(path, "/"))

		fileList = append(fileList, rsync.FileInfo{
			Path:  []byte(path),
			Size:  int64(repositoryFile.Size),
			Mtime: int32(repositoryFile.ModifiedTime),
			Mode:  rsync.FileMode(repositoryFile.Mode),
		})
	}

	sort.Sort(fileList)

	return fileList, nil
}

func (s *Storage) Close() {
	close(s.pushChan)
}
