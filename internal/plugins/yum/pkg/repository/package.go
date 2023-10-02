// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cavaliergopher/rpm"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yummeta"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"golang.org/x/crypto/openpgp" //nolint:staticcheck
)

func (h *Handler) processPackageManifest(ctx context.Context, packageManifest *v1.Manifest) (errFn error) {
	packageLayer, err := getLayer(packageManifest, orasrpm.RPMPackageLayerType)
	if err != nil {
		return err
	}
	ref := filepath.Join(h.repository, "packages@sha256:"+packageLayer.Digest.Hex)

	packageName := packageLayer.Annotations[imagespec.AnnotationTitle]
	packagePath := filepath.Join(h.downloadDir(), packageName)

	defer func() {
		if errFn == nil {
			return
		}
		h.logger.Error("process package manifest", "package", packageName, "error", errFn.Error())
		h.logDatabase(ctx, yumdb.LogError, "process package %s manifest: %s", packageName, err)
		// TODO: remove package
	}()

	if err := downloadBlob(ref, packagePath, h.params); err != nil {
		return fmt.Errorf("while downloading package %s: %w", packageName, err)
	}
	defer os.Remove(packagePath)

	repositoryPackage, err := validatePackage(packageLayer.Digest.Hex, packagePath, h.getKeyring())
	if err != nil {
		return fmt.Errorf("while validating package %s: %w", packageName, err)
	}
	packageMetadata, err := extractPackageXMLMetadata(packageLayer.Digest.Hex, packagePath)
	if err != nil {
		return fmt.Errorf("while extracting package %s metadata: %w", packageName, err)
	}

	err = h.addPackageToMetadataDatabase(ctx, packageMetadata)
	if err != nil {
		return fmt.Errorf("while adding package %s to database: %w", packageName, err)
	}

	err = h.addPackageToRepositoryDatabase(ctx, repositoryPackage)
	if err != nil {
		return fmt.Errorf("while adding package %s to repository database: %w", packageName, err)
	}

	return nil
}

func (h *Handler) generateAndPushMetadata(ctx context.Context) (errFn error) {
	defer func() {
		if errFn == nil {
			return
		}
		h.logger.Error("metadata generation", "error", errFn.Error())
		h.logDatabase(ctx, yumdb.LogError, "metadata generation: %s", errFn)
	}()

	db, err := h.getMetadataDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close(true)

	packageCount, err := db.CountPackages(ctx)
	if err != nil {
		return err
	}

	repodataDir, err := os.MkdirTemp(h.repoDir, "repodata-")
	if err != nil {
		return fmt.Errorf("while creating temporary package directory: %w", err)
	}
	defer os.RemoveAll(repodataDir)

	outputDir := filepath.Join(repodataDir, "repodata")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		return err
	}

	repomd, err := newRepomd(outputDir, filepath.Join(h.repository, "repodata"), packageCount)
	if err != nil {
		return err
	}

	err = db.WalkPackageMetadata(ctx, func(pkg *yumdb.PackageMetadata) error {
		if err := repomd.add(bytes.NewReader(pkg.Primary), yummeta.PrimaryXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", yummeta.PrimaryXMLFile, err)
		}
		if err := repomd.add(bytes.NewReader(pkg.Filelists), yummeta.FilelistsXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", yummeta.FilelistsXMLFile, err)
		}
		if err := repomd.add(bytes.NewReader(pkg.Other), yummeta.OtherXMLFile); err != nil {
			return fmt.Errorf("while adding %s: %w", yummeta.OtherXMLFile, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	var extraMetadatas []*yumdb.ExtraMetadata

	err = db.WalkExtraMetadata(ctx, func(em *yumdb.ExtraMetadata) error {
		extraMetadatas = append(extraMetadatas, em)
		return nil
	})
	if err != nil {
		return err
	}

	return repomd.push(h, extraMetadatas)
}

func (h *Handler) deletePackageManifest(_ context.Context, _ *v1.Manifest) error {
	// TODO: implement this
	return fmt.Errorf("not supported yet")
}

func validatePackage(packageID, packagePath string, keyring openpgp.KeyRing) (*yumdb.RepositoryPackage, error) {
	r, err := os.Open(packagePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if err := rpm.MD5Check(r); err != nil {
		return nil, err
	} else if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if keyring != nil {
		if _, err := rpm.GPGCheck(r, keyring); err != nil {
			return nil, err
		} else if _, err := r.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
	}

	pkg, err := rpm.Read(r)
	if err != nil {
		return nil, err
	}

	return &yumdb.RepositoryPackage{
		ID:           packageID,
		Name:         pkg.Name(),
		UploadTime:   time.Now().UTC().Unix(),
		BuildTime:    pkg.BuildTime().Unix(),
		Size:         pkg.Size(),
		Architecture: pkg.Architecture(),
		SourceRPM:    pkg.SourceRPM(),
		Version:      pkg.Version(),
		Release:      pkg.Release(),
		Groups:       strings.Join(pkg.Groups(), ", "),
		License:      pkg.License(),
		Vendor:       pkg.Vendor(),
		Summary:      pkg.Summary(),
		Description:  pkg.Description(),
		Verified:     keyring != nil,
		GPGSignature: pkg.GPGSignature().String(),
	}, nil
}
