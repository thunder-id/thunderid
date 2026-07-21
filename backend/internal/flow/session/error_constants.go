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

package session

import "errors"

var (
	// errVersionConflict is returned by Update when the optimistic-lock version no longer
	// matches (the row was updated concurrently or no longer exists).
	errVersionConflict = errors.New("session version conflict")

	// errSessionContextTooLarge is returned when a serialized session context exceeds
	// MaxSessionContextBytes. The bounded snapshot keeps the sibling row small.
	errSessionContextTooLarge = errors.New("session context exceeds maximum size")
)
