// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package orasrpm

import (
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

func (rp *RPMPuller) Reference() name.Reference {
	return rp.ref
}

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

func (rp *RPMPuller) RawConfig(_ []byte) error {
	return nil
}

func (rp *RPMPuller) Config(config v1.Descriptor) error {
	if config.MediaType != RPMConfigType {
		return ErrNoRPMConfig
	}
	return nil
}

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

func NewRPMPusher(path, repo string, opts ...name.Option) (oras.Pusher, error) {
	if !strings.HasPrefix(repo, "artifacts/") {
		if !strings.HasPrefix(repo, "yum/") {
			repo = filepath.Join("artifacts", "yum", repo)
		} else {
			repo = filepath.Join("artifacts", repo)
		}
	}

	rpmFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("while opening %s: %w", path, err)
	}
	defer rpmFile.Close()

	pkg, err := rpm.Read(rpmFile)
	if err != nil {
		return nil, fmt.Errorf("while reading %s metadata: %w", path, err)
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
		return nil, fmt.Errorf("while parsing reference %s: %w", rawRef, err)
	}

	return oras.NewGenericPusher(
		ref,
		oras.NewManifestConfig(RPMConfigType, nil),
		NewRPMLayer(path, DefaultRPMLayerOptions(rpmName, arch)...),
	), nil
}

func DefaultRPMLayerOptions(rpmName, arch string) []oras.LocalFileLayerOption {
	return []oras.LocalFileLayerOption{
		oras.WithLocalFileLayerPlatform(
			&v1.Platform{
				Architecture: arch,
				OS:           "linux",
			},
		),
		oras.WithLocalFileLayerAnnotations(map[string]string{
			imagespec.AnnotationTitle: rpmName,
		}),
	}
}

// rpmLayer defines an OCI layer descriptor associated to a RPM package.
type rpmLayer struct {
	*oras.LocalFileLayer
}

// NewRPMLayer returns an OCI layer descriptor for the corresponding
// RPM package.
func NewRPMLayer(path string, options ...oras.LocalFileLayerOption) oras.Layer {
	options = append(options, oras.WithLocalFileLayerMediaType(RPMPackageLayerType))
	return &rpmLayer{
		LocalFileLayer: oras.NewLocalFileLayer(path, options...),
	}
}
