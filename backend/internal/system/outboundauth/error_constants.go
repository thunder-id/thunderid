// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import "errors"

// errUnsupportedType reports a configuration naming an authentication method this build does
// not implement.
var errUnsupportedType = errors.New("unsupported authentication type")
