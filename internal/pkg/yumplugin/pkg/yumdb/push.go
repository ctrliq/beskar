// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumdb

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pierrec/lz4/v4"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 1024*1024)
		return &buffer
	},
}

func Push(path string, remoteWriter io.Writer) error {
	lz := lz4.NewWriter(remoteWriter)

	tw := tar.NewWriter(lz)
	defer tw.Close()

	buf := bufferPool.Get().(*[]byte)

	err := filepath.WalkDir(path, func(file string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := de.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, de.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(strings.Replace(file, path, "", -1), string(filepath.Separator))

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !de.Type().IsRegular() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.CopyBuffer(tw, f, *buf)
		return err
	})

	bufferPool.Put(buf)

	lzErr := lz.Close()
	if err == nil {
		err = lzErr
	}

	return err
}
