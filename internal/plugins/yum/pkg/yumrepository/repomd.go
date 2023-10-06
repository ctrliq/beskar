// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumrepository

import (
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/klauspost/compress/gzip"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/pkg/repository"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yummeta"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
)

const RepomdXMLTag = "repomdxml"

type writer struct {
	writeFn func([]byte) (int, error)
}

func (w writer) Write(p []byte) (int, error) {
	return w.writeFn(p)
}

type metaXML struct {
	io.Writer
	path         string
	openChecksum hash.Hash
	openSize     int
	checkSum     hash.Hash
	size         int
	close        func() error
}

func newMetaXML(path, header string) (*metaXML, error) {
	meta := &metaXML{
		path:         path,
		openChecksum: sha256.New(),
		checkSum:     sha256.New(),
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("while creating %s: %w", path, err)
	}
	gw := gzip.NewWriter(io.MultiWriter(f, meta.checkSum, meta.getWriter()))

	meta.close = func() error {
		if err := gw.Close(); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	}
	meta.Writer = io.MultiWriter(gw, meta.openChecksum, meta.getOpenWriter())

	_, err = meta.Write([]byte(header + "\n"))
	return meta, err
}

func (x *metaXML) getWriter() io.Writer {
	x.size = 0
	return &writer{
		writeFn: func(p []byte) (n int, err error) {
			x.size += len(p)
			return len(p), nil
		},
	}
}

func (x *metaXML) getOpenWriter() io.Writer {
	x.openSize = 0
	return &writer{
		writeFn: func(p []byte) (n int, err error) {
			x.openSize += len(p)
			return len(p), nil
		},
	}
}

func (x *metaXML) Path() string {
	return x.path
}

func (x *metaXML) Digest() (string, string) {
	return "sha256", fmt.Sprintf("%x", x.checkSum.Sum(nil))
}

func (x *metaXML) Size() int64 {
	return int64(x.size)
}

func (x *metaXML) add(rc io.Reader) error {
	_, err := io.Copy(x, rc)
	return err
}

func (x *metaXML) save(footer string) error {
	_, err := x.Write([]byte(footer + "\n"))
	closeErr := x.close()
	if err == nil {
		err = closeErr
	}
	return err
}

type primaryXML struct {
	*metaXML
}

func newPrimaryXML(path string, packageCount int) (*primaryXML, error) {
	header := fmt.Sprintf(yummeta.PrimaryHeaderFormat, packageCount)
	metaXML, err := newMetaXML(path, header)
	if err != nil {
		return nil, err
	}
	return &primaryXML{
		metaXML: metaXML,
	}, nil
}

func (x *primaryXML) Mediatype() string {
	return fmt.Sprintf(orasrpm.RepomdDataLayerTypeFormat, yummeta.PrimaryDataType)
}

func (x *primaryXML) Annotations() map[string]string {
	_, hex := x.Digest()
	return map[string]string{
		imagespec.AnnotationTitle: fmt.Sprintf("%s-%s", hex, yummeta.DataFilePrefix(yummeta.PrimaryDataType)),
	}
}

type filelistsXML struct {
	*metaXML
}

func newFilelistsXML(path string, packageCount int) (*filelistsXML, error) {
	header := fmt.Sprintf(yummeta.FilelistsHeaderFormat, packageCount)
	metaXML, err := newMetaXML(path, header)
	if err != nil {
		return nil, err
	}
	return &filelistsXML{
		metaXML: metaXML,
	}, nil
}

func (x *filelistsXML) Mediatype() string {
	return fmt.Sprintf(orasrpm.RepomdDataLayerTypeFormat, yummeta.FilelistsDataType)
}

func (x *filelistsXML) Annotations() map[string]string {
	_, hex := x.Digest()
	return map[string]string{
		imagespec.AnnotationTitle: fmt.Sprintf("%s-%s", hex, yummeta.DataFilePrefix(yummeta.FilelistsDataType)),
	}
}

type otherXML struct {
	*metaXML
}

