// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package doltdb

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDB(t *testing.T) {
	dbPath, err := os.MkdirTemp("", "db-")
	require.NoError(t, err)

	defer os.RemoveAll(dbPath)

	os.Clearenv()
	os.Setenv("HOME", dbPath)

	db, err := Open(dbPath, "testing")
	require.NoError(t, err)

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS t1 (id INT PRIMARY KEY, name VARCHAR(64))")
	require.NoError(t, err)

	rows, err := db.Queryx("SELECT table_name FROM dolt_status")
	require.NoError(t, err)

	for rows.Next() {
		table := ""
		rows.Scan(&table)
	}

	require.NoError(t, rows.Err())

	err = db.CommitAll(context.Background(), "New table")
	require.NoError(t, err)
}
