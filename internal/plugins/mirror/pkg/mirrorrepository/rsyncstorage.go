// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/antoniomika/go-rsync/rsync"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/index"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasmirror"
)

func (h *Handler) Put(filePath string, content io.Reader, fileSize int64, metadata rsync.FileMetadata) (written int64, err error) {
	repoPath := h.Repository
	fileName := filePath

	// Only handle regular file content
	if metadata.Mode.IsREG() {
		dir, file := filepath.Split(fileName)
		if dir != "" {
			repoPath = h.Repository + "/" + filepath.Clean(dir)
			fileName = file
		}

		pusher, err := orasmirror.NewStaticFileStreamPusher(content, strings.ToLower(fileName), strings.ToLower(repoPath), h.Params.NameOptions...)
		if err != nil {
			return 0, err
		}

		err = oras.Push(pusher, h.Params.RemoteOptions...)
		if err != nil {
			return 0, err
		}
	}

	fileReference := filepath.Clean(h.generateFileReference(strings.ToLower(filePath)))

	var link string
	if metadata.Mode.IsLNK() {
		c, err := io.ReadAll(content)
		if err != nil {
			return 0, err
		}

		intermediate := filepath.Clean(h.generateFileReference(strings.ToLower(string(c))))
		target := strings.TrimPrefix(intermediate, h.Repository)
		link = filepath.Join(filepath.Dir(fileReference), target)
	}

	//nolint:gosec
	s := md5.Sum([]byte(fileReference))
	tag := hex.EncodeToString(s[:])

	err = h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
		Tag:          tag,
		Name:         fileReference,
		Link:         link,
		ModifiedTime: int64(metadata.Mtime),
		Mode:         uint32(metadata.Mode),
		Size:         uint64(fileSize),
	})
	if err != nil {
		return 0, err
	}

	return fileSize, nil
}

func (h *Handler) Delete(filePath string, _ rsync.FileMode) error {
	fileReference := h.generateFileReference(filePath)

	err := h.removeFileFromRepositoryDatabase(context.Background(), fileReference)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) List() (rsync.FileList, error) {
	repositoryFiles, err := h.listRepositoryFiles(context.Background())
	if err != nil {
		return nil, err
	}

	var fileList rsync.FileList
	for _, repositoryFile := range repositoryFiles {
		path := filepath.Clean(strings.TrimPrefix(repositoryFile.Name, h.Repository))
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

func (h *Handler) GenerateIndexes(remoteList rsync.FileList) error {
	indexes := map[string][]rsync.FileInfo{}
	directoryMap := map[string]rsync.FileInfo{}
	orderedDirectories := []string{}

	for _, f := range remoteList {
		if f.Mode.IsDIR() {
			directoryMap[filepath.Clean(string(f.Path))] = f
		}

		indexes[filepath.Dir(string(f.Path))] = append(indexes[filepath.Dir(string(f.Path))], f)
	}

	for d := range indexes {
		orderedDirectories = append(orderedDirectories, d)
	}

	sort.Strings(orderedDirectories)

	for _, dir := range orderedDirectories {
		dirInfo := directoryMap[dir]
		fis := indexes[dir]

		c := index.Config{
			Current:  filepath.Clean(fmt.Sprintf("/artifacts/mirror/web/v1/%s/%s", strings.TrimPrefix(h.Repository, "artifacts/mirror/"), filepath.Clean(dir))),
			Previous: filepath.Clean(fmt.Sprintf("/artifacts/mirror/web/v1/%s/%s", strings.TrimPrefix(h.Repository, "artifacts/mirror/"), filepath.Dir(dir))),
		}
		for _, fi := range fis {
			if fi.Mode.IsDIR() {
				if string(fi.Path) == "." {
					continue
				}

				c.Directories = append(c.Directories, index.Directory{
					Name:  filepath.Base(string(fi.Path)),
					Ref:   fmt.Sprintf("/artifacts/mirror/web/v1/%s/%s", strings.TrimPrefix(h.Repository, "artifacts/mirror/"), filepath.Clean(string(fi.Path))),
					MTime: time.Unix(int64(fi.Mtime), 0),
				})
			} else if fi.Mode.IsLNK() {
				fileReference := filepath.Clean(h.generateFileReference(strings.ToLower(string(fi.Path))))
				file, err := h.getRepositoryFile(context.Background(), fileReference)
				if err != nil {
					return err
				}

				// NOTE: Assume all symlinks are to directories for now.
				c.Directories = append(c.Directories, index.Directory{
					Name:  filepath.Base(string(fi.Path)),
					Ref:   fmt.Sprintf("/artifacts/mirror/web/v1/%s", strings.TrimPrefix(filepath.Clean(file.Link), "artifacts/mirror/")),
					MTime: time.Unix(int64(fi.Mtime), 0),
				})
			} else {
				dir, file := filepath.Split(string(fi.Path))
				ref := fmt.Sprintf("/%s/%s/file/%s", h.Repository, filepath.Clean(dir), file)

				c.Files = append(c.Files, index.File{
					Name:  filepath.Base(string(fi.Path)),
					Ref:   ref,
					MTime: time.Unix(int64(fi.Mtime), 0),
					Size:  uint64(fi.Size),
				})
			}
		}

		rawIndex, err := index.Generate(c)
		if err != nil {
			return err
		}

		dir = filepath.Join(h.Repository, dir)

		pusher, err := orasmirror.NewStaticFileStreamPusher(bytes.NewReader(rawIndex), "index.html", strings.ToLower(dir), h.Params.NameOptions...)
		if err != nil {
			return err
		}

		err = oras.Push(pusher, h.Params.RemoteOptions...)
		if err != nil {
			return err
		}

		//nolint:gosec
		s := md5.Sum([]byte(dir))
		tag := hex.EncodeToString(s[:])

		err = h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
			Tag:          tag,
			Name:         dir,
			ModifiedTime: int64(dirInfo.Mtime),
			Mode:         uint32(dirInfo.Mode),
			Size:         uint64(dirInfo.Size),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
