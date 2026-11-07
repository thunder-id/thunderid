// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package stdoutsink

import "errors"

var errLineTooLong = errors.New("audit line does not fit the line size limit")