func newOtherXML(path string, packageCount int) (*otherXML, error) {
	header := fmt.Sprintf(yummeta.OtherHeaderFormat, packageCount)
	metaXML, err := newMetaXML(path, header)
	if err != nil {
		return nil, err
	}
	return &otherXML{
		metaXML: metaXML,
	}, nil
}

func (x *otherXML) Mediatype() string {
	return fmt.Sprintf(orasrpm.RepomdDataLayerTypeFormat, yummeta.OtherDataType)
}

func (x *otherXML) Annotations() map[string]string {
	_, hex := x.Digest()
	return map[string]string{
		imagespec.AnnotationTitle: fmt.Sprintf("%s-%s", hex, yummeta.DataFilePrefix(yummeta.OtherDataType)),
	}
}

type repomd struct {
	repository    string
	repomdXMLPath string
	primaryXML    *primaryXML
	filelistsXML  *filelistsXML
	otherXML      *otherXML
}

func newRepomd(dir, repository string, packageCount int) (*repomd, error) {
	var err error

	rm := &repomd{
		repository:    repository,
		repomdXMLPath: filepath.Join(dir, yummeta.RepomdXMLFile),
	}

	rm.primaryXML, err = newPrimaryXML(filepath.Join(dir, yummeta.DataFilePrefix(yummeta.PrimaryDataType)), packageCount)
	if err != nil {
		return nil, err
	}

	rm.filelistsXML, err = newFilelistsXML(filepath.Join(dir, yummeta.DataFilePrefix(yummeta.FilelistsDataType)), packageCount)
	if err != nil {
		return nil, err
	}

	rm.otherXML, err = newOtherXML(filepath.Join(dir, yummeta.DataFilePrefix(yummeta.OtherDataType)), packageCount)
	if err != nil {
		return nil, err
	}

	return rm, nil
}

func (r *repomd) add(rc io.Reader, file string) error {
	switch file {
	case yummeta.PrimaryXMLFile:
		if err := r.primaryXML.add(rc); err != nil {
			return err
		}
	case yummeta.FilelistsXMLFile:
		if err := r.filelistsXML.add(rc); err != nil {
			return err
		}
	case yummeta.OtherXMLFile:
		if err := r.otherXML.add(rc); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown file %s", file)
	}

	return nil
}

func (r *repomd) save(repomdRoot *yummeta.RepoMdRoot) error {
	repomd, err := os.Create(r.repomdXMLPath)
	if err != nil {
		return err
	}

	repomdRoot.XmlnsRpm = repomdRoot.Rpm
	repomdRoot.Rpm = ""

	encoder := xml.NewEncoder(repomd)
	encoder.Indent("", "  ")

	if err := encoder.Encode(repomdRoot); err != nil {
		_ = repomd.Close()
		return err
	}

	return repomd.Close()
}

func (r *repomd) getRoot() (*yummeta.RepoMdRoot, error) {
	repomd, err := os.Open(r.repomdXMLPath)
	if err != nil {
		return nil, err
	}
	defer repomd.Close()

	repomdRoot := new(yummeta.RepoMdRoot)
	return repomdRoot, xml.NewDecoder(repomd).Decode(repomdRoot)
}

