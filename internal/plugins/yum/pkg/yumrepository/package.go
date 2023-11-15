// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumrepository

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "unsafe" // for go:linkname

	"github.com/cavaliergopher/rpm"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yummeta"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"golang.org/x/crypto/openpgp" //nolint:staticcheck
)

func (h *Handler) processPackageManifest(ctx context.Context, packageManifest *v1.Manifest, manifestDigest string) (errFn error) {
	defer func() {
		if errFn != nil {
			ref := filepath.Join(h.Repository, "packages@"+manifestDigest)
			if err := h.DeleteManifest(ref); err != nil {
				h.logger.Error("delete package manifest", "digest", manifestDigest, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "delete package manifest %s: %s", manifestDigest, err)
			}
		}
	}()

	packageLayer, err := oras.GetLayer(packageManifest, orasrpm.RPMPackageLayerType)
	if err != nil {
		return err
	}
	ref := filepath.Join(h.Repository, "packages@"+packageLayer.Digest.String())

	packageName := packageLayer.Annotations[imagespec.AnnotationTitle]
	packagePath := filepath.Join(h.downloadDir(), packageName)

	defer func() {
		h.syncArtifactsMutex.RLock()
		if errCh, ok := h.syncArtifacts[packageName]; ok {
			errCh <- errFn
		}
		h.syncArtifactsMutex.RUnlock()

		if errFn != nil {
			h.logger.Error("process package manifest", "package", packageName, "error", errFn.Error())
			h.logDatabase(ctx, yumdb.LogError, "process package %s manifest: %s", packageName, errFn)
		}
	}()

	if err := h.DownloadBlob(ref, packagePath); err != nil {
		return fmt.Errorf("while downloading package %s: %w", packageName, err)
	}
	defer os.Remove(packagePath)

	repositoryPackage, err := validatePackage(packageLayer.Digest.Hex, packagePath, h.getKeyring())
	if err != nil {
		return fmt.Errorf("while validating package %s: %w", packageName, err)
	}

	if !h.getMirror() {
		packageMetadata, err := extractPackageXMLMetadata(packageLayer.Digest.Hex, packagePath)
		if err != nil {
			return fmt.Errorf("while extracting package %s metadata: %w", packageName, err)
		}

		err = h.addPackageToMetadataDatabase(ctx, packageMetadata)
		if err != nil {
			return fmt.Errorf("while adding package %s to metadata database: %w", packageName, err)
		}
	}

	err = h.addPackageToRepositoryDatabase(ctx, repositoryPackage)
	if err != nil {
		return fmt.Errorf("while adding package %s to repository database: %w", packageName, err)
	}

	return nil
}

func (h *Handler) generateAndPushMetadata(ctx context.Context) (errFn error) {
	defer func() {
		if errFn != nil {
			h.logger.Error("metadata generation", "error", errFn.Error())
			h.logDatabase(ctx, yumdb.LogError, "metadata generation: %s", errFn)
		}
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

	repomd, err := newRepomd(outputDir, filepath.Join(h.Repository, "repodata"), packageCount)
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

	return repomd.push(h.Params, extraMetadatas)
}

func (h *Handler) deletePackageManifest(ctx context.Context, packageManifest *v1.Manifest) (errFn error) {
	packageLayer, err := oras.GetLayer(packageManifest, orasrpm.RPMPackageLayerType)
	if err != nil {
		return err
	}
	packageName := packageLayer.Annotations[imagespec.AnnotationTitle]
	packageID := packageLayer.Digest.Hex

	defer func() {
		h.syncArtifactsMutex.RLock()
		if errCh, ok := h.syncArtifacts[packageName]; ok {
			errCh <- errFn
		}
		h.syncArtifactsMutex.RUnlock()

		if errFn != nil {
			h.logger.Error("process package manifest removal", "package", packageName, "error", errFn.Error())
			h.logDatabase(ctx, yumdb.LogError, "process package %s manifest removal: %s", packageName, errFn)
		}
	}()

	if !h.getMirror() {
		err = h.removePackageFromMetadataDatabase(ctx, packageID)
		if err != nil {
			return fmt.Errorf("while removing package %s from metadata database: %w", packageName, err)
		}
	}

	err = h.removePackageFromRepositoryDatabase(ctx, packageID)
	if err != nil {
		return fmt.Errorf("while removing package %s from repository database: %w", packageName, err)
	}

	return nil
}

func validatePackage(packageID, packagePath string, keyring openpgp.KeyRing) (*yumdb.RepositoryPackage, error) {
	r, err := os.Open(packagePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if err := md5Check(r); err != nil {
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
	arch := pkg.Architecture()
	if pkg.SourceRPM() == "" {
		arch = "src"
	}

	return &yumdb.RepositoryPackage{
		ID:           packageID,
		Name:         pkg.Name(),
		UploadTime:   time.Now().UTC().Unix(),
		BuildTime:    pkg.BuildTime().Unix(),
		Size:         pkg.Size(),
		Architecture: arch,
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

//go:linkname readSigHeader github.com/cavaliergopher/rpm.readSigHeader
func readSigHeader(r io.Reader) (*rpm.Header, error)

func md5Check(r io.Reader) error {
	sigheader, err := readSigHeader(r)
	if err != nil {
		return err
	}
	payloadSize := sigheader.GetTag(270).Int64() // RPMSIGTAG_LONGSIGSIZE
	if payloadSize == 0 {
		payloadSize = sigheader.GetTag(1000).Int64() // RPMSIGTAG_SIGSIZE
		if payloadSize == 0 {
			return fmt.Errorf("tag not found: RPMSIGTAG_SIZE")
		}
	}
	expect := sigheader.GetTag(1004).Bytes() // RPMSIGTAG_MD5
	if expect == nil {
		return fmt.Errorf("tag not found: RPMSIGTAG_MD5")
	}
	//nolint:gosec
	h := md5.New()
	if n, err := io.Copy(h, r); err != nil {
		return err
	} else if n != payloadSize {
		return rpm.ErrMD5CheckFailed
	}
	actual := h.Sum(nil)
	if !bytes.Equal(expect, actual) {
		return rpm.ErrMD5CheckFailed
	}
	return nil
}
