package orasrpm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/pkg/oras"
)

const (
	RPMPackageLayerType = "application/vnd.ciq.rpm-package.v1.bin"
)

// RPMConfigTypes is the supported config types for RPM packages. Default type first.
var RPMConfigTypes = []types.MediaType{
	"application/vnd.ciq.rpm-package.v1.config+json",
}

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
	for _, t := range RPMConfigTypes {
		if config.MediaType == t {
			return nil
		}
	}
	return ErrNoRPMConfig
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

var _ oras.Pusher = &RPMPusher{}

func NewRPMPusher(ref name.Reference, path string, opts ...RPMLayerOption) *RPMPusher {
	return &RPMPusher{
		ref:     ref,
		path:    path,
		options: opts,
	}
}

// RPMPusher type to push RPM packages to registry.
type RPMPusher struct {
	ref     name.Reference
	path    string
	options []RPMLayerOption
}

func (rp *RPMPusher) Reference() name.Reference {
	return rp.ref
}

func (rp *RPMPusher) ImageIndex() (v1.ImageIndex, error) {
	return nil, oras.ErrNoImageIndexToPush
}

func (rp *RPMPusher) Image() (v1.Image, error) {
	img := oras.NewImage()
	rpm := NewRPMLayer(rp.path, rp.options...)

	if err := img.AddConfig(RPMConfigTypes[0], nil); err != nil {
		return nil, fmt.Errorf("while adding RPM config: %w", err)
	}

	if err := img.AddLayer(rpm); err != nil {
		return nil, fmt.Errorf("while adding RPM package layer: %w", err)
	}

	return img, nil
}

type RPMLayerOption func(*RPMLayer)

func WithRPMLayerAnnotations(annotations map[string]string) RPMLayerOption {
	return func(l *RPMLayer) {
		l.annotations = annotations
	}
}

func WithRPMLayerPlatform(platform *v1.Platform) RPMLayerOption {
	return func(l *RPMLayer) {
		l.platform = platform
	}
}

// RPMLayer defines an OCI layer descriptor associated to a RPM package.
type RPMLayer struct {
	path string

	digestOnce sync.Once
	digest     v1.Hash

	annotations map[string]string
	platform    *v1.Platform
}

// NewRPMLayer returns an OCI layer descriptor for the corresponding
// RPM package.
func NewRPMLayer(path string, options ...RPMLayerOption) *RPMLayer {
	layer := &RPMLayer{
		path: path,
	}
	for _, opt := range options {
		opt(layer)
	}
	if layer.annotations == nil {
		layer.annotations = make(map[string]string)
	}
	return layer
}

// Digest returns the Hash of the RPM package.
func (l *RPMLayer) Digest() (v1.Hash, error) {
	var err error

	l.digestOnce.Do(func() {
		var f *os.File

		f, err = os.Open(l.path)
		if err != nil {
			return
		}
		defer f.Close()

		l.digest, _, err = v1.SHA256(f)
	})

	return l.digest, err
}

// DiffID returns the Hash of the uncompressed layer (not supported by ORAS).
func (l *RPMLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("not supported by ORAS")
}

// Compressed returns an io.ReadCloser for the RPM file content.
func (l *RPMLayer) Compressed() (io.ReadCloser, error) {
	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents
// (not supported by ORAS).
func (l *RPMLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported by ORAS")
}

// Size returns the size of the RPM file.
func (l *RPMLayer) Size() (int64, error) {
	st, err := os.Stat(l.path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// MediaType returns the media type of the Layer.
func (l *RPMLayer) MediaType() (types.MediaType, error) {
	return types.MediaType(RPMPackageLayerType), nil
}

// Annotations returns annotations associated to this layer.
func (l *RPMLayer) Annotations() map[string]string {
	if l.annotations[imagespec.AnnotationTitle] == "" {
		l.annotations[imagespec.AnnotationTitle] = filepath.Base(l.path)
	}
	return l.annotations
}

// Platform returns platform information for this layer.
func (l *RPMLayer) Platform() *v1.Platform {
	return l.platform
}
