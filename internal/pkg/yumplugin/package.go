// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumplugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/yumdb"
	"go.ciq.dev/beskar/pkg/orasrpm"
)

func (p *Plugin) processPackages(ctx context.Context, manifests []*v1.Manifest) {
	repos := make(map[string]string)

	for idx, manifest := range manifests {
		if repository, dbDir, err := p.processPackage(ctx, manifest, idx == len(manifests)-1); err != nil {
			fmt.Printf("ERROR: %s\n", err)
		} else {
			repos[repository] = dbDir
		}
	}

	for repo, dbDir := range repos {
		err := p.GenerateAndSaveMetadata(ctx, filepath.Dir(repo), dbDir, true)
		if err != nil {
			fmt.Printf("ERROR: %s\n", err)
		}
	}
}

func (p *Plugin) processPackage(ctx context.Context, manifest *v1.Manifest, keepDatabaseDir bool) (string, string, error) {
	layerIndex := -1

	for i, layer := range manifest.Layers {
		if layer.MediaType != orasrpm.RPMPackageLayerType {
			continue
		}
		layerIndex = i
		break
	}

	if layerIndex < 0 {
		return "", "", fmt.Errorf("no RPM package layer found in manifest")
	}

	packageLayer := manifest.Layers[layerIndex]

	packageFilename := packageLayer.Annotations[imagespec.AnnotationTitle]

	repository := manifest.Annotations["repository"]
	ref := filepath.Join(p.registry, repository+"@sha256:"+packageLayer.Digest.Hex)
	repoDir := filepath.Join(p.dataDir, repository)

	tmpDir, err := os.MkdirTemp(p.dataDir, "package-")
	if err != nil {
		return "", "", fmt.Errorf("while creating temporary package directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	packageFile := filepath.Join(tmpDir, packageFilename)

	if err := downloadPackage(ref, packageFile, p); err != nil {
		return "", "", fmt.Errorf("while downloading package %s: %w", packageFilename, err)
	}

	href := fmt.Sprintf("Packages/%c/%s", strings.ToLower(packageFilename)[0], packageFilename)
	packageDir, err := extractPackageMetadata(tmpDir, repoDir, packageFilename, href)
	if err != nil {
		return "", "", fmt.Errorf("while extracting package %s metadata: %w", packageFilename, err)
	}

	dbDir, err := p.AddPackageToDatabase(ctx, manifest.Layers[0].Digest.Hex, repository, packageDir, true, keepDatabaseDir)
	if err != nil {
		return "", "", fmt.Errorf("while adding package %s to database: %w", packageFilename, err)
	}
	defer os.RemoveAll(packageDir)

	return repository, dbDir, err
}

func (p *Plugin) GenerateAndSaveMetadata(ctx context.Context, repository, dbDir string, execute bool) error {
	if execute {
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		args := []string{
			"gen-metadata",
			fmt.Sprintf("-config-dir=%s", p.beskarYumConfig.ConfigDirectory),
			fmt.Sprintf("-db-dir=%s", dbDir),
			fmt.Sprintf("-repository=%s", repository),
		}

		//nolint:gosec // internal use only
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("while generating metadata: %s", stderr.String())
		}

		return nil
	}

	defer func() {
		_ = os.RemoveAll(dbDir)
	}()

	repodataDir, err := os.MkdirTemp(p.dataDir, "repodata-")
	if err != nil {
		return fmt.Errorf("while creating temporary package directory: %w", err)
	}
	defer os.RemoveAll(repodataDir)

	outputDir := filepath.Join(repodataDir, "repodata")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		return err
	}

	db, err := yumdb.Open(dbDir)
	if err != nil {
		return err
	}

	packageCount, err := db.CountPackages(ctx)
	if err != nil {
		return err
	}

	repomd, err := newRepoMetadata(outputDir, p.registry, filepath.Join(repository, "repodata"), packageCount)
	if err != nil {
		return err
	}

	err = db.WalkPackages(ctx, func(pkg *yumdb.Package) error {
		if err := repomd.Add(bytes.NewReader(pkg.Primary), primaryXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", primaryXMLFile, err)
		}
		if err := repomd.Add(bytes.NewReader(pkg.Filelists), filelistsXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", filelistsXMLFile, err)
		}
		if err := repomd.Add(bytes.NewReader(pkg.Other), otherXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", otherXMLFile, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return repomd.Save(p)
}

func downloadPackage(ref string, destinationPath string, plugin *Plugin) (errFn error) {
	dst, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		err = dst.Close()
		if errFn == nil {
			errFn = err
		}
	}()

	digest, err := name.NewDigest(ref, plugin.nameOptions...)
	if err != nil {
		return err
	}
	layer, err := remote.Layer(digest, plugin.remoteOptions...)
	if err != nil {
		return err
	}
	rc, err := layer.Compressed()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(dst, rc)
	return err
}
