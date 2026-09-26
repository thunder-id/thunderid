// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package secretresolver

import "sync"

// The resolver is reached as a process-global because the value that needs resolving is read deep
// inside configuration accessors that take no dependencies, the same reason the configuration crypto
// service is a singleton. Set it once during startup, before any request is served.
var (
	defaultMu       sync.RWMutex
	defaultResolver *Resolver
)

// SetDefault installs the process-wide resolver.
func SetDefault(r *Resolver) {
	defaultMu.Lock()
	defaultResolver = r
	defaultMu.Unlock()
}

// Default returns the process-wide resolver. It is never nil: when none is installed the returned
// resolver is disabled, so a deployment without a secret provider behaves exactly as before.
func Default() *Resolver {
	defaultMu.RLock()
	r := defaultResolver
	defaultMu.RUnlock()
	if r == nil {
		return New(Config{})
	}
	return r
}
