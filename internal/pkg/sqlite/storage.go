// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/pierrec/lz4"
	"gocloud.dev/blob"
)

type Storage struct {
	Bucket     *blob.Bucket
	DataDir    string
	Repository string

	SchemaFS   fs.FS
	SchemaGlob string

	Filename           string
	CompressedFilename string
}

func newBuffer() interface{} {
	buffer := make([]byte, 1024*1024)
	return &buffer
}

var bufferPool = sync.Pool{
	New: newBuffer,
}

func push(path string, remoteWriter io.Writer) error {
	lw := lz4.NewWriter(remoteWriter)

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := bufferPool.Get().(*[]byte)
	_, err = io.CopyBuffer(lw, f, *buf)
	bufferPool.Put(buf)

	lwErr := lw.Close()
	if err == nil {
		err = lwErr
	}

	return err
}

func pull(path string, remoteReader io.Reader) error {
	lr := lz4.NewReader(remoteReader)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}

	buf := bufferPool.Get().(*[]byte)
	_, err = io.CopyBuffer(f, lr, *buf)
	bufferPool.Put(buf)

	closeErr := f.Close()
	if err != nil {
		return err
	} else if closeErr != nil {
		return closeErr
	}

	return nil
}
