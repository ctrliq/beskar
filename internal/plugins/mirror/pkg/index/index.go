// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"bytes"
	_ "embed"
	"html/template"
	"time"
)

//go:embed embedded/index.html.tpl
var indexTpl string

type Directory struct {
	Name  string
	Ref   string
	MTime time.Time
}

type File struct {
	Name  string
	Ref   string
	MTime time.Time
	Size  uint64
}

type Config struct {
	Current     string
	Previous    string
	Directories []Directory
	Files       []File
}

func Generate(c Config) ([]byte, error) {
	buf := &bytes.Buffer{}
	t, err := template.New("index").Parse(indexTpl)
	if err != nil {
		return nil, err
	}

	err = t.ExecuteTemplate(buf, "index", &c)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
