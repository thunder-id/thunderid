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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package revocationcache

import "time"

// Config holds the Resource Server token-revocation cache settings, mapped by the caller from the
// deployment server security config. It is intentionally decoupled from system/config so this
// package does not depend on the global configuration type.
type Config struct {
	// Enabled turns RS revocation enforcement on. When false, Initialize returns a no-op enforcer.
	Enabled bool
	// RefreshInterval bounds how stale a cached status list may be: once a cached list is older than
	// this, the next lookup that needs it re-fetches from the source. A non-positive value falls back
	// to the built-in default.
	RefreshInterval time.Duration
}
