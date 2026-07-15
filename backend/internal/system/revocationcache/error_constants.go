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

import "errors"

// errTokenRevoked is returned by EnforcerInterface.EnsureNotRevoked when the referenced token has a
// non-VALID status in its Token Status List.
var errTokenRevoked = errors.New("token has been revoked")

// errStatusUnavailable is returned when a token's revocation status cannot be resolved (its status
// list has never been cached and cannot be fetched). The enforcer fails closed on it: an unknown
// status must not let a possibly-revoked token through.
var errStatusUnavailable = errors.New("token revocation status is unavailable")
