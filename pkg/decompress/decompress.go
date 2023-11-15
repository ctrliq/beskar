// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package decompress

import (
	"bytes"
	"compress/bzip2"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

var decompressors = []struct {
	header []byte
	reader func(io.Reader) (io.Reader, error)
}{
	{
		header: []byte{0x1F, 0x8B},
		reader: func(r io.Reader) (io.Reader, error) {
			return gzip.NewReader(r)
		},
	},
	{
		header: []byte{0x42, 0x5A, 0x68},
		reader: func(r io.Reader) (io.Reader, error) {
			return bzip2.NewReader(r), nil
		},
	},
	{
		header: []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00},
		reader: func(r io.Reader) (io.Reader, error) {
			return xz.NewReader(r)
		},
	},
	{
		header: []byte{0x28, 0xB5, 0x2F, 0xFD},
		reader: func(r io.Reader) (io.Reader, error) {
			return zstd.NewReader(r)
		},
	},
}

type HashBuffer struct {
	hash.Hash
	buffer    *bytes.Buffer
	bytesRead int
}

func NewHashBuffer(h hash.Hash, buffer *bytes.Buffer) *HashBuffer {
	return &HashBuffer{
		Hash:   h,
		buffer: buffer,
	}
}

func (hb *HashBuffer) Write(p []byte) (int, error) {
	n, err := hb.Hash.Write(p)
	if err != nil {
		return n, err
	}
	hb.bytesRead += n
	if hb.buffer != nil {
		if _, err := hb.buffer.Write(p); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (hb *HashBuffer) BytesRead() int {
	return hb.bytesRead
}

func (hb *HashBuffer) Bytes() []byte {
	if hb.buffer != nil {
		return hb.buffer.Bytes()
	}
	return nil
}

func (hb *HashBuffer) Hex() string {
	return hex.EncodeToString(hb.Sum(nil))
}

func (hb *HashBuffer) Reset() {
	hb.bytesRead = 0
	hb.Hash.Reset()
	if hb.buffer != nil {
		hb.buffer.Reset()
	}
}

type readCloser struct {
	io.Reader
	io.Closer
}

func (rc readCloser) Close() error {
	if rc.Closer != nil {
		return rc.Closer.Close()
	}
	return nil
}

type FileOption func(f *file)

type file struct {
	io.Reader
	compressed io.ReadSeekCloser
	h          hash.Hash
	oh         hash.Hash
}

func (f *file) Close() error {
	if c, ok := f.Reader.(io.Closer); ok {
		_ = c.Close()
	}
	if f.compressed != nil {
		return f.compressed.Close()
	}
	return nil
}

func WithHash(h hash.Hash) FileOption {
	return func(f *file) {
		f.h = h
	}
}

func WithOpenHash(oh hash.Hash) FileOption {
	return func(f *file) {
		f.oh = oh
	}
}

func File(path string, opts ...FileOption) (io.ReadCloser, error) {
	var err error

	file := &file{}

	for _, opt := range opts {
		opt(file)
	}

	file.compressed, err = os.Open(path)
	if err != nil {
		return nil, err
	}

	header := make([]byte, 8)
	if _, err := file.compressed.Read(header); err != nil {
		return nil, err
	}
	if _, err := file.compressed.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	for _, decomp := range decompressors {
		if len(header) < len(decomp.header) || !bytes.Equal(decomp.header, header[:len(decomp.header)]) {
			continue
		}

		var r io.Reader = file.compressed

		if file.h != nil {
			r = io.TeeReader(r, file.h)
		}

		file.Reader, err = decomp.reader(r)
		if err != nil {
			return nil, err
		} else if file.oh != nil {
			rc := &readCloser{
				Reader: io.TeeReader(file.Reader, file.oh),
			}
			if closer, ok := file.Reader.(io.Closer); ok {
				rc.Closer = closer
			}
			file.Reader = rc
		}

		break
	}

	if file.Reader == nil {
		file.Reader = file.compressed
		if file.h != nil {
			rc := &readCloser{
				Reader: io.TeeReader(file.Reader, file.h),
			}
			if closer, ok := file.Reader.(io.Closer); ok {
				rc.Closer = closer
			}
			file.Reader = rc
		}
		file.compressed = nil
	}

	return file, nil
}
