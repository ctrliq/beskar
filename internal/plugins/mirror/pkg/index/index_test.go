// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"bytes"
	_ "embed"
	"testing"
	"time"
)

//go:embed testdata/index.html
var goldenIndex []byte

func TestGenerate(t *testing.T) {
	// Prepare test data
	c := Config{
		Current:  "/foo/bar/",
		Previous: "/foo/",
		Directories: []Directory{
			{
				Name:  "dir1",
				Ref:   "ref1",
				MTime: time.Date(2024, time.January, 1, 2, 3, 0, 0, time.UTC),
			},
			{
				Name:  "dir2",
				Ref:   "ref2",
				MTime: time.Date(2024, time.January, 1, 2, 4, 0, 0, time.UTC),
			},
		},
		Files: []File{
			{
				Name:  "file1",
				Ref:   "ref3",
				MTime: time.Date(2024, time.January, 1, 2, 5, 0, 0, time.UTC),
				Size:  100,
			},
			{
				Name:  "file2",
				Ref:   "ref4",
				MTime: time.Date(2024, time.January, 1, 2, 6, 0, 0, time.UTC),
				Size:  200,
			},
		},
	}

	// Call the function under test
	result, err := Generate(c)
	// Check the result
	if err != nil {
		t.Errorf("Generate() returned an error: %v", err)
	}

	// Perform assertions on the result
	if !bytes.Equal(result, goldenIndex) {
		t.Errorf("Generate() returned incorrect result, got: %s, want: %s", result, goldenIndex)
	}
}
