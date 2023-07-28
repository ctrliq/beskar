package beskar

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"github.com/distribution/distribution/v3"
	"github.com/mailgun/groupcache/v2"
	"github.com/opencontainers/go-digest"
)

func getCacheKey(repository distribution.Repository, dgst digest.Digest) string {
	return fmt.Sprintf("%s@%s", repository.Named().Name(), dgst.String())
}

type ManifestSink struct {
	groupcache.Sink
	value           []byte
	manifestService distribution.ManifestService
	options         []distribution.ManifestServiceOption
}

func newManifestSink(manifestService distribution.ManifestService, options ...distribution.ManifestServiceOption) *ManifestSink {
	s := &ManifestSink{
		manifestService: manifestService,
		options:         options,
	}
	s.Sink = groupcache.AllocatingByteSliceSink(&s.value)
	return s
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

	if err := gob.NewDecoder(bytes.NewReader(ms.value)).Decode(&gm); err != nil {
		return nil, err
	}

	manifest, _, err := distribution.UnmarshalManifest(gm.MediaType, gm.Payload)
	if err != nil {
		return nil, err
	}

	return manifest, nil
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

	idx := strings.Index(key, "@")
	if idx < 0 || idx+1 == len(key) {
		return fmt.Errorf("wrong cache key format")
	}
	dgst := digest.Digest(key[idx+1:])

	manifest, err := manifestSink.manifestService.Get(ctx, dgst, manifestSink.options...)
	if err != nil {
		return err
	}

	return manifestSink.FromManifest(manifest)
}
