/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package sqlstore

import "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"

// Test-local aliases to the authz types/values exercised by the relocated
// white-box store tests, so the test bodies stay identical after moving to the
// sqlstore package. These shims are test-only and add no production surface.
type (
	AuthorizationCode                  = authz.AuthorizationCode
	authRequestContext                 = authz.AuthRequestContext
	AuthorizationCodeStoreInterface    = authz.AuthorizationCodeStoreInterface
	AuthorizationRequestStoreInterface = authz.AuthorizationRequestStoreInterface
)

const (
	AuthCodeStateActive   = authz.AuthCodeStateActive
	AuthCodeStateInactive = authz.AuthCodeStateInactive
)

var errAuthorizationCodeNotFound = authz.ErrAuthorizationCodeNotFound

// Constructor aliases so the relocated white-box tests can keep using the original
// (now exported) constructor names unchanged.
var (
	newAuthorizationCodeStore    = NewAuthorizationCodeStore
	newAuthorizationRequestStore = NewAuthorizationRequestStore
)