func (r *repomd) push(params *repository.HandlerParams, extraMetadatas []*yumdb.ExtraMetadata) error {
	pushRef, err := name.ParseReference(
		r.repository+":"+RepomdXMLTag,
		params.NameOptions...,
	)
	if err != nil {
		return err
	}

	if err := r.primaryXML.save(yummeta.PrimaryFooter); err != nil {
		return err
	}
	if err := r.filelistsXML.save(yummeta.FilelistsFooter); err != nil {
		return err
	}
	if err := r.otherXML.save(yummeta.OtherFooter); err != nil {
		return err
	}

	repomdRoot := new(yummeta.RepoMdRoot)
	repomdRoot.Xmlns = "http://linux.duke.edu/metadata/repo"
	repomdRoot.XmlnsRpm = "http://linux.duke.edu/metadata/rpm"

	now := time.Now().UTC().Unix()

	repomdRoot.Data = []*yummeta.RepoMdData{
		{
			Type: string(yummeta.PrimaryDataType),
			Checksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.primaryXML.checkSum.Sum(nil)),
			},
			Size: r.primaryXML.size,
			OpenChecksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.primaryXML.openChecksum.Sum(nil)),
			},
			OpenSize: r.primaryXML.openSize,
			Location: &yummeta.RepoMdDataLocation{
				Href: filepath.Join("repodata", yummeta.DataFilePrefix(yummeta.PrimaryDataType)),
			},
			Timestamp: now,
		},
		{
			Type: string(yummeta.FilelistsDataType),
			Checksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.filelistsXML.checkSum.Sum(nil)),
			},
			Size: r.filelistsXML.size,
			OpenChecksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.filelistsXML.openChecksum.Sum(nil)),
			},
			OpenSize: r.filelistsXML.openSize,
			Location: &yummeta.RepoMdDataLocation{
				Href: filepath.Join("repodata", yummeta.DataFilePrefix(yummeta.FilelistsDataType)),
			},
			Timestamp: now,
		},
		{
			Type: string(yummeta.OtherDataType),
			Checksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.otherXML.checkSum.Sum(nil)),
			},
			Size: r.otherXML.size,
			OpenChecksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.otherXML.openChecksum.Sum(nil)),
			},
			OpenSize: r.otherXML.openSize,
			Location: &yummeta.RepoMdDataLocation{
				Href: filepath.Join("repodata", yummeta.DataFilePrefix(yummeta.OtherDataType)),
			},
			Timestamp: now,
		},
	}

	if err := r.save(repomdRoot); err != nil {
		return err
	}

	repodataDir := filepath.Dir(r.repomdXMLPath)

	if err := generateSQLiteFiles(repodataDir); err != nil {
		return err
	}

	repomdRoot, err = r.getRoot()
	if err != nil {
		return err
	}

	metadataLayers := make([]oras.Layer, 0, 3)

	sqliteFiles := map[yummeta.DataType]string{
		yummeta.PrimaryDatabaseDataType:   yummeta.DataFilePrefix(yummeta.PrimaryDatabaseDataType),
		yummeta.FilelistsDatabaseDataType: yummeta.DataFilePrefix(yummeta.FilelistsDatabaseDataType),
		yummeta.OtherDatabaseDataType:     yummeta.DataFilePrefix(yummeta.OtherDatabaseDataType),
	}

	for dataType, sqliteFile := range sqliteFiles {
		mediatype := orasrpm.GetRepomdDataLayerType(string(dataType))
		path := filepath.Join(repodataDir, sqliteFile)
		annotations := make(map[string]string)
		layer, err := orasrpm.NewGenericRPMMetadata(path, mediatype, annotations)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(layer))

		for _, data := range repomdRoot.Data {
			if yummeta.DataType(data.Type) == dataType {
				_, layerDigest := layer.Digest()
				data.Location.Href = fmt.Sprintf("repodata/%s-%s", layerDigest, sqliteFile)
				annotations[imagespec.AnnotationTitle] = filepath.Base(data.Location.Href)
				break
			}
		}
	}

	for _, data := range repomdRoot.Data {
		switch yummeta.DataType(data.Type) {
		case yummeta.FilelistsDataType, yummeta.OtherDataType, yummeta.PrimaryDataType:
			data.Location.Href = fmt.Sprintf("repodata/%s-%s", data.Checksum.Value, yummeta.DataFilePrefix(yummeta.DataType(data.Type)))
		}
	}

	for _, extraMetadata := range extraMetadatas {
		path := filepath.Join(repodataDir, extraMetadata.Filename)
		if err := os.WriteFile(path, extraMetadata.Data, 0o600); err != nil {
			return err
		}

		annotations := make(map[string]string)
		layer, err := orasrpm.NewGenericRPMMetadata(path, orasrpm.GetRepomdDataLayerType(extraMetadata.Type), annotations)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(layer))

		repomdData := &yummeta.RepoMdData{
			Type: extraMetadata.Type,
			Checksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: extraMetadata.Checksum,
			},
			Size:      int(extraMetadata.Size),
			Timestamp: extraMetadata.Timestamp,
			Location: &yummeta.RepoMdDataLocation{
				Href: fmt.Sprintf("repodata/%s-%s", extraMetadata.Checksum, extraMetadata.Filename),
			},
		}

		if extraMetadata.OpenSize > 0 {
			repomdData.OpenChecksum = &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: extraMetadata.OpenChecksum,
			}
			repomdData.OpenSize = int(extraMetadata.OpenSize)
		}

		repomdRoot.Data = append(repomdRoot.Data, repomdData)

		annotations[imagespec.AnnotationTitle] = filepath.Base(repomdData.Location.Href)
	}

	if err := r.save(repomdRoot); err != nil {
		return err
	}

	repomdXML, err := orasrpm.NewGenericRPMMetadata(r.repomdXMLPath, orasrpm.RepomdXMLLayerType, nil)
	if err != nil {
		return err
	}

	metadataLayers = append(
		metadataLayers,
		orasrpm.NewRPMMetadataLayer(repomdXML),
		orasrpm.NewRPMMetadataLayer(r.primaryXML),
		orasrpm.NewRPMMetadataLayer(r.filelistsXML),
		orasrpm.NewRPMMetadataLayer(r.otherXML),
	)

	return oras.Push(
		orasrpm.NewRPMMetadataPusher(pushRef, orasrpm.RepomdConfigType, metadataLayers...),
		params.RemoteOptions...,
	)
}

