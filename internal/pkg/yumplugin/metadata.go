// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumplugin

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
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/klauspost/compress/gzip"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/orasrpm"
	"go.ciq.dev/beskar/internal/pkg/yumplugin/pkg/yummeta"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	repomdXMLFile = "repomd.xml"

	primaryXMLFile     = "primary.xml"
	primaryXMLGzipFile = "primary.xml.gz"
	primarySQLiteFile  = "primary.sqlite.gz"

	filelistsXMLFile     = "filelists.xml"
	filelistsXMLGzipFile = "filelists.xml.gz"
	filelistsSQLiteFile  = "filelists.sqlite.gz"

	otherXMLFile     = "other.xml"
	otherXMLGzipFile = "other.xml.gz"
	otherSQLiteFile  = "other.sqlite.gz"

	primaryHeaderFormat = "<metadata xmlns=\"http://linux.duke.edu/metadata/common\" rpm=\"http://linux.duke.edu/metadata/rpm\" packages=\"%d\">"
	primaryFooter       = "</metadata>"

	filelistsHeaderFormat = "<filelists xmlns=\"http://linux.duke.edu/metadata/filelists\" packages=\"%d\">"
	filelistsFooter       = "</filelists>"

	otherHeaderFormat = "<otherdata xmlns=\"http://linux.duke.edu/metadata/other\" packages=\"%d\">"
	otherFooter       = "</otherdata>"
)

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
	header := fmt.Sprintf(primaryHeaderFormat, packageCount)
	metaXML, err := newMetaXML(path, header)
	if err != nil {
		return nil, err
	}
	return &primaryXML{
		metaXML: metaXML,
	}, nil
}

func (x *primaryXML) Mediatype() string {
	return orasrpm.PrimaryXMLLayerType
}

func (x *primaryXML) Annotations() map[string]string {
	return nil
}

type filelistsXML struct {
	*metaXML
}

func newFilelistsXML(path string, packageCount int) (*filelistsXML, error) {
	header := fmt.Sprintf(filelistsHeaderFormat, packageCount)
	metaXML, err := newMetaXML(path, header)
	if err != nil {
		return nil, err
	}
	return &filelistsXML{
		metaXML: metaXML,
	}, nil
}

func (x *filelistsXML) Mediatype() string {
	return orasrpm.FilelistsXMLLayerType
}

func (x *filelistsXML) Annotations() map[string]string {
	return nil
}

type otherXML struct {
	*metaXML
}

func newOtherXML(path string, packageCount int) (*otherXML, error) {
	header := fmt.Sprintf(otherHeaderFormat, packageCount)
	metaXML, err := newMetaXML(path, header)
	if err != nil {
		return nil, err
	}
	return &otherXML{
		metaXML: metaXML,
	}, nil
}

func (x *otherXML) Mediatype() string {
	return orasrpm.OtherXMLLayerType
}

func (x *otherXML) Annotations() map[string]string {
	return nil
}

type repoMetadata struct {
	repository    string
	registry      string
	repomdXMLPath string
	primaryXML    *primaryXML
	filelistsXML  *filelistsXML
	otherXML      *otherXML
}

func newRepoMetadata(dir, registry, repository string, packageCount int) (*repoMetadata, error) {
	var err error

	rm := &repoMetadata{
		repository:    repository,
		registry:      registry,
		repomdXMLPath: filepath.Join(dir, repomdXMLFile),
	}

	rm.primaryXML, err = newPrimaryXML(filepath.Join(dir, primaryXMLGzipFile), packageCount)
	if err != nil {
		return nil, err
	}

	rm.filelistsXML, err = newFilelistsXML(filepath.Join(dir, filelistsXMLGzipFile), packageCount)
	if err != nil {
		return nil, err
	}

	rm.otherXML, err = newOtherXML(filepath.Join(dir, otherXMLGzipFile), packageCount)
	if err != nil {
		return nil, err
	}

	return rm, nil
}

func (r *repoMetadata) Add(rc io.Reader, file string) error {
	switch file {
	case primaryXMLFile:
		if err := r.primaryXML.add(rc); err != nil {
			return err
		}
	case filelistsXMLFile:
		if err := r.filelistsXML.add(rc); err != nil {
			return err
		}
	case otherXMLFile:
		if err := r.otherXML.add(rc); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown file %s", file)
	}

	return nil
}

