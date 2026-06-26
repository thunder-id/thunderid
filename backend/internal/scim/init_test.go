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

package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
)

func TestInitialize_RegistersDiscoveryRoutes(t *testing.T) {
	mux := http.NewServeMux()
	Initialize(mux, nil, nil, "https://example.com", config.SCIMConfig{})
	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/scim/v2/ServiceProviderConfig"},
		{method: http.MethodGet, path: "/scim/v2/Schemas"},
		{method: http.MethodGet, path: "/scim/v2/ResourceTypes"},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Fatalf("expected route to be registered for %s %s", tc.method, tc.path)
		}
	}
}
