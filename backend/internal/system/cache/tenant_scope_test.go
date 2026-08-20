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

package cache

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// TestTenantScopedCache_IsolatesByDeploymentID verifies the same cache key resolves to per-tenant
// values when the request context carries different deployment ids (token mode).
func TestTenantScopedCache_IsolatesByDeploymentID(t *testing.T) {
	mgr := Initialize(engineconfig.CacheConfig{Size: 100, TTL: 60, EvictionPolicy: "LRU"}, "test")
	c := GetCache[string](mgr, "tenantIsolationCache")

	ctxA := deployment.WithID(context.Background(), "tenant-a")
	ctxB := deployment.WithID(context.Background(), "tenant-b")
	key := CacheKey{Key: "shared-id"}

	if err := c.Set(ctxA, key, "value-a"); err != nil {
		t.Fatalf("set A: %v", err)
	}
	if err := c.Set(ctxB, key, "value-b"); err != nil {
		t.Fatalf("set B: %v", err)
	}

	if v, ok := c.Get(ctxA, key); !ok || v != "value-a" {
		t.Fatalf("tenant A got (%q,%v), want value-a", v, ok)
	}
	if v, ok := c.Get(ctxB, key); !ok || v != "value-b" {
		t.Fatalf("tenant B got (%q,%v), want value-b (cross-tenant bleed)", v, ok)
	}

	// A tenant only sees its own entry; deleting one leaves the other intact.
	if err := c.Delete(ctxA, key); err != nil {
		t.Fatalf("delete A: %v", err)
	}
	if _, ok := c.Get(ctxA, key); ok {
		t.Fatal("tenant A entry should be gone after delete")
	}
	if v, ok := c.Get(ctxB, key); !ok || v != "value-b" {
		t.Fatalf("tenant B entry should survive tenant A delete, got (%q,%v)", v, ok)
	}
}
