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

package dataplanetoken

import (
	"context"
	"strings"
	"testing"
)

// reversingCrypto stands in for the configuration crypto service: it transforms the bytes, so a test
// can tell ciphertext from plaintext without a key manager.
func reversingCrypto(_ context.Context, content []byte) ([]byte, error) {
	out := make([]byte, len(content))
	for i, b := range content {
		out[len(content)-1-i] = b
	}
	return out, nil
}

// memoryStore records what the service persists.
type memoryStore struct {
	rows map[string]string
}

func newTestService() (*Service, *memoryStore) {
	mem := &memoryStore{rows: map[string]string{}}
	svc := &Service{encrypt: reversingCrypto, decrypt: reversingCrypto}
	svc.putFn = func(_ context.Context, dataPlaneID, _, ciphertext string) error {
		mem.rows[dataPlaneID] = ciphertext
		return nil
	}
	svc.getFn = func(_ context.Context, dataPlaneID string) (string, bool, error) {
		v, ok := mem.rows[dataPlaneID]
		return v, ok, nil
	}
	return svc, mem
}

// A token is readable exactly once, when it is issued. What is kept is ciphertext, so a database dump
// alone does not yield a working set of data plane credentials.
func TestIssueReturnsTheTokenAndStoresOnlyCiphertext(t *testing.T) {
	svc, mem := newTestService()

	token, err := svc.Issue(context.Background(), "org1:dev", "org1:dev")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if token == "" {
		t.Fatal("expected a token")
	}
	stored := mem.rows["org1:dev"]
	if stored == "" {
		t.Fatal("expected the token to be recorded")
	}
	if strings.Contains(stored, token) {
		t.Fatalf("the token must not be stored in readable form, got %q", stored)
	}
}

// The stored form has to come back as what the data plane presents, or no handshake could ever match.
func TestTokenForRecoversTheIssuedToken(t *testing.T) {
	svc, _ := newTestService()
	issued, err := svc.Issue(context.Background(), "org1:dev", "org1:dev")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, found, err := svc.TokenFor(context.Background(), "org1:dev")

	if err != nil || !found {
		t.Fatalf("expected the token to be found, got %v %v", found, err)
	}
	if got != issued {
		t.Fatalf("expected %q, got %q", issued, got)
	}
}

// Rotation must invalidate what came before. A previous token that still worked would not have been
// rotated.
func TestIssueReplacesThePreviousToken(t *testing.T) {
	svc, _ := newTestService()
	first, _ := svc.Issue(context.Background(), "org1:dev", "org1:dev")

	second, err := svc.Issue(context.Background(), "org1:dev", "org1:dev")
	if err != nil {
		t.Fatalf("reissue: %v", err)
	}
	if first == second {
		t.Fatal("a reissued token must differ from the one it replaces")
	}

	current, _, _ := svc.TokenFor(context.Background(), "org1:dev")
	if current != second {
		t.Fatalf("the newest token should be the one held, got %q", current)
	}
}

func TestTokenForReportsADataPlaneWithNoToken(t *testing.T) {
	svc, _ := newTestService()

	_, found, err := svc.TokenFor(context.Background(), "org1:dev")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected no token to be found")
	}
}

func TestIssueRequiresADataPlaneID(t *testing.T) {
	svc, _ := newTestService()

	if _, err := svc.Issue(context.Background(), "  ", "org1:dev"); err == nil {
		t.Fatal("expected an unnamed data plane to be refused")
	}
}
