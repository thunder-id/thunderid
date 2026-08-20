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

package declarativeresource

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// withServer installs a server configuration for the duration of a test.
func withServer(t *testing.T, source, systemID string) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server = engineconfig.ServerConfig{DeploymentIDSource: source, SystemDeploymentID: systemID}
	if err := config.InitializeServerRuntime(t.TempDir(), cfg); err != nil {
		t.Fatalf("initialize server runtime: %v", err)
	}
	t.Cleanup(config.ResetServerRuntime)
}

// A single-tenant server has one deployment to belong to, so declarative resources stay visible and
// nothing about it changes.
func TestVisibleTo_SingleTenantSeesEverything(t *testing.T) {
	withServer(t, "", "root")
	if !VisibleTo(context.Background()) {
		t.Fatal("expected declarative resources to be visible on a single-tenant server")
	}
	if !VisibleTo(deployment.WithID(context.Background(), "acme:dev")) {
		t.Fatal("expected them visible whatever deployment is named, when the server is not multi-tenant")
	}
}

// On a multi-tenant server a declarative resource carries no deployment of its own, so it belongs to
// the system tenant alone. Any other tenant must not see it.
func TestVisibleTo_MultiTenantConfinesThemToTheSystemTenant(t *testing.T) {
	withServer(t, "token", "root")

	if !VisibleTo(deployment.WithID(context.Background(), "root")) {
		t.Fatal("expected the system tenant to see its own declarative resources")
	}
	if VisibleTo(deployment.WithID(context.Background(), "acme:dev")) {
		t.Fatal("expected another tenant not to see the system tenant's declarative resources")
	}
	// Loading names no deployment: it happens before any request, and the files reference each
	// other, so it must see them all.
	if !VisibleTo(context.Background()) {
		t.Fatal("expected loading, which names no deployment, to see every declarative resource")
	}
}
