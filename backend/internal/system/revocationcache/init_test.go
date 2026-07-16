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

package revocationcache

import (
	"context"
	"errors"
	"testing"
)

func TestInitializeDisabledReturnsNoop(t *testing.T) {
	enforcer := Initialize(Config{Enabled: false}, &fakeSource{statuses: map[int64]int{7: 1}})
	// A disabled enforcer never rejects, even for a revoked index.
	if err := enforcer.EnsureNotRevoked(context.Background(), "uri", 7); err != nil {
		t.Fatalf("disabled enforcer = %v, want nil", err)
	}
}

func TestInitializeWithoutSourceReturnsNoop(t *testing.T) {
	// With no status list source (the list feature is off) there is nothing to enforce, so an enabled
	// config must still yield a no-op enforcer rather than reject tokens.
	enforcer := Initialize(Config{Enabled: true}, nil)
	if err := enforcer.EnsureNotRevoked(context.Background(), "uri", 7); err != nil {
		t.Fatalf("sourceless enforcer = %v, want nil", err)
	}
}

func TestInitializeEnabledBuildsEnforcer(t *testing.T) {
	enforcer := Initialize(Config{Enabled: true},
		&fakeSource{statuses: map[int64]int{7: 1}, capacity: 100, found: true})
	// A revoked index is now rejected, confirming a real enforcer was built.
	if err := enforcer.EnsureNotRevoked(context.Background(), "uri", 7); !errors.Is(err, errTokenRevoked) {
		t.Fatalf("enabled enforcer = %v, want errTokenRevoked", err)
	}
}
