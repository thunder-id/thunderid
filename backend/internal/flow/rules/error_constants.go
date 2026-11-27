// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import "errors"

// errNegationArity is returned when a negation without exactly one condition is encoded.
var errNegationArity = errors.New("a negation needs exactly one condition")
