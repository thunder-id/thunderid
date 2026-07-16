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

import "errors"

var (
	// errInvalidBits indicates an entry width other than the spec-permitted 1, 2, 4, or 8.
	errInvalidBits = errors.New("tokenstatus: bits per entry must be 1, 2, 4, or 8")
	// errInvalidSize indicates a negative status-list size.
	errInvalidSize = errors.New("tokenstatus: size must not be negative")
	// errIndexOutOfRange indicates an entry index outside the allocated bit array.
	errIndexOutOfRange = errors.New("tokenstatus: index out of range")
	// errListTooLarge indicates a decoded status list whose inflated size exceeds the accepted bound,
	// rejected to prevent a decompression bomb from exhausting memory.
	errListTooLarge = errors.New("tokenstatus: decoded status list exceeds maximum size")
	// errAllocationExhausted indicates the index allocator retried past its bound under contention.
	errAllocationExhausted = errors.New("tokenstatus: index allocation exhausted retries")
	// errInvalidListURI indicates a status list URI that does not carry a resolvable list id.
	errInvalidListURI = errors.New("tokenstatus: invalid status list URI")
	// errEmptyBaseURL indicates the subsystem was initialized without a base URL to build list URIs.
	errEmptyBaseURL = errors.New("tokenstatus: base URL must not be empty when enabled")
	// errNilJWTService indicates the subsystem was initialized without a signer to produce list tokens.
	errNilJWTService = errors.New("tokenstatus: JWT service must not be nil when enabled")
	// errNonPositiveTTL indicates a non-positive TTL, which would publish a Status List Token whose ttl
	// claim is zero while its exp is set from the signer's default validity — an inconsistent token.
	errNonPositiveTTL = errors.New("tokenstatus: TTL must be positive when enabled")
	// ErrListNotFound indicates a requested status list id does not exist. It is exported because the
	// composition-root adapter that feeds the Resource Server cache maps it to a not-found result.
	ErrListNotFound = errors.New("tokenstatus: status list not found")
	// errMalformedStatusListToken indicates a Status List Token whose status_list claim is missing or
	// not shaped as the spec requires, so its bit array cannot be recovered.
	errMalformedStatusListToken = errors.New("tokenstatus: malformed status list token")
)
