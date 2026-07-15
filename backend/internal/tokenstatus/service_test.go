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

package tokenstatus

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// testListURI is the public URI of the "abc" status list used across these tests.
const testListURI = "https://issuer.example/statuslists/abc"

// craftStatusListToken builds an unsigned JWT (header.payload.sig) carrying the given claims, for
// exercising the decode path without a real signer. DecodeStatusListToken reads the payload only.
func craftStatusListToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"statuslist+jwt"}`))
	body, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(body) + ".sig"
}

// fakeStore is a hand-written statusStoreInterface for exercising the service without a database.
type fakeStore struct {
	allocListID string
	allocIdx    int64
	allocErr    error

	gotListID string
	gotIdx    int64
	gotStatus byte
	setErr    error

	getStatusVal byte
	getErr       error

	list       listRecord
	listFound  bool
	getListErr error
	entries    []entryRecord
	entriesErr error
}

func (f *fakeStore) allocateIndex(context.Context) (string, int64, error) {
	return f.allocListID, f.allocIdx, f.allocErr
}

func (f *fakeStore) setStatus(_ context.Context, listID string, idx int64, status byte, _ time.Time) error {
	f.gotListID, f.gotIdx, f.gotStatus = listID, idx, status
	return f.setErr
}

func (f *fakeStore) getStatus(context.Context, string, int64) (byte, error) {
	return f.getStatusVal, f.getErr
}

func (f *fakeStore) getList(context.Context, string) (listRecord, bool, error) {
	return f.list, f.listFound, f.getListErr
}
func (f *fakeStore) listEntries(context.Context, string) ([]entryRecord, error) {
	return f.entries, f.entriesErr
}
func (f *fakeStore) dropExpiredSealedLists(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestRetentionFor(t *testing.T) {
	if got := retentionFor(0); got != 0 {
		t.Fatalf("retentionFor(0) = %v, want 0 (reaping disabled)", got)
	}
	if got := retentionFor(-time.Hour); got != 0 {
		t.Fatalf("retentionFor(negative) = %v, want 0 (reaping disabled)", got)
	}
	maxTTL := 24 * time.Hour
	if got := retentionFor(maxTTL); got != maxTTL+retentionGrace {
		t.Fatalf("retentionFor(%v) = %v, want %v", maxTTL, got, maxTTL+retentionGrace)
	}
}

func TestIssueReference(t *testing.T) {
	store := &fakeStore{allocListID: "abc", allocIdx: 42}
	svc := &service{store: store, baseURL: "https://issuer.example/"}

	idx, uri, err := svc.IssueReference(context.Background())

	if err != nil {
		t.Fatalf("IssueReference: %v", err)
	}
	if idx != 42 {
		t.Fatalf("idx = %d, want 42", idx)
	}
	// Trailing slash on the base URL must not double up in the URI.
	if want := testListURI; uri != want {
		t.Fatalf("uri = %q, want %q", uri, want)
	}
}

func TestIssueReferencePropagatesError(t *testing.T) {
	store := &fakeStore{allocErr: errors.New("db down")}
	svc := &service{store: store, baseURL: "https://issuer.example"}

	if _, _, err := svc.IssueReference(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSetStatusResolvesListID(t *testing.T) {
	store := &fakeStore{}
	svc := &service{store: store, baseURL: "https://issuer.example"}

	err := svc.SetStatus(context.Background(),
		testListURI, 7, int(statusInvalid), time.Now())

	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if store.gotListID != "abc" || store.gotIdx != 7 || store.gotStatus != statusInvalid {
		t.Fatalf("store got (%q, %d, %d), want (abc, 7, %d)",
			store.gotListID, store.gotIdx, store.gotStatus, statusInvalid)
	}
}

func TestGetStatusResolvesListID(t *testing.T) {
	store := &fakeStore{list: listRecord{capacity: 100}, listFound: true, getStatusVal: statusInvalid}
	svc := &service{store: store, baseURL: "https://issuer.example"}

	status, err := svc.GetStatus(context.Background(), testListURI, 7)

	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status != int(statusInvalid) {
		t.Fatalf("status = %d, want %d", status, statusInvalid)
	}
}

func TestGetStatusRejectsUnknownList(t *testing.T) {
	svc := &service{store: &fakeStore{listFound: false}, baseURL: "https://issuer.example"}

	uri := testListURI
	if _, err := svc.GetStatus(context.Background(), uri, 7); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("GetStatus(unknown list) error = %v, want ErrListNotFound", err)
	}
}

func TestGetStatusRejectsOutOfRangeIndex(t *testing.T) {
	store := &fakeStore{list: listRecord{capacity: 100}, listFound: true}
	svc := &service{store: store, baseURL: "https://issuer.example"}

	uri := testListURI
	if _, err := svc.GetStatus(context.Background(), uri, 100); !errors.Is(err, errIndexOutOfRange) {
		t.Fatalf("GetStatus(out of range) error = %v, want errIndexOutOfRange", err)
	}
	if _, err := svc.GetStatus(context.Background(), uri, -1); !errors.Is(err, errIndexOutOfRange) {
		t.Fatalf("GetStatus(negative) error = %v, want errIndexOutOfRange", err)
	}
}

func TestDecodeStatusListToken(t *testing.T) {
	// A size of 10 bits pads up to a 2-byte array, so the decoded capacity is the byte-padded 16, not 10.
	list, err := newStatusList(10, 1)
	if err != nil {
		t.Fatalf("newStatusList: %v", err)
	}
	_ = list.set(3, statusInvalid)
	lst, err := list.encodeLst()
	if err != nil {
		t.Fatalf("encodeLst: %v", err)
	}
	token := craftStatusListToken(t, map[string]interface{}{
		claimStatusList: map[string]interface{}{claimBits: 1, claimLst: lst},
	})

	statuses, capacity, err := DecodeStatusListToken(token)
	if err != nil {
		t.Fatalf("DecodeStatusListToken: %v", err)
	}
	if capacity != 16 {
		t.Fatalf("capacity = %d, want 16 (byte-padded from 10)", capacity)
	}
	if len(statuses) != 1 || statuses[3] != int(statusInvalid) {
		t.Fatalf("statuses = %v, want {3: invalid}", statuses)
	}
}

func TestDecodeStatusListTokenRejectsMalformed(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]interface{}
	}{
		{"no status_list claim", map[string]interface{}{"foo": "bar"}},
		{
			"lst not a string",
			map[string]interface{}{claimStatusList: map[string]interface{}{claimBits: 1, claimLst: 7}},
		},
		{"bits missing", map[string]interface{}{claimStatusList: map[string]interface{}{claimLst: "AAAA"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := craftStatusListToken(t, tt.claims)
			if _, _, err := DecodeStatusListToken(token); !errors.Is(err, errMalformedStatusListToken) {
				t.Fatalf("DecodeStatusListToken error = %v, want errMalformedStatusListToken", err)
			}
		})
	}
}

func TestSetAndGetStatusRejectInvalidURI(t *testing.T) {
	svc := &service{store: &fakeStore{}, baseURL: "https://issuer.example"}

	uri := "https://issuer.example/wrong/abc"
	if err := svc.SetStatus(context.Background(), uri, 1, 1, time.Now()); !errors.Is(err, errInvalidListURI) {
		t.Fatalf("SetStatus error = %v, want errInvalidListURI", err)
	}
	if _, err := svc.GetStatus(context.Background(), uri, 1); !errors.Is(err, errInvalidListURI) {
		t.Fatalf("GetStatus error = %v, want errInvalidListURI", err)
	}
}

func TestListIDFromURI(t *testing.T) {
	tests := []struct {
		uri     string
		want    string
		wantErr bool
	}{
		{testListURI, "abc", false},
		{"https://issuer.example/statuslists/01900000-uuid", "01900000-uuid", false},
		{"https://issuer.example/statuslists/", "", true},
		{"https://issuer.example/statuslists/abc/extra", "", true},
		{"https://issuer.example/statuslists/abc?time=1", "", true},
		{"https://issuer.example/other/abc", "", true},
	}
	for _, tt := range tests {
		got, err := ListIDFromURI(tt.uri)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ListIDFromURI(%q) = %q, want error", tt.uri, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ListIDFromURI(%q): %v", tt.uri, err)
		}
		if got != tt.want {
			t.Fatalf("ListIDFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestProduce(t *testing.T) {
	store := &fakeStore{
		list:      listRecord{id: "abc", bits: 1, capacity: 100},
		listFound: true,
		entries:   []entryRecord{{idx: 3, status: statusInvalid}, {idx: 42, status: statusInvalid}},
	}
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	svc := &service{store: store, jwtService: jwtService, baseURL: "https://issuer.example", ttlSeconds: 3600}

	jwtService.EXPECT().GenerateJWT(
		mock.Anything, testListURI, "", int64(3600),
		mock.Anything, jwt.TokenTypeStatusList, "").
		Return("signed.status.list", int64(0), nil).Once()

	token, ttl, err := svc.Produce(context.Background(), "abc")

	if err != nil {
		t.Fatalf("Produce: %v", err)
	}
	if token != "signed.status.list" {
		t.Fatalf("token = %q", token)
	}
	if ttl != 3600 {
		t.Fatalf("ttl = %d, want 3600", ttl)
	}
}

func TestProduceListNotFound(t *testing.T) {
	svc := &service{store: &fakeStore{listFound: false}, baseURL: "https://issuer.example"}

	if _, _, err := svc.Produce(context.Background(), "missing"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("Produce error = %v, want ErrListNotFound", err)
	}
}

func TestProducePropagatesGetListError(t *testing.T) {
	svc := &service{store: &fakeStore{getListErr: errors.New("db down")}, baseURL: "https://issuer.example"}
	if _, _, err := svc.Produce(context.Background(), "abc"); err == nil {
		t.Fatal("expected error from getList")
	}
}

func TestProducePropagatesListEntriesError(t *testing.T) {
	store := &fakeStore{list: listRecord{bits: 1, capacity: 100}, listFound: true, entriesErr: errors.New("db down")}
	svc := &service{store: store, baseURL: "https://issuer.example"}
	if _, _, err := svc.Produce(context.Background(), "abc"); err == nil {
		t.Fatal("expected error from listEntries")
	}
}

func TestProduceRejectsInvalidBits(t *testing.T) {
	// A stored bit width the packer does not support surfaces as an error instead of a bad token.
	store := &fakeStore{list: listRecord{bits: 3, capacity: 100}, listFound: true}
	svc := &service{store: store, baseURL: "https://issuer.example"}
	if _, _, err := svc.Produce(context.Background(), "abc"); err == nil {
		t.Fatal("expected error for unsupported bit width")
	}
}

func TestProduceRejectsOutOfRangeEntry(t *testing.T) {
	// An entry index beyond the list capacity cannot be packed and must surface as an error.
	store := &fakeStore{
		list:      listRecord{bits: 1, capacity: 8},
		listFound: true,
		entries:   []entryRecord{{idx: 999, status: statusInvalid}},
	}
	svc := &service{store: store, baseURL: "https://issuer.example"}
	if _, _, err := svc.Produce(context.Background(), "abc"); err == nil {
		t.Fatal("expected error for out-of-range entry index")
	}
}

func TestProducePropagatesSignError(t *testing.T) {
	store := &fakeStore{list: listRecord{bits: 1, capacity: 100}, listFound: true}
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	svc := &service{store: store, jwtService: jwtService, baseURL: "https://issuer.example", ttlSeconds: 3600}

	jwtService.EXPECT().GenerateJWT(
		mock.Anything, testListURI, "", int64(3600), mock.Anything, jwt.TokenTypeStatusList, "").
		Return("", int64(0), &tidcommon.ServiceError{}).Once()

	if _, _, err := svc.Produce(context.Background(), "abc"); err == nil {
		t.Fatal("expected error when signing fails")
	}
}

func TestGetStatusPropagatesGetListError(t *testing.T) {
	svc := &service{store: &fakeStore{getListErr: errors.New("db down")}, baseURL: "https://issuer.example"}
	if _, err := svc.GetStatus(context.Background(), testListURI, 7); err == nil {
		t.Fatal("expected error from getList")
	}
}

func TestGetStatusPropagatesReadError(t *testing.T) {
	store := &fakeStore{list: listRecord{capacity: 100}, listFound: true, getErr: errors.New("db down")}
	svc := &service{store: store, baseURL: "https://issuer.example"}
	if _, err := svc.GetStatus(context.Background(), testListURI, 7); err == nil {
		t.Fatal("expected error from getStatus")
	}
}

func TestInitializeRejectsEmptyBaseURL(t *testing.T) {
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	if _, err := Initialize(Config{Enabled: true}, jwtService); !errors.Is(err, errEmptyBaseURL) {
		t.Fatalf("Initialize error = %v, want errEmptyBaseURL", err)
	}
}

func TestInitializeRejectsNilJWTService(t *testing.T) {
	_, err := Initialize(Config{Enabled: true, BaseURL: "https://issuer.example"}, nil)
	if !errors.Is(err, errNilJWTService) {
		t.Fatalf("Initialize error = %v, want errNilJWTService", err)
	}
}

func TestInitializeRejectsNonPositiveTTL(t *testing.T) {
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	if _, err := Initialize(
		Config{Enabled: true, BaseURL: "https://issuer.example"}, jwtService); !errors.Is(err, errNonPositiveTTL) {
		t.Fatalf("Initialize error = %v, want errNonPositiveTTL", err)
	}
}

func TestInitializeBuildsService(t *testing.T) {
	// Initialize builds the real store, which reads the deployment id from the server runtime.
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: "test-deployment"},
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)

	svc, err := Initialize(
		Config{Enabled: true, BaseURL: "https://issuer.example", ListSize: 1000, Bits: 1, TTL: time.Hour},
		jwtService)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if svc == nil {
		t.Fatal("Initialize returned nil service")
	}
}
