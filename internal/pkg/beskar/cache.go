package beskar

import (
	"bytes"
	"context"
	"encoding/gob"
	"time"

	"github.com/distribution/distribution/v3"
	"github.com/mailgun/groupcache/v2"
	"github.com/opencontainers/go-digest"
)

type ManifestSink struct {
	groupcache.Sink
	byteView        *groupcache.ByteView
	manifestService distribution.ManifestService
	options         []distribution.ManifestServiceOption
}

func newManifestSink(manifestService distribution.ManifestService, options ...distribution.ManifestServiceOption) *ManifestSink {
	byteView := &groupcache.ByteView{}
	return &ManifestSink{
		Sink:            groupcache.ByteViewSink(byteView),
		byteView:        byteView,
		manifestService: manifestService,
		options:         options,
	}
}

func (ms *ManifestSink) FromManifest(manifest distribution.Manifest) error {
	mt, payload, err := manifest.Payload()
	if err != nil {
		return err
	}

	value, err := encodeManifest(mt, payload)
	if err != nil {
		return err
	}

	return ms.Sink.SetBytes(value, time.Now().Add(1*time.Hour))
}

func (ms *ManifestSink) ToManifest() (distribution.Manifest, error) {
	var gm gobManifest

	reader := ms.byteView.Reader()
	if err := gob.NewDecoder(reader).Decode(&gm); err != nil {
		return nil, err
	}

	manifest, _, err := distribution.UnmarshalManifest(gm.MediaType, gm.Payload)

	return manifest, err
}

type gobManifest struct {
	MediaType string
	Payload   []byte
}

func encodeManifest(mediaType string, payload []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	err := gob.NewEncoder(buf).Encode(&gobManifest{
		MediaType: mediaType,
		Payload:   payload,
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type cacheGetter struct{}

func (cacheGetter) Get(ctx context.Context, key string, dest groupcache.Sink) error {
	manifestSink, ok := dest.(*ManifestSink)
	if !ok {
		return nil
	}
	dgst := digest.Digest(key)

	manifest, err := manifestSink.manifestService.Get(ctx, dgst, manifestSink.options...)
	if err != nil {
		return err
	}

	return manifestSink.FromManifest(manifest)
}
