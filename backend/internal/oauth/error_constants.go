// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import "errors"

// errNilEventBus is returned when the OAuth services are initialized without the event bus they audit on.
var errNilEventBus = errors.New("the OAuth services need an event bus")
