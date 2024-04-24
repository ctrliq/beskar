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

	directories, err := h.listRepositoryDirectories(context.Background())
	if err != nil {
		h.logger.Error("Failed to list distinct parents", "error", err.Error())
		return err
	}

	// Sort parents to ensure consistent index generation
	sort.Slice(directories, func(i, j int) bool {
		return directories[i].Name < directories[j].Name
	})

	for _, directory := range directories {
		// Generate index config for parent
		c := index.Config{
			Current: path.Join(webPrefix, filepath.Clean(directory.Name)),
		}

		// Don't set previous for root directory
		if filepath.Clean(directory.Name) != filepath.Dir(directory.Name) {
			c.Previous = path.Join(webPrefix, filepath.Dir(directory.Name))
		}

		files, err := h.listRepositoryFilesByParent(context.Background(), directory.Name)
		if err != nil {
			h.logger.Error("Failed to list files by parent", "error", err.Error(), "directory", directory.Name)
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

				// Ensure link directory is preserved and only the link is replaced.
				targetName := file.Link
				if strings.Contains(file.Name, "/") {
					targetName = path.Join(path.Dir(file.Name), file.Link)
				}

				h.logger.Debug("Processing symlink", "file", fileInfo.Name, "link", file.Link, "target", targetName)
				targetInfo, err := h.GetRepositoryFileRaw(context.Background(), targetName)
				if err != nil {
					h.logger.Error("Failed to get target info", "error", err.Error(), "link", file.Link)
					return err
				}

				if rsync.FileMode(targetInfo.Mode).IsDIR() {
					c.Directories = append(c.Directories, index.Directory{
						Name:  filepath.Base(file.Name),
						Ref:   path.Join(webPrefix, targetName),
						MTime: time.Unix(file.ModifiedTime, 0),
					})
				} else {
					c.Files = append(c.Files, index.File{
						Name:  filepath.Base(fileInfo.Name),
						Ref:   path.Join(webPrefix, targetName),
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

		pusher, err := orasmirror.NewStaticFileStreamPusher(bytes.NewReader(rawIndex), "index.html", directory.Reference, h.Params.NameOptions...)
		if err != nil {
			return err
		}

		err = oras.Push(pusher, h.Params.RemoteOptions...)
		if err != nil {
			return err
		}

		//nolint:gosec
		s := md5.Sum([]byte(directory.Reference))
		tag := hex.EncodeToString(s[:])

		err = h.addFileToRepositoryDatabase(context.Background(), &mirrordb.RepositoryFile{
			Tag:          tag,
			Name:         directory.Name,
			Reference:    directory.Reference,
			Parent:       directory.Parent,
			ModifiedTime: directory.ModifiedTime,
			Mode:         directory.Mode,
			Size:         directory.Size,
			ConfigID:     directory.ConfigID,
		})
		if err != nil {
			return err
		}

		h.logger.Debug("Generated Index", "Name", directory.Name, "Reference", directory.Reference)
	}

	return nil
}
