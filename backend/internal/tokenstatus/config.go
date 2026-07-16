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

package tokenstatus

import "time"

// Config configures the Token Status List subsystem. It is deliberately decoupled from the engine
// configuration types so the subsystem can be wired from any composition root (in-process AS today, a
// standalone Status Provider later). BaseURL is the origin used to build published list URIs; ListSize
// and Bits are stamped into lists this instance creates; TTL bounds the freshness of a published token.
type Config struct {
	Enabled  bool
	BaseURL  string
	ListSize int64
	Bits     int
	TTL      time.Duration
	// MaxTokenTTL is the longest lifetime of any token this instance references. Initialize derives the
	// sealed-list retention window from it (adding a safety grace) so a list is dropped only once every
	// token it covers has expired; dropping it earlier would let a revoked token's status become
	// unresolvable. The composition root passes the maximum of the configured token validities. A
	// non-positive value disables reaping.
	MaxTokenTTL time.Duration
}
