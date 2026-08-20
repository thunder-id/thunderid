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

package core

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// ExecutorMetadataProvider is the read-only, runtime-free view of the executor registry that flow
// definition validation and graph building require. It exposes only the static metadata surface
// (registration check + executor metadata) and never a live executor instance, so consumers that
// depend on it do not link the runtime executor constructors. The full executor registry used by
// the flow execution engine satisfies this interface structurally.
type ExecutorMetadataProvider interface {
	IsRegistered(name string) bool
	GetExecutorMeta(name string) (*providers.ExecutorMeta, error)
}
