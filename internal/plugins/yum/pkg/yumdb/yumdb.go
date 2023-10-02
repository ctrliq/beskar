// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumdb

import (
	"database/sql/driver"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pierrec/lz4"
	// load sqlite driver
	_ "modernc.org/sqlite"
)

const driverName = "sqlite"

type DBTime struct {
	time.Time
}

// Scan implements the Scanner interface.
func (dbt *DBTime) Scan(value any) error {
	if value == nil {
		dbt.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case int64:
		dbt.Time = time.Unix(v, 0)
	default:
		dbt.Time = time.Time{}
	}
	return nil
}

// Value implements the driver Valuer interface.
func (dbt DBTime) Value() (driver.Value, error) {
	return dbt.Time, nil
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
