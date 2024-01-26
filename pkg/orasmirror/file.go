// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasmirror

import (
	"crypto/md5" //nolint:gosec
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	MirrorFileConfigType          = "application/vnd.ciq.mirror.file.v1.config+json"
	MirrorFileLayerType           = "application/vnd.ciq.mirror.v1.file"
	MirrorFileModeAnnotationType  = "application/vnd.ciq.mirror.v1.file.mode"
	MirrorFileMTimeAnnotationType = "application/vnd.ciq.mirror.v1.file.mtime"
)

var ErrNoMirrorFileConfig = errors.New("mirror file config not found")

func FileReference(filename, repo string, opts ...name.Option) (name.Reference, error) {
	if !strings.HasPrefix(repo, "artifacts/") {
		if !strings.HasPrefix(repo, "mirror/") {
			repo = filepath.Join("artifacts", "mirror", repo)
		} else {
			repo = filepath.Join("artifacts", repo)
		}
	}

	//nolint:gosec
	fileTag := fmt.Sprintf("%x", md5.Sum([]byte(filename)))

	rawRef := filepath.Join(repo, "files:"+fileTag)
	return name.ParseReference(rawRef, opts...)
}

func NewStaticFilePusher(path, repo string, opts ...name.Option) (oras.Pusher, error) {
	filename := filepath.Base(path)

	ref, err := FileReference(filename, repo, opts...)
	if err != nil {
		return nil, err
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(MirrorFileConfigType, nil),
		oras.NewLocalFileLayer(
			path,
			oras.WithLayerMediaType(MirrorFileLayerType),
		),
	), nil
}

func NewStaticFileStreamPusher(stream io.Reader, filename, repo string, opts ...name.Option) (oras.Pusher, error) {
	ref, err := FileReference(filename, repo, opts...)
	if err != nil {
		return nil, err
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(MirrorFileConfigType, nil),
		oras.NewStreamLayer(
			stream,
			oras.WithLayerMediaType(MirrorFileLayerType),
			oras.WithLayerAnnotations(map[string]string{
				imagespec.AnnotationTitle: filename,
			}),
		),
	), nil
}

var _ oras.Puller = &MirrorPuller{}

// NewMirrorPuller returns a puller instance to pull file from
// the reference and write image content to the writer.
func NewMirrorPuller(ref name.Reference, writer io.Writer) *MirrorPuller {
	return &MirrorPuller{
		ref:    ref,
		writer: writer,
	}
}

// MirrorPuller type to pull mirror file from registry.
type MirrorPuller struct {
	ref    name.Reference
	writer io.Writer
}

// Reference returns the reference for the puller.
func (rp *MirrorPuller) Reference() name.Reference {
	return rp.ref
}

// IndexManifest returns the manifest digest corresponding
// to the current architecture.
func (rp *MirrorPuller) IndexManifest(index *v1.IndexManifest) *v1.Hash {
	for _, manifest := range index.Manifests {
		platform := manifest.Platform
		if platform == nil {
			continue
		} else if platform.OS != "" && platform.OS != runtime.GOOS {
			continue
		} else if platform.Architecture != "" && platform.Architecture != runtime.GOARCH {
			continue
		}
		return &manifest.Digest
	}
	if len(index.Manifests) == 0 {
		return nil
	}
	return &index.Manifests[0].Digest
}

// RawConfig sets the raw config, ignored for mirror files.
func (rp *MirrorPuller) RawConfig(_ []byte) error {
	return nil
}

// Config validates the config mediatype.
func (rp *MirrorPuller) Config(config v1.Descriptor) error {
	if config.MediaType != MirrorFileConfigType {
		return ErrNoMirrorFileConfig
	}
	return nil
}

// Layers copy layers to the pull writer.
func (rp *MirrorPuller) Layers(layers []v1.Layer) error {
	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			return fmt.Errorf("while getting mirror file layer media type: %w", err)
		} else if mt != MirrorFileLayerType {
			continue
		}

		rc, err := l.Compressed()
		if err != nil {
			return err
		}
		defer rc.Close()

		if _, err := io.Copy(rp.writer, rc); err != nil {
			return fmt.Errorf("while copying mirror file : %w", err)
		}

		return nil
	}
	return fmt.Errorf("no mirror file layer found for %s", rp.ref.Name())
}
