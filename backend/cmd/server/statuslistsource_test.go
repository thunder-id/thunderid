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

package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/tokenstatus"
)

type fakeProducer struct {
	token string
	err   error
}

func (f fakeProducer) Produce(context.Context, string) (string, int, error) {
	return f.token, 0, f.err
}

// craftStatusListToken builds an unsigned Status List Token carrying the given packed bit array, using
// only the spec-fixed wire keys and stdlib compression, so the source can be exercised without the
// Status List subsystem's signer or its unexported codec.
func craftStatusListToken(t *testing.T, data []byte, bits int) string {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("zlib write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zlib close: %v", err)
	}
	lst := base64.RawURLEncoding.EncodeToString(buf.Bytes())
	payload, err := json.Marshal(map[string]interface{}{
		"status_list": map[string]interface{}{"bits": bits, "lst": lst},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"statuslist+jwt"}`))
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestFetchRejectsInvalidURI(t *testing.T) {
	src := newStatusListSource(fakeProducer{})
	if _, _, _, err := src.Fetch(context.Background(), "https://issuer.example/wrong/abc"); err == nil {
		t.Fatal("expected error for a URI with no resolvable list id")
	}
}

func TestFetchListNotFound(t *testing.T) {
	src := newStatusListSource(fakeProducer{err: tokenstatus.ErrListNotFound})

	statuses, capacity, found, err := src.Fetch(context.Background(), "https://issuer.example/statuslists/abc")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if found || statuses != nil || capacity != 0 {
		t.Fatalf("found=%v statuses=%v capacity=%d, want false nil 0", found, statuses, capacity)
	}
}

func TestFetchPropagatesProducerError(t *testing.T) {
	src := newStatusListSource(fakeProducer{err: errors.New("db down")})

	_, _, found, err := src.Fetch(context.Background(), "https://issuer.example/statuslists/abc")
	if err == nil || found {
		t.Fatalf("Fetch found=%v err=%v, want found=false and an error", found, err)
	}
}

func TestFetchDecodesToken(t *testing.T) {
	// One byte with bit index 3 set (LSB-first, 1 bit per entry): 0b00001000 = 0x08.
	src := newStatusListSource(fakeProducer{token: craftStatusListToken(t, []byte{0x08}, 1)})

	statuses, capacity, found, err := src.Fetch(context.Background(), "https://issuer.example/statuslists/abc")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !found || capacity != 8 {
		t.Fatalf("found=%v capacity=%d, want true 8", found, capacity)
	}
	if len(statuses) != 1 || statuses[3] != int(1) {
		t.Fatalf("statuses = %v, want {3: 1}", statuses)
	}
}

func TestFetchRejectsMalformedToken(t *testing.T) {
	src := newStatusListSource(fakeProducer{token: "not-a-jwt"})

	if _, _, _, err := src.Fetch(context.Background(), "https://issuer.example/statuslists/abc"); err == nil {
		t.Fatal("expected error for a malformed token")
	}
}
