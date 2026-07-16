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

// retentionGrace is the safety margin added to the longest token lifetime when deriving how long a
// sealed status list is retained before reaping — it absorbs clock skew and issuance lag so a list is
// never dropped while a live token still references it.
const retentionGrace = 24 * time.Hour

// statusListURISegment is the fixed path segment under which status lists are published; the list id
// follows it. The full URI is stamped into every referenced token and is immutable once issued, so the
// segment is a one-way door and must not encode identity (herd privacy).
const statusListURISegment = "/statuslists/"

// Status List Token wire claim names (draft-ietf-oauth-status-list §5). These are the subsystem's own
// copy of the wire keys; the subsystem never imports the OAuth packages, so it does not share the
// constants the token builder uses to stamp the referenced-token side.
const (
	claimAud        = "aud"
	claimTTL        = "ttl"
	claimStatusList = "status_list"
	claimBits       = "bits"
	claimLst        = "lst"
)

// Status values as defined by draft-ietf-oauth-status-list-21 §7.1. Only VALID and INVALID are used in
// the current single-token, 1-bit scope; SUSPENDED is listed for completeness of the value space.
const (
	// statusValid marks a referenced token as active (spec 0x00, VALID).
	statusValid byte = 0x00
	// statusInvalid marks a referenced token as revoked (spec 0x01, INVALID).
	statusInvalid byte = 0x01
)

// maxDecodedListBytes bounds the inflated size of a decoded status list to guard against a
// decompression bomb: a small ZLIB payload that expands until it exhausts process memory. It is the
// byte length of the largest list this implementation will accept, maxListEntries entries at the widest
// 8-bit width (one byte per entry), which is far above any realistic deployment list size.
const (
	maxListEntries      = 1 << 24 // 16,777,216
	maxDecodedListBytes = maxListEntries
)

// listStateActive marks a list that still accepts new index allocations.
const listStateActive = 0

// defaultListCapacity is the number of indices a list holds before it is sealed and a new one rolls in.
// Chosen large enough for herd privacy (draft-ietf-oauth-status-list §11); overridable via config.
const defaultListCapacity int64 = 100000

// maxAllocationAttempts bounds the compare-and-swap retry loop so a pathological contention or config
// error can never spin forever.
const maxAllocationAttempts = 16
