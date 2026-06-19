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

package integration

import "context"

// ServiceInterface exposes the integration catalog.
type ServiceInterface interface {
	GetIntegrations(ctx context.Context) ListResponse
}

// service serves a static set of integration descriptors.
type service struct {
	integrations []Descriptor
}

// newService creates an integration service over the given descriptors.
func newService(integrations []Descriptor) ServiceInterface {
	return &service{integrations: integrations}
}

// GetIntegrations returns the full integration catalog.
func (s *service) GetIntegrations(_ context.Context) ListResponse {
	return ListResponse{Integrations: s.integrations}
}
