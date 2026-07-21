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

// errTokenRevoked is returned by EnforcerInterface.EnsureNotRevoked when the token identifier is
// present in the cached deny list.
var errTokenRevoked = errors.New("token has been revoked")

// errUnsupportedSource is returned by Initialize when cfg.Source names a sync source that is not
// supported.
var errUnsupportedSource = errors.New("unsupported revocation sync source")
