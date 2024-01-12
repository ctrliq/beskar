// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yummeta

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var expectedPrimaryPackages = []PrimaryPackage{
	{
		ID:   "7e6e14dc80f29ab22894c3f854fedd4973546c1713d98c6897b25b7d728f50fa",
		Href: "Packages/n/NetworkManager-initscripts-updown-1.40.16-3.el8_8.noarch.rpm",
	},
	{
		ID:   "0cf9f96f80808ca6ce9804779d9efc64cc564c8b7cbb98afc5c5f1315e7340cd",
		Href: "Packages/n/NetworkManager-initscripts-updown-1.40.16-4.el8_8.noarch.rpm",
	},
}

func TestWalkPrimaryPackages(t *testing.T) {
	r, err := os.Open("testdata/primary.xml")
	require.NoError(t, err)
	defer r.Close()

	i := 0

	err = WalkPrimaryPackages(r, func(pkg PrimaryPackage, totalPackages int) error {
		if expectedPrimaryPackages[i] != pkg {
			return fmt.Errorf("package mismatch")
		} else if totalPackages != 2 {
			return fmt.Errorf("total packages mismatch")
		}
		i++
		return nil
	})

	require.NoError(t, err)
}
