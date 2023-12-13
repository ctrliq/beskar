// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ioutil

import (
	"io"
)

type multiReader struct {
	io.Reader
	readers []io.Reader
}

func (mr *multiReader) Close() error {
	for _, reader := range mr.readers {
		if closer, ok := reader.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// MultiReaderCloser wraps io.MultiReader and close readers
// implementing the io.Closer interface.
func MultiReaderCloser(readers ...io.Reader) io.ReadCloser {
	return &multiReader{
		Reader:  io.MultiReader(readers...),
		readers: readers,
	}
}