func generateSQLiteFiles(dir string) error {
	//nolint:gosec // internal use only
	cmd := exec.Command(
		"sqliterepo_c",
		"--compress-type=gz",
		"--checksum=sha256",
		filepath.Dir(dir),
	)

	return cmd.Run()
}

func extractPackageXMLMetadata(packageID, packagePath string) (_ *yumdb.PackageMetadata, errFn error) {
	packageDir, packageName := filepath.Split(packagePath)

	tmpDir, err := os.MkdirTemp(packageDir, "repodata-")
	if err != nil {
		return nil, fmt.Errorf("while creating temporary package directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	//nolint:gosec
	cmd := exec.Command(
		"createrepo_c",
		"--no-database",
		"--checksum=sha256",
		"--simple-md-filenames",
		"--general-compress-type=gz",
		"--location-prefix="+fmt.Sprintf("Packages/%c", strings.ToLower(packageName)[0]),
		"-n", packageName,
		"-o", tmpDir,
		packageDir,
	)

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	repodataDir := filepath.Join(tmpDir, "repodata")

	packageMetadata := &yumdb.PackageMetadata{
		ID:   packageID,
		Name: packageName,
	}

	primaryXMLInput := filepath.Join(repodataDir, yummeta.DataFilePrefix(yummeta.PrimaryDataType))

	packageMetadata.Primary, err = extractMetadataTo(primaryXMLInput, &yummeta.PrimaryRoot{})
	if err != nil {
		return nil, err
	}

	filelistsXMLInput := filepath.Join(repodataDir, yummeta.DataFilePrefix(yummeta.FilelistsDataType))

	packageMetadata.Filelists, err = extractMetadataTo(filelistsXMLInput, &yummeta.FilelistsRoot{})
	if err != nil {
		return nil, err
	}

	otherXMLInput := filepath.Join(repodataDir, yummeta.DataFilePrefix(yummeta.OtherDataType))

	packageMetadata.Other, err = extractMetadataTo(otherXMLInput, &yummeta.OtherRoot{})
	if err != nil {
		return nil, err
	}

	return packageMetadata, nil
}

func extractMetadataTo(input string, root yummeta.XMLRoot) (_ []byte, errFn error) {
	in, err := os.Open(input)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	gr, err := gzip.NewReader(in)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	if err := xml.NewDecoder(gr).Decode(root); err != nil {
		return nil, err
	}

	return root.Data(), nil
}
