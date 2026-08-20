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

package deployment

import (
	"context"
	"testing"
)

func TestResolve_FallsBackToConfiguredWhenNoClaim(t *testing.T) {
	got := Resolve(context.Background(), "default-deployment")
	if got != "default-deployment" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestResolve_UsesContextDeploymentIDWhenPresent(t *testing.T) {
	ctx := WithID(context.Background(), "tenant-abc:dev")
	if got := Resolve(ctx, "default-deployment"); got != "tenant-abc:dev" {
		t.Fatalf("expected tenant id from context, got %q", got)
	}
}

func TestWithID_EmptyIsIgnored(t *testing.T) {
	ctx := WithID(context.Background(), "")
	if _, ok := IDFromContext(ctx); ok {
		t.Fatal("empty deployment id should not be stored")
	}
	if got := Resolve(ctx, "cfg"); got != "cfg" {
		t.Fatalf("expected fallback for empty id, got %q", got)
	}
}

func TestIDFromContext_ReportsPresence(t *testing.T) {
	if _, ok := IDFromContext(context.Background()); ok {
		t.Fatal("no id should be present on a bare context")
	}
	ctx := WithID(context.Background(), "t1")
	if id, ok := IDFromContext(ctx); !ok || id != "t1" {
		t.Fatalf("expected (t1,true), got (%q,%v)", id, ok)
	}
}