func (r *repoMetadata) saveRepomd(repomdRoot *yummeta.RepoMdRoot) error {
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

func (r *repoMetadata) getRepomd() (*yummeta.RepoMdRoot, error) {
	repomd, err := os.Open(r.repomdXMLPath)
	if err != nil {
		return nil, err
	}
	defer repomd.Close()

	repomdRoot := new(yummeta.RepoMdRoot)
	return repomdRoot, xml.NewDecoder(repomd).Decode(repomdRoot)
}

func (r *repoMetadata) Save(plugin *Plugin) error {
	pushRef, err := name.ParseReference(
		filepath.Join(r.registry, r.repository+":latest"),
		plugin.nameOptions...,
	)
	if err != nil {
		return err
	}

	if err := r.primaryXML.save(primaryFooter); err != nil {
		return err
	}
	if err := r.filelistsXML.save(filelistsFooter); err != nil {
		return err
	}
	if err := r.otherXML.save(otherFooter); err != nil {
		return err
	}

	repomdRoot := new(yummeta.RepoMdRoot)
	repomdRoot.Xmlns = "http://linux.duke.edu/metadata/repo"
	repomdRoot.XmlnsRpm = "http://linux.duke.edu/metadata/rpm"

	now := time.Now().UTC().Unix()

	primaryChecksum := fmt.Sprintf("%x", r.primaryXML.checkSum.Sum(nil))
	otherChecksum := fmt.Sprintf("%x", r.otherXML.checkSum.Sum(nil))
	filelistsChecksum := fmt.Sprintf("%x", r.filelistsXML.checkSum.Sum(nil))

	repomdRoot.Data = []*yummeta.RepoMdData{
		{
			Type: "primary",
			Checksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: primaryChecksum,
			},
			Size: r.primaryXML.size,
			OpenChecksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.primaryXML.openChecksum.Sum(nil)),
			},
			OpenSize: r.primaryXML.openSize,
			Location: &yummeta.RepoMdDataLocation{
				Href: filepath.Join("repodata", primaryXMLGzipFile),
			},
			Timestamp: now,
		},
		{
			Type: "filelists",
			Checksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: filelistsChecksum,
			},
			Size: r.filelistsXML.size,
			OpenChecksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.filelistsXML.openChecksum.Sum(nil)),
			},
			OpenSize: r.filelistsXML.openSize,
			Location: &yummeta.RepoMdDataLocation{
				Href: filepath.Join("repodata", filelistsXMLGzipFile),
			},
			Timestamp: now,
		},
		{
			Type: "other",
			Checksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: otherChecksum,
			},
			Size: r.otherXML.size,
			OpenChecksum: &yummeta.RepoMdDataChecksum{
				Type:  "sha256",
				Value: fmt.Sprintf("%x", r.otherXML.openChecksum.Sum(nil)),
			},
			OpenSize: r.otherXML.openSize,
			Location: &yummeta.RepoMdDataLocation{
				Href: filepath.Join("repodata", otherXMLGzipFile),
			},
			Timestamp: now,
		},
	}

	if err := r.saveRepomd(repomdRoot); err != nil {
		return err
	}

	repodataDir := filepath.Dir(r.repomdXMLPath)

	if err := generateSQLiteFiles(repodataDir); err != nil {
		return err
	}

	repomdRoot, err = r.getRepomd()
	if err != nil {
		return err
	}

	metadataLayers := make([]oras.Layer, 0, 3)

	sqliteFiles := map[string]string{
		primarySQLiteFile:   orasrpm.PrimarySQLiteLayerType,
		filelistsSQLiteFile: orasrpm.FilelistsSQLiteLayerType,
		otherSQLiteFile:     orasrpm.OtherSQLiteLayerType,
	}

	for sqliteFile, mediatype := range sqliteFiles {
		path := filepath.Join(repodataDir, sqliteFile)
		layer, err := newGenericRPMMetadata(path, mediatype, nil)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(layer))
	}

	for _, data := range repomdRoot.Data {
		switch data.Type {
		case "primary":
			data.Location.Href = fmt.Sprintf("repodata/%s-%s", "sha256:"+primaryChecksum, primaryXMLGzipFile)
		case "filelists":
			data.Location.Href = fmt.Sprintf("repodata/%s-%s", "sha256:"+filelistsChecksum, filelistsXMLGzipFile)
		case "other":
			data.Location.Href = fmt.Sprintf("repodata/%s-%s", "sha256:"+otherChecksum, otherXMLGzipFile)
		case "primary_db":
			for _, layer := range metadataLayers {
				mt, _ := layer.MediaType()
				if mt == orasrpm.PrimarySQLiteLayerType {
					h, _ := layer.Digest()
					data.Location.Href = fmt.Sprintf("repodata/%s-%s", h.String(), primarySQLiteFile)
					break
				}
			}
		case "filelists_db":
			for _, layer := range metadataLayers {
				mt, _ := layer.MediaType()
				if mt == orasrpm.FilelistsSQLiteLayerType {
					h, _ := layer.Digest()
					data.Location.Href = fmt.Sprintf("repodata/%s-%s", h.String(), filelistsSQLiteFile)
					break
				}
			}
		case "other_db":
			for _, layer := range metadataLayers {
				mt, _ := layer.MediaType()
				if mt == orasrpm.OtherSQLiteLayerType {
					h, _ := layer.Digest()
					data.Location.Href = fmt.Sprintf("repodata/%s-%s", h.String(), otherSQLiteFile)
					break
				}
			}
		}
	}

	metadataLayers = append(
		metadataLayers,
		orasrpm.NewRPMMetadataLayer(r.primaryXML),
		orasrpm.NewRPMMetadataLayer(r.filelistsXML),
		orasrpm.NewRPMMetadataLayer(r.otherXML),
	)

	if err := r.saveRepomd(repomdRoot); err != nil {
		return err
	}

	repomdLayer, err := newGenericRPMMetadata(r.repomdXMLPath, orasrpm.RepomdXMLLayerType, nil)
	if err != nil {
		return err
	}
	metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(repomdLayer))

	metadataPusher := orasrpm.NewRPMMetadataPusher(pushRef, metadataLayers...)

	return oras.Push(metadataPusher, plugin.remoteOptions...)
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

