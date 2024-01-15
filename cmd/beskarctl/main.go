// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/cmd/beskarctl/ostree"
	"go.ciq.dev/beskar/cmd/beskarctl/static"
	"go.ciq.dev/beskar/cmd/beskarctl/yum"
)

func main() {
	ctl.Execute(
		yum.RootCmd(),
		static.RootCmd(),
		ostree.RootCmd(),
	)
}
