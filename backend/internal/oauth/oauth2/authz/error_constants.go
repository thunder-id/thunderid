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

package authz

import "errors"

// errAuthorizationCodeNotFound is returned when an authorization code is not found in the database.
var errAuthorizationCodeNotFound = errors.New("authorization code not found")

// errAuthorizationCodeAlreadyConsumed is returned when an authorization code has already been consumed,
// indicating a potential replay attack.
var errAuthorizationCodeAlreadyConsumed = errors.New("authorization code already consumed")

// errAuthRequestNotFound is returned when an authorization request context is not found in the store.
var errAuthRequestNotFound = errors.New("authorization request context not found")

// errAssertionClaimInvalid is returned when a claim in the flow assertion has an unexpected shape
// (e.g. wrong JSON type). It distinguishes client-facing input errors from genuine internal decode failures.
var errAssertionClaimInvalid = errors.New("assertion claim is invalid")
