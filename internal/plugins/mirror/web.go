// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirror

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasmirror"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/mirror/api/v1"
	"go.ciq.dev/go-rsync/rsync"
)

func hasPathPrefix(path, prefix string) bool {
	pathSplit := strings.Split(path, "/")
	prefixSplit := strings.Split(prefix, "/")

	for i, p := range prefixSplit {
		if p != pathSplit[i] {
			return false
		}
	}

	return true
}

func (p *Plugin) resolveSymlinks(repository, fileName string) (*apiv1.RepositoryFile, error) {
	symlinks, err := p.repositoryManager.Get(p.ctx, repository).ListRepositorySymlinks(p.ctx, nil)
	if err != nil {
		return nil, err
	}

	const (
		maxLinks = 50
	)

	intermediate := fileName
	for i := 0; i < maxLinks; i++ {
		// Check if file has a symlink in its path and replace it
		var replacement string
		for _, symlink := range symlinks {
			if hasPathPrefix(intermediate, symlink.Name) {
				// Ensure link directory is preserved and only the link is replaced.
				replacementPrefix := symlink.Link
				if strings.Contains(symlink.Name, "/") {
					replacementPrefix = path.Join(path.Dir(symlink.Name), symlink.Link)
				}

				replacement = strings.Replace(intermediate, symlink.Name, replacementPrefix, 1)
				break
			}
		}
		if replacement == "" {
			return nil, fmt.Errorf("not found")
		}

		repositoryFile, err := p.repositoryManager.Get(p.ctx, repository).GetRepositoryFile(p.ctx, replacement)
		if err != nil && !strings.Contains(err.Error(), "no entry found") {
			return nil, err
		} else if err == nil {
			return repositoryFile, nil
		}

		intermediate = replacement
	}

	return nil, fmt.Errorf("not found, too many symlinks")
}

func (p *Plugin) WebHandler(w http.ResponseWriter, r *http.Request) {
	subPath := strings.TrimPrefix(r.URL.Path, "/artifacts/mirror/web/v1/")

	repositoryName := strings.SplitN(subPath, "/", 2)[0]

	fileName := "."
	if len(strings.SplitN(subPath, "/", 2)) > 1 {
		fileName = filepath.Clean(strings.SplitN(subPath, "/", 2)[1])
	}

	repository := fmt.Sprintf("artifacts/mirror/%s", repositoryName)

	if err := checkRepository(repository); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	repositoryFile, err := p.repositoryManager.Get(p.ctx, repository).GetRepositoryFile(p.ctx, fileName)
	if err != nil {
		if strings.Contains(err.Error(), "no entry found") {
			// Attempt to resolve symlinks
			repositoryFile, err = p.resolveSymlinks(repository, fileName)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Redirect to file api to fetch blob signed url
	if rsync.FileMode(repositoryFile.Mode).IsREG() {
		repo, file := filepath.Split(repositoryFile.Reference)
		http.Redirect(w, r, fmt.Sprintf("/%s/file/%s", repo, file), http.StatusMovedPermanently)
		return
	} else if rsync.FileMode(repositoryFile.Mode).IsLNK() {
		file, err := p.repositoryManager.Get(p.ctx, repository).GetRepositoryFile(p.ctx, repositoryFile.Link)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !rsync.FileMode(file.Mode).IsDIR() {
			repo, file := filepath.Split(file.Reference)
			http.Redirect(w, r, fmt.Sprintf("/%s/file/%s", repo, file), http.StatusMovedPermanently)
		}

		repositoryFile.Reference = repositoryFile.Link
	}

	// Fetch index.html for directory listing
	ref, err := orasmirror.FileReference("index.html", strings.ToLower(repositoryFile.Reference), p.handlerParams.NameOptions...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	puller := orasmirror.NewMirrorPuller(ref, w)
	err = oras.Pull(puller, p.handlerParams.RemoteOptions...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/html")
}
