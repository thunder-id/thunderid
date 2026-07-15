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

package tokenservice

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
)

type fakeStatusReader struct {
	status int
	err    error
}

func (f fakeStatusReader) GetStatus(context.Context, string, int64) (int, error) {
	return f.status, f.err
}

func claimsWithStatusRef() map[string]interface{} {
	return map[string]interface{}{
		"status": map[string]interface{}{
			"status_list": map[string]interface{}{
				"idx": float64(42),
				"uri": "https://issuer.example/statuslists/abc",
			},
		},
	}
}

func TestEnsureNotRevokedStatusListRevoked(t *testing.T) {
	tv := &tokenValidator{statusReader: fakeStatusReader{status: 1}}

	err := tv.ensureNotRevoked(context.Background(), claimsWithStatusRef())
	if !errors.Is(err, revocation.ErrTokenRevoked) {
		t.Fatalf("error = %v, want ErrTokenRevoked", err)
	}
}

func TestEnsureNotRevokedStatusListValid(t *testing.T) {
	tv := &tokenValidator{statusReader: fakeStatusReader{status: 0}}

	if err := tv.ensureNotRevoked(context.Background(), claimsWithStatusRef()); err != nil {
		t.Fatalf("ensureNotRevoked: %v", err)
	}
}

func TestEnsureNotRevokedReadErrorFailsClosed(t *testing.T) {
	tv := &tokenValidator{statusReader: fakeStatusReader{err: errors.New("db down")}}

	err := tv.ensureNotRevoked(context.Background(), claimsWithStatusRef())
	if !errors.Is(err, revocation.ErrEnforcementUnavailable) {
		t.Fatalf("error = %v, want ErrEnforcementUnavailable", err)
	}
}

func TestEnsureNotRevokedMalformedRefFailsClosed(t *testing.T) {
	// A token that carries a status_list object with an invalid index is a present-but-malformed
	// reference: it must fail closed rather than be treated as having no revocation channel.
	tv := &tokenValidator{statusReader: fakeStatusReader{status: 0}}
	claims := map[string]interface{}{
		"status": map[string]interface{}{
			"status_list": map[string]interface{}{
				"idx": float64(-1),
				"uri": "https://issuer.example/statuslists/abc",
			},
		},
	}

	err := tv.ensureNotRevoked(context.Background(), claims)
	if !errors.Is(err, revocation.ErrEnforcementUnavailable) {
		t.Fatalf("error = %v, want ErrEnforcementUnavailable", err)
	}
}

func TestEnsureNotRevokedNoRefAllowed(t *testing.T) {
	// The reader would report revoked, but a token without a status reference has no revocation
	// channel: enforcement must not consult the reader and must allow the token.
	tv := &tokenValidator{statusReader: fakeStatusReader{status: 1}}

	if err := tv.ensureNotRevoked(context.Background(), map[string]interface{}{}); err != nil {
		t.Fatalf("ensureNotRevoked: %v", err)
	}
}

func TestEnsureNotRevokedDisabledAllowed(t *testing.T) {
	// Nil status reader => the Token Status List feature is off; there is no revocation mechanism, so
	// even a status-referencing token is allowed.
	tv := &tokenValidator{}

	if err := tv.ensureNotRevoked(context.Background(), claimsWithStatusRef()); err != nil {
		t.Fatalf("ensureNotRevoked: %v", err)
	}
}
