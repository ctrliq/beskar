//nolint:goheader
// Copyright (c) All respective contributors to the Peridot Project. All rights reserved.
// Copyright (c) 2021-2022 Rocky Enterprise Software Foundation, Inc. All rights reserved.
// Copyright (c) 2021-2022 Ctrl IQ, Inc. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
// this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation
// and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors
// may be used to endorse or promote products derived from this software without
// specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package yummeta

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

var hrefMatch = regexp.MustCompile(`(href=")(.*)(")`)

type PrimaryRoot struct {
	XMLName      xml.Name `xml:"metadata,omitempty"`
	Xmlns        string   `xml:"xmlns,attr,omitempty"`
	XmlnsRpm     string   `xml:"xmlns:rpm,attr,omitempty"`
	Rpm          string   `xml:"rpm,attr,omitempty"`
	PackageCount int      `xml:"packages,attr,omitempty"`
	Packages     []byte   `xml:",innerxml"`
}

func (r *PrimaryRoot) Data() []byte {
	return r.Packages
}

func (r *PrimaryRoot) Href(href string) {
	r.Packages = hrefMatch.ReplaceAll(r.Packages, []byte(fmt.Sprintf("${1}%s${3}", href)))
}

type PrimaryPackage struct {
	Href string
	ID   string
}

func WalkPrimaryPackages(r io.Reader, walkFn func(PrimaryPackage, int) error) error {
	var pkg PrimaryPackage

	decoder := xml.NewDecoder(r)
	pkgid := false
	totalPackages := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "package":
				pkg.Href = ""
				pkg.ID = ""
			case "checksum":
				pkgid = true
				for _, attr := range t.Attr {
					if attr.Name.Local == "type" && attr.Value != "sha256" {
						return fmt.Errorf("package checksum type %s not supported", attr.Value)
					}
				}
			case "location":
				for _, attr := range t.Attr {
					if attr.Name.Local == "href" {
						pkg.Href = attr.Value
					}
				}
			case "metadata":
				for _, attr := range t.Attr {
					if attr.Name.Local == "packages" {
						packages, err := strconv.ParseInt(attr.Value, 10, 32)
						if err != nil {
							return fmt.Errorf("total packages conversion error: %w", err)
						}
						totalPackages = int(packages)
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "package" {
				if err := walkFn(pkg, totalPackages); err != nil {
					return err
				}
			}
		case xml.CharData:
			if pkgid {
				pkg.ID = string(t)
				pkgid = !pkgid
			}
		}
	}
}
