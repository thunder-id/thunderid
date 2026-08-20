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

package channel

import "errors"

// ErrDataPlaneNotConnected is returned by CallMethod when no active connection exists for the
// target Data Plane id.
var ErrDataPlaneNotConnected = errors.New("data plane not connected")

// errUnauthorized is returned by the handshake verifier when the bearer token is absent or wrong.
var errUnauthorized = errors.New("unauthorized channel handshake")

// errAuthNotConfigured is returned by the handshake verifier when no shared token is configured, so
// the server refuses all connections (secure by default).
var errAuthNotConfigured = errors.New("channel auth token not configured")

// errMissingDataPlaneID is returned when the handshake request omits the Data Plane id header.
var errMissingDataPlaneID = errors.New("missing data plane id header")
