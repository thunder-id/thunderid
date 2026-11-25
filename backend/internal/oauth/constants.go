// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import "time"

// jwksFetchTimeout bounds the retrieval of a relying party's JWKS.
const jwksFetchTimeout = 30 * time.Second
