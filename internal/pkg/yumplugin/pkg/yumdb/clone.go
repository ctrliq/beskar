// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumdb

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"

	"github.com/pierrec/lz4/v4"
)

func Clone(path string, remoteReader io.Reader) error {
	tr := tar.NewReader(lz4.NewReader(remoteReader))

	buf := bufferPool.Get().(*[]byte)
	defer func() {
		bufferPool.Put(buf)
	}()

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		//nolint:gosec // internal use only
		target := filepath.Join(path, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			//nolint:gosec // internal use only
			_, err = io.CopyBuffer(f, tr, *buf)
			closeErr := f.Close()
			if err != nil {
				return err
			} else if closeErr != nil {
				return closeErr
			}
		}
	}
}