func extractPackageMetadata(tmpDir, repoDir, packageFilename, href string) (string, error) {
	cmd := exec.Command(
		"createrepo_c",
		"--no-database",
		"--simple-md-filenames",
		"--general-compress-type=gz",
		"-n", packageFilename,
		"-o", tmpDir,
		tmpDir,
	)

	if err := cmd.Run(); err != nil {
		return "", err
	}

	repodataDir := filepath.Join(tmpDir, "repodata")

	relativeDir := filepath.Join("Packages", string(packageFilename[0]), packageFilename)
	packageDir := filepath.Join(repoDir, relativeDir)
	if err := os.MkdirAll(packageDir, 0o700); err != nil {
		return "", err
	}

	primaryXMLInput := filepath.Join(repodataDir, primaryXMLGzipFile)
	primaryXMLOutput := filepath.Join(packageDir, primaryXMLFile)

	if err := extractMetadataTo(primaryXMLInput, primaryXMLOutput, &yummeta.PrimaryRoot{}, href); err != nil {
		return "", err
	}

	filelistsXMLInput := filepath.Join(repodataDir, filelistsXMLGzipFile)
	filelistsXMLOutput := filepath.Join(packageDir, filelistsXMLFile)

	if err := extractMetadataTo(filelistsXMLInput, filelistsXMLOutput, &yummeta.FilelistsRoot{}, href); err != nil {
		return "", err
	}

	otherXMLInput := filepath.Join(repodataDir, otherXMLGzipFile)
	otherXMLOutput := filepath.Join(packageDir, otherXMLFile)

	if err := extractMetadataTo(otherXMLInput, otherXMLOutput, &yummeta.OtherRoot{}, href); err != nil {
		return "", err
	}

	return packageDir, nil
}

func extractMetadataTo(input, output string, root yummeta.XMLRoot, href string) (errFn error) {
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	gr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}

	if err := xml.NewDecoder(gr).Decode(root); err != nil {
		return err
	}

	root.Href(href)

	return os.WriteFile(output, root.Data(), 0o600)
}

type genericRPMMetadata struct {
	path        string
	digest      string
	mediatype   string
	size        int64
	annotations map[string]string
}

func newGenericRPMMetadata(path string, mediatype string, annotations map[string]string) (*genericRPMMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	checksum := sha256.New()

	_, err = io.Copy(checksum, f)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return &genericRPMMetadata{
		path:        path,
		digest:      fmt.Sprintf("%x", checksum.Sum(nil)),
		mediatype:   mediatype,
		size:        fi.Size(),
		annotations: annotations,
	}, nil
}

func (g *genericRPMMetadata) Path() string {
	return g.path
}

func (g *genericRPMMetadata) Digest() (string, string) {
	return "sha256", g.digest
}

func (g *genericRPMMetadata) Size() int64 {
	return g.size
}

func (g *genericRPMMetadata) Mediatype() string {
	return g.mediatype
}

func (g *genericRPMMetadata) Annotations() map[string]string {
	return g.annotations
}
