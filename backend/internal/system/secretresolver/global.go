/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
