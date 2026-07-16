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

// listRecord is a STATUS_LIST row: one status list's allocator counter and lifecycle state.
type listRecord struct {
	id        string
	bits      int
	state     int
	nextIdx   int64
	capacity  int64
	createdAt time.Time
	sealedAt  time.Time // zero value when the list is still active (SEALED_AT is NULL)
}

// entryRecord is a STATUS_LIST_ENTRY row: the status of one revoked referenced token by its index.
type entryRecord struct {
	idx    int64
	status byte
}
