// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yummeta

type XMLRoot interface {
	Data() []byte
	Href(string)
}
