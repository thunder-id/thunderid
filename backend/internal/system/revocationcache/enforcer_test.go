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
	"time"
)

func TestEnforcerEmptyURIIsNoop(t *testing.T) {
	src := &fakeSource{statuses: map[int64]int{7: 1}}
	e := newEnforcer(newStatusCache(src, time.Hour))

	if err := e.EnsureNotRevoked(context.Background(), "", 7); err != nil {
		t.Fatalf("EnsureNotRevoked(empty uri) = %v, want nil", err)
	}
	if src.calls != 0 {
		t.Fatalf("source Fetch called %d times for empty uri, want 0", src.calls)
	}
}

func TestEnforcerAllowsValidToken(t *testing.T) {
	e := newEnforcer(newStatusCache(
		&fakeSource{statuses: map[int64]int{7: statusValid}, capacity: 100, found: true}, time.Hour))

	if err := e.EnsureNotRevoked(context.Background(), "uri", 7); err != nil {
		t.Fatalf("EnsureNotRevoked(valid) = %v, want nil", err)
	}
}

func TestEnforcerRejectsRevokedToken(t *testing.T) {
	e := newEnforcer(newStatusCache(
		&fakeSource{statuses: map[int64]int{7: 1}, capacity: 100, found: true}, time.Hour))

	if err := e.EnsureNotRevoked(context.Background(), "uri", 7); !errors.Is(err, errTokenRevoked) {
		t.Fatalf("EnsureNotRevoked(revoked) = %v, want errTokenRevoked", err)
	}
}

func TestEnforcerFailsClosedWhenStatusUnavailable(t *testing.T) {
	e := newEnforcer(newStatusCache(&fakeSource{err: errors.New("db down")}, time.Hour))

	if err := e.EnsureNotRevoked(context.Background(), "uri", 7); !errors.Is(err, errStatusUnavailable) {
		t.Fatalf("EnsureNotRevoked(unavailable) = %v, want errStatusUnavailable (fail closed)", err)
	}
}

func TestNoopEnforcerNeverRejects(t *testing.T) {
	if err := (noopEnforcer{}).EnsureNotRevoked(context.Background(), "uri", 7); err != nil {
		t.Fatalf("noopEnforcer = %v, want nil", err)
	}
}
