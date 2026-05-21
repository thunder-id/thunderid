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

package enginebridge

import (
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauthauthz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

// RuntimeStores holds internal runtime persistence adapters derived from a host RuntimeStore.
type RuntimeStores struct {
	PAR         par.StoreInterface
	AuthCode    oauthauthz.AuthorizationCodeStoreInterface
	AuthRequest oauthauthz.RequestStoreInterface
	FlowContext flowexec.ContextStoreInterface
}

// NewHostRuntimeStores builds internal store implementations backed by a host RuntimeStore.
func NewHostRuntimeStores(host thunderidengine.RuntimeStore) RuntimeStores {
	if host == nil {
		return RuntimeStores{}
	}
	return RuntimeStores{
		PAR:         par.NewStoreFromRuntime(host),
		AuthCode:    oauthauthz.NewCodeStoreFromRuntime(host),
		AuthRequest: oauthauthz.NewRequestStoreFromRuntime(host),
		FlowContext: flowexec.NewContextStoreFromRuntime(host),
	}
}

// NewDefaultRuntimeStore returns a RuntimeStore backed by Thunder's default SQL/Redis runtime stores.
func NewDefaultRuntimeStore() (thunderidengine.RuntimeStore, error) {
	return newDefaultRuntimeStore()
}
