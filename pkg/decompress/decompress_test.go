// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package decompress

import (
	"compress/gzip"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

func TestOpenFile(t *testing.T) {
	h := NewHashBuffer(sha256.New(), nil)
	oh := NewHashBuffer(sha256.New(), nil)

	expected := []byte("test")

	dir, err := os.MkdirTemp("", "compress-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	tests := []struct {
		name             string
		filename         string
		expectedHash     string
		expectedOpenHash string
		writer           func(io.Writer) (io.WriteCloser, error)
	}{
		{
			name:             "gzip",
			filename:         "gzip",
			expectedHash:     "c0fbeb16675b9d52fc67d55134f5c1256b414b8e47737e3398ba93b074d85bc3",
			expectedOpenHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			writer: func(w io.Writer) (io.WriteCloser, error) {
				return gzip.NewWriter(w), nil
			},
		},
		{
			name:             "xz",
			filename:         "xz",
			expectedHash:     "5faa24846664c0d75d045d8dcb4ae47025a29acb3ac5a4d7a906659cecd5dde0",
			expectedOpenHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			writer: func(w io.Writer) (io.WriteCloser, error) {
				return xz.NewWriter(w)
			},
		},
		{
			name:             "zstd",
			filename:         "zstd",
			expectedHash:     "d07ea39bc7f457a09c1a738a7fbd823ca8f9741a74bedae5b59f1ab8ae42fbcb",
			expectedOpenHash: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			writer: func(w io.Writer) (io.WriteCloser, error) {
				return zstd.NewWriter(w)
			},
		},
		{
			name:             "none",
			filename:         "none",
			expectedHash:     "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			expectedOpenHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			writer: func(w io.Writer) (io.WriteCloser, error) {
				return nil, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := os.Create(filepath.Join(dir, tt.filename))
			require.NoError(t, err)

			compW, err := tt.writer(w)
			require.NoError(t, err)

			expectedOpenSize := int64(len(expected))

			if compW != nil {
				_, err = compW.Write(expected)
				require.NoError(t, err)
				require.NoError(t, compW.Close())
			} else {
				_, err = w.Write(expected)
				require.NoError(t, err)
				expectedOpenSize = 0
			}

			fi, err := w.Stat()
			require.NoError(t, err)
			require.NoError(t, w.Close())

			f, err := File(w.Name(), WithHash(h), WithOpenHash(oh))
			require.NoError(t, err)

			res, err := io.ReadAll(f)
			require.NoError(t, f.Close())
			require.NoError(t, err)
			require.Equal(t, expected, res)

			require.Equal(t, tt.expectedHash, h.Hex())
			require.Equal(t, fi.Size(), h.BytesRead())

			require.Equal(t, tt.expectedOpenHash, oh.Hex())
			require.Equal(t, expectedOpenSize, oh.BytesRead())

			h.Reset()
			oh.Reset()
		})
	}
}
