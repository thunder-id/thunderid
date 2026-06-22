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

// Package runtime holds the small, dependency-free contract types that an embedding
// application's host providers (see package host) interact with. It is part of the public
// thunderidengine SDK surface and intentionally imports nothing from internal/*, so any
// external Go application can depend on it.
package runtime

import "errors"

// ErrNotFound is the sentinel a host provider returns when a requested entity, client,
// application, or other record does not exist. The engine treats it as a normal "absent"
// result (for example, an unknown client during OAuth processing) rather than an internal
// error. Host implementations should return ErrNotFound (or an error wrapping it) instead of
// inventing their own not-found errors.
var ErrNotFound = errors.New("thunderidengine: not found")
