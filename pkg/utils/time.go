// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package utils

import "time"

const timeFormat = time.DateTime + " MST"

func TimeToString(t int64) string {
	if t == 0 {
		return ""
	}
	return time.Unix(t, 0).Format(timeFormat)
}
