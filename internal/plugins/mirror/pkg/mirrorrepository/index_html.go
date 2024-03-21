// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/index"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasmirror"
	"go.ciq.dev/go-rsync/rsync"
)

const (
	defaultWebPrefix = "/artifacts/mirror/web/v1"
)

func (h *Handler) GenerateIndexes() error {
	webPrefix := path.Join(defaultWebPrefix, strings.TrimPrefix(h.Repository, "artifacts/mirror/"))
	if h.getWebConfig() != nil && h.getWebConfig().Prefix != "" {
		webPrefix = h.getWebConfig().Prefix
	}

	parents, err := h.listRepositoryDistinctParents(context.Background())
	if err != nil {
		h.logger.Error("Failed to list distinct parents", "error", err.Error())
		return err
	}

	sort.Strings(parents)

	for _, p := range parents {
		// Get parentInfo file info
		parentInfo, err := h.getRepositoryFile(context.Background(), p)
		if err != nil {
			h.logger.Error("Failed to get parent info", "error", err.Error(), "parent", p)
			return err
		}

		// Generate index config for parent
		c := index.Config{
			Current: path.Join(webPrefix, filepath.Clean(parentInfo.Name)),
		}

		// Don't set previous for root directory
		if filepath.Clean(parentInfo.Name) != filepath.Dir(parentInfo.Name) {
			c.Previous = path.Join(webPrefix, filepath.Dir(parentInfo.Name))
		}

		files, err := h.listRepositoryFilesByParent(context.Background(), p)
		if err != nil {
			h.logger.Error("Failed to list files by parent", "error", err.Error(), "parent", p)
			return err
		}

		// Sort files by name
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})

		// Process all files in the parent directory
		for _, fileInfo := range files {
			// Add files and directories to index config
			if rsync.FileMode(fileInfo.Mode).IsDIR() {
				if fileInfo.Name == "." {
					continue
				}

				c.Directories = append(c.Directories, index.Directory{
					Name:  filepath.Base(fileInfo.Name),
					Ref:   path.Join(webPrefix, filepath.Clean(fileInfo.Name)),
					MTime: time.Unix(fileInfo.ModifiedTime, 0),
				})
			} else if rsync.FileMode(fileInfo.Mode).IsLNK() {
				file, err := h.getRepositoryFile(context.Background(), fileInfo.Name)
				if err != nil {
					h.logger.Error("Failed to get file info", "error", err.Error(), "file", fileInfo.Name)
					return err
				}

				h.logger.Debug("Processing symlink", "file", fileInfo.Name, "link", file.Link)
				targetInfo, err := h.GetRepositoryFileByReferenceRaw(context.Background(), file.Link)
				if err != nil {
					h.logger.Error("Failed to get target info", "error", err.Error(), "link", file.Link)
					return err
				}

				if rsync.FileMode(targetInfo.Mode).IsDIR() {
					c.Directories = append(c.Directories, index.Directory{
						Name:  filepath.Base(file.Name),
						Ref:   path.Join(webPrefix, strings.TrimPrefix(file.Link, h.Repository)),
						MTime: time.Unix(file.ModifiedTime, 0),
					})
				} else {
					c.Files = append(c.Files, index.File{
						Name:  filepath.Base(fileInfo.Name),
						Ref:   path.Join(webPrefix, filepath.Clean(targetInfo.Name)),
						MTime: time.Unix(targetInfo.ModifiedTime, 0),
						Size:  targetInfo.Size,
					})
				}
			} else {
				c.Files = append(c.Files, index.File{
					Name:  filepath.Base(fileInfo.Name),
					Ref:   path.Join(webPrefix, filepath.Clean(fileInfo.Name)),
					MTime: time.Unix(fileInfo.ModifiedTime, 0),
					Size:  fileInfo.Size,
				})
			}
		}

		// Generate index.html file
		rawIndex, err := index.Generate(c)
		if err != nil {
			return err
		}

		pusher, err := orasmirror.NewStaticFileStreamPusher(bytes.NewReader(rawIndex), "index.html", parentInfo.Reference, h.Params.NameOptions...)
		if err != nil {
			return err
		}

		err = oras.Push(pusher, h.Params.RemoteOptions...)
		if err != nil {
			return err
		}

		//nolint:gosec
		s := md5.Sum([]byte(parentInfo.Reference))
		tag := hex.EncodeToString(s[:])

		err = h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
			Tag:          tag,
			Name:         parentInfo.Name,
			Reference:    parentInfo.Reference,
			Parent:       parentInfo.Parent,
			ModifiedTime: parentInfo.ModifiedTime,
			Mode:         parentInfo.Mode,
			Size:         parentInfo.Size,
			ConfigID:     parentInfo.ConfigID,
		})
		if err != nil {
			return err
		}

		h.logger.Debug("Generated Index", "Name", parentInfo.Name, "Reference", parentInfo.Reference)
	}

	return nil
}
