/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package security

import "errors"

var (
	// errUnauthorized indicates that the request lacks valid authentication credentials.
	errUnauthorized = errors.New("unauthorized")

	// errForbidden indicates that the authenticated user lacks sufficient permissions.
	errForbidden = errors.New("forbidden")

	// errInsufficientPermissions indicates that the user's permissions are insufficient for the requested resource.
	errInsufficientPermissions = errors.New("insufficient permissions")

	// errNoHandlerFound indicates that no security handler could process the request.
	errNoHandlerFound = errors.New("no security handler found")

	// errInvalidToken indicates that the provided authentication token is invalid.
	errInvalidToken = errors.New("invalid token")

	// errMissingAuthHeader indicates that the Authorization header is missing.
	errMissingAuthHeader = errors.New("missing authorization header")
)
