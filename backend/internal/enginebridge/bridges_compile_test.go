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
	"testing"

	"github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func TestBridgesSatisfyInternalInterfaces(t *testing.T) {
	var (
		_ inboundclient.InboundClientServiceInterface = (*clientBridge)(nil)
		_ manager.AuthnProviderManagerInterface       = (*authnBridge)(nil)
		_ authz.AuthorizationServiceInterface         = (*authzBridge)(nil)
		_ resource.ResourceServiceInterface           = (*resourceBridge)(nil)
		_ ou.OrganizationUnitServiceInterface         = (*ouBridge)(nil)
		_ idp.IDPServiceInterface                     = (*idpBridge)(nil)
		_ observability.ObservabilityServiceInterface = (*observabilityBridge)(nil)
		_ executor.ExecutorRegistryInterface          = (*executorRegistryBridge)(nil)
	)

	_ = thunderidengine.ErrNotImplemented
}
