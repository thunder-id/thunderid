// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import "time"

// dialTimeout bounds the connection attempts of a client's dialer.
const dialTimeout = 10 * time.Second
