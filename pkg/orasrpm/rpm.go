// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasrpm

import (
	"bytes"
	"crypto/md5" //nolint:gosec
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cavaliergopher/rpm"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/pkg/ioutil"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	RPMConfigType       = "application/vnd.ciq.rpm.package.v1.config+json"
	RPMPackageLayerType = "application/vnd.ciq.rpm.package.v1.rpm"
)

var ErrNoRPMConfig = errors.New("RPM config not found")

var _ oras.Puller = &RPMPuller{}

// NewRPMPuller returns a puller instance to pull RPM package from
// the reference and write image content to the writer.
func NewRPMPuller(ref name.Reference, writer io.Writer) *RPMPuller {
	return &RPMPuller{
		ref:    ref,
		writer: writer,
	}
}

// RPMPuller type to pull RPM package from registry.
type RPMPuller struct {
	ref    name.Reference
	writer io.Writer
}

// Reference returns the reference for the puller.
func (rp *RPMPuller) Reference() name.Reference {
	return rp.ref
}

// IndexManifest returns the manifest digest corresponding
// to the current architecture.
func (rp *RPMPuller) IndexManifest(index *v1.IndexManifest) *v1.Hash {
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

// RawConfig sets the raw config, ignored for RPM.
func (rp *RPMPuller) RawConfig(_ []byte) error {
	return nil
}

// Config validates the config mediatype.
func (rp *RPMPuller) Config(config v1.Descriptor) error {
	if config.MediaType != RPMConfigType {
		return ErrNoRPMConfig
	}
	return nil
}

// Layers copy layers to the pull writer.
func (rp *RPMPuller) Layers(layers []v1.Layer) error {
	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			return fmt.Errorf("while getting RPM layer media type: %w", err)
		} else if mt != RPMPackageLayerType {
			continue
		}

		rc, err := l.Compressed()
		if err != nil {
			return err
		}
		defer rc.Close()

		if _, err := io.Copy(rp.writer, rc); err != nil {
			return fmt.Errorf("while copying RPM package: %w", err)
		}

		return nil
	}
	return fmt.Errorf("no RPM layer found for %s", rp.ref.Name())
}

// RPMReferenceLayer returns the reference and the layer options for a RPM package.
func RPMReferenceLayer(r io.Reader, repo string, opts ...name.Option) (name.Reference, []oras.LayerOption, error) {
	if !strings.HasPrefix(repo, "artifacts/") {
		if !strings.HasPrefix(repo, "yum/") {
			repo = filepath.Join("artifacts", "yum", repo)
		} else {
			repo = filepath.Join("artifacts", repo)
		}
	}

	pkg, err := rpm.Read(r)
	if err != nil {
		return nil, nil, fmt.Errorf("while reading RPM metadata: %w", err)
	}

	arch := pkg.Architecture()
	if pkg.SourceRPM() == "" {
		arch = "src"
	}

	rpmName := fmt.Sprintf("%s-%s-%s.%s.rpm", pkg.Name(), pkg.Version(), pkg.Release(), arch)
	//nolint:gosec
	pkgTag := fmt.Sprintf("%x", md5.Sum([]byte(rpmName)))

	rawRef := filepath.Join(repo, "packages:"+pkgTag)
	ref, err := name.ParseReference(rawRef, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("while parsing reference %s: %w", rawRef, err)
	}

	return ref, DefaultRPMLayerOptions(rpmName, arch), nil
}

// NewRPMPuller returns a pusher instance to push a local RPM package.
func NewRPMPusher(path, repo string, opts ...name.Option) (oras.Pusher, error) {
	rpmFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("while opening %s: %w", path, err)
	}
	defer rpmFile.Close()

	ref, rpmLayerOptions, err := RPMReferenceLayer(rpmFile, repo, opts...)
	if err != nil {
		return nil, err
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(RPMConfigType, nil),
		oras.NewLocalFileLayer(path, rpmLayerOptions...),
	), nil
}

type fullStream struct {
	io.Reader
}

func (fs *fullStream) Read(p []byte) (int, error) {
	return io.ReadFull(fs.Reader, p)
}

// NewRPMStreamPusher returns a pusher instance to push a RPM package from a stream.
func NewRPMStreamPusher(stream io.Reader, repo string, opts ...name.Option) (oras.Pusher, string, error) {
	buf := new(bytes.Buffer)

	streamReader := io.TeeReader(&fullStream{stream}, buf)

	ref, rpmLayerOptions, err := RPMReferenceLayer(streamReader, repo, opts...)
	if err != nil {
		return nil, "", err
	}

	streamLayer := oras.NewStreamLayer(
		ioutil.MultiReaderCloser(buf, stream),
		rpmLayerOptions...,
	)

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(RPMConfigType, nil),
		streamLayer,
	), streamLayer.Annotations()[imagespec.AnnotationTitle], nil
}

// DefaultRPMLayerOptions returns the default layer options for a RPM package.
func DefaultRPMLayerOptions(rpmName, arch string) []oras.LayerOption {
	return []oras.LayerOption{
		oras.WithLayerPlatform(
			&v1.Platform{
				Architecture: arch,
				OS:           "linux",
			},
		),
		oras.WithLayerAnnotations(map[string]string{
			imagespec.AnnotationTitle: rpmName,
		}),
		oras.WithLayerMediaType(RPMPackageLayerType),
	}
}
