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

package envmgr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/deployment"
)

func TestRegistryGivesEachDeploymentItsOwnManager(t *testing.T) {
	reg := newRegistry(nil)

	a, err := reg.serverFor(deployment.WithID(context.Background(), "tenant-a"))
	if err != nil {
		t.Fatalf("tenant-a: %v", err)
	}
	b, err := reg.serverFor(deployment.WithID(context.Background(), "tenant-b"))
	if err != nil {
		t.Fatalf("tenant-b: %v", err)
	}
	if a == b {
		t.Fatal("two deployments must not share one environment manager")
	}

	// The same deployment keeps its manager, so its store is opened once rather than per request.
	again, err := reg.serverFor(deployment.WithID(context.Background(), "tenant-a"))
	if err != nil {
		t.Fatalf("tenant-a again: %v", err)
	}
	if again != a {
		t.Fatal("a deployment's manager must be reused")
	}
}

func TestRegistryRefusesARequestWithNoDeployment(t *testing.T) {
	reg := newRegistry(nil)

	// Serving this from a default store would hand one deployment another's environments.
	if _, err := reg.serverFor(context.Background()); err == nil {
		t.Fatal("a request with no deployment must be refused")
	}
}

func TestInitializeRegistersTheSecretRoutes(t *testing.T) {
	mux := http.NewServeMux()
	if _, err := Initialize(mux, nil); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// A pattern that does not resolve here would 404 at runtime rather than at startup.
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/environments/env-1/secrets"},
		{http.MethodPut, "/environments/env-1/secrets/APP_A_CLIENT_SECRET"},
		{http.MethodPost, "/environments/env-1/secrets/APP_A_CLIENT_SECRET/regenerate"},
	} {
		req := httptest.NewRequest(route.method, route.path, nil)
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Fatalf("%s %s is not routed", route.method, route.path)
		}
	}
}
