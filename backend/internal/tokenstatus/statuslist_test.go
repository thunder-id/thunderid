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
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

// specVector1Bit is the 1-bit status list from draft-ietf-oauth-status-list-21 §4.1: sixteen statuses
// packing to the bytes 0xB9 0xA3 and, once compressed and base64url-encoded, to "eNrbuRgAAhcBXQ".
var specVector1Bit = []byte{1, 0, 0, 1, 1, 1, 0, 1, 1, 1, 0, 0, 0, 1, 0, 1}

// specVector2Bit is the 2-bit status list from the same section: twelve statuses packing to
// 0xC9 0x44 0xF9 and, compressed, to "eNo76fITAAPfAgc".
var specVector2Bit = []byte{1, 2, 0, 3, 0, 1, 0, 1, 1, 2, 3, 3}

func TestPack1Bit(t *testing.T) {
	l, err := newStatusList(len(specVector1Bit), 1)
	if err != nil {
		t.Fatalf("newStatusList: %v", err)
	}
	for i, v := range specVector1Bit {
		if err := l.set(i, v); err != nil {
			t.Fatalf("set(%d): %v", i, err)
		}
	}
	want := []byte{0xb9, 0xa3}
	if !bytes.Equal(l.data, want) {
		t.Fatalf("packed bytes = %x, want %x", l.data, want)
	}
}

func TestPack2Bit(t *testing.T) {
	l, err := newStatusList(len(specVector2Bit), 2)
	if err != nil {
		t.Fatalf("newStatusList: %v", err)
	}
	for i, v := range specVector2Bit {
		if err := l.set(i, v); err != nil {
			t.Fatalf("set(%d): %v", i, err)
		}
	}
	want := []byte{0xc9, 0x44, 0xf9}
	if !bytes.Equal(l.data, want) {
		t.Fatalf("packed bytes = %x, want %x", l.data, want)
	}
}

func TestGetReturnsWhatWasSet(t *testing.T) {
	l, err := newStatusList(len(specVector2Bit), 2)
	if err != nil {
		t.Fatalf("newStatusList: %v", err)
	}
	for i, v := range specVector2Bit {
		if err := l.set(i, v); err != nil {
			t.Fatalf("set(%d): %v", i, err)
		}
	}
	for i, v := range specVector2Bit {
		got, err := l.get(i)
		if err != nil {
			t.Fatalf("get(%d): %v", i, err)
		}
		if got != v {
			t.Fatalf("get(%d) = %d, want %d", i, got, v)
		}
	}
}

// TestDecodeSpecVectorLst pins us to the spec's published "lst" strings. Decompression is deterministic
// regardless of the compressor, so decoding the spec value must reproduce the exact packed bytes.
func TestDecodeSpecVectorLst(t *testing.T) {
	tests := []struct {
		name string
		lst  string
		bits int
		want []byte
	}{
		{"1-bit", "eNrbuRgAAhcBXQ", 1, []byte{0xb9, 0xa3}},
		{"2-bit", "eNo76fITAAPfAgc", 2, []byte{0xc9, 0x44, 0xf9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := decodeStatusList(tt.lst, tt.bits)
			if err != nil {
				t.Fatalf("decodeStatusList: %v", err)
			}
			if !bytes.Equal(l.data, tt.want) {
				t.Fatalf("decoded bytes = %x, want %x", l.data, tt.want)
			}
		})
	}
}

// TestEncodeDecodeSpecVectors ties our encoder to the spec's packed bytes without pinning to the
// reference compressor. The spec only mandates the ZLIB/DEFLATE format (RFC 1950/1951), not a canonical
// byte output — Go's compress/flate emits a valid but differently framed stream that decompresses to the
// identical payload. So we assert that encoding the spec's status values and decoding the result
// reproduces the spec's exact packed bytes, which is what a relying party actually depends on.
func TestEncodeDecodeSpecVectors(t *testing.T) {
	tests := []struct {
		name   string
		values []byte
		bits   int
		want   []byte
	}{
		{"1-bit", specVector1Bit, 1, []byte{0xb9, 0xa3}},
		{"2-bit", specVector2Bit, 2, []byte{0xc9, 0x44, 0xf9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := newStatusList(len(tt.values), tt.bits)
			if err != nil {
				t.Fatalf("newStatusList: %v", err)
			}
			for i, v := range tt.values {
				if err := l.set(i, v); err != nil {
					t.Fatalf("set(%d): %v", i, err)
				}
			}
			lst, err := l.encodeLst()
			if err != nil {
				t.Fatalf("encodeLst: %v", err)
			}
			decoded, err := decodeStatusList(lst, tt.bits)
			if err != nil {
				t.Fatalf("decodeStatusList: %v", err)
			}
			if !bytes.Equal(decoded.data, tt.want) {
				t.Fatalf("encode/decode bytes = %x, want %x", decoded.data, tt.want)
			}
		})
	}
}

// TestDecodeStatusListRejectsOversized guards the decompression bound: a small compressed payload that
// inflates past the accepted maximum must be rejected rather than fully materialized, so an adversarial
// "lst" cannot exhaust memory.
func TestDecodeStatusListRejectsOversized(t *testing.T) {
	// An all-zero list one byte larger than the bound compresses to a tiny payload but inflates past it.
	oversized, err := newStatusList(maxDecodedListBytes+1, 8)
	if err != nil {
		t.Fatalf("newStatusList: %v", err)
	}
	lst, err := oversized.encodeLst()
	if err != nil {
		t.Fatalf("encodeLst: %v", err)
	}
	if _, err := decodeStatusList(lst, 8); !errors.Is(err, errListTooLarge) {
		t.Fatalf("decodeStatusList error = %v, want errListTooLarge", err)
	}
}

func TestNewStatusListRejectsNegativeSize(t *testing.T) {
	if _, err := newStatusList(-1, 1); !errors.Is(err, errInvalidSize) {
		t.Fatalf("newStatusList(-1, 1) error = %v, want errInvalidSize", err)
	}
}

func TestDecodeStatusListRejectsInvalidBits(t *testing.T) {
	if _, err := decodeStatusList("AAAA", 3); !errors.Is(err, errInvalidBits) {
		t.Fatalf("decodeStatusList(_, 3) error = %v, want errInvalidBits", err)
	}
}

func TestDecodeStatusListRejectsBadBase64(t *testing.T) {
	if _, err := decodeStatusList("!!!not-base64!!!", 1); err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeStatusListRejectsNonZlib(t *testing.T) {
	// Valid base64url that is not a ZLIB stream must be rejected at inflation.
	lst := base64.RawURLEncoding.EncodeToString([]byte("not a zlib stream"))
	if _, err := decodeStatusList(lst, 1); err == nil {
		t.Fatal("expected error for a non-zlib payload")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	l, err := newStatusList(1000, 1)
	if err != nil {
		t.Fatalf("newStatusList: %v", err)
	}
	revoked := []int{0, 1, 7, 8, 63, 64, 512, 999}
	for _, idx := range revoked {
		if err := l.set(idx, statusInvalid); err != nil {
			t.Fatalf("set(%d): %v", idx, err)
		}
	}
	lst, err := l.encodeLst()
	if err != nil {
		t.Fatalf("encodeLst: %v", err)
	}
	decoded, err := decodeStatusList(lst, 1)
	if err != nil {
		t.Fatalf("decodeStatusList: %v", err)
	}
	if !bytes.Equal(decoded.data, l.data) {
		t.Fatalf("round-trip mismatch: got %x, want %x", decoded.data, l.data)
	}
	for _, idx := range revoked {
		got, err := decoded.get(idx)
		if err != nil {
			t.Fatalf("get(%d): %v", idx, err)
		}
		if got != statusInvalid {
			t.Fatalf("get(%d) = %d, want %d", idx, got, statusInvalid)
		}
	}
}

func TestNewStatusListRejectsInvalidBits(t *testing.T) {
	for _, bits := range []int{0, 3, 5, 7, 9, 16} {
		if _, err := newStatusList(8, bits); !errors.Is(err, errInvalidBits) {
			t.Fatalf("newStatusList(8, %d) error = %v, want errInvalidBits", bits, err)
		}
	}
}

func TestOutOfRangeIndex(t *testing.T) {
	l, err := newStatusList(16, 1)
	if err != nil {
		t.Fatalf("newStatusList: %v", err)
	}
	if err := l.set(16, statusInvalid); !errors.Is(err, errIndexOutOfRange) {
		t.Fatalf("set(16) error = %v, want errIndexOutOfRange", err)
	}
	if _, err := l.get(-1); !errors.Is(err, errIndexOutOfRange) {
		t.Fatalf("get(-1) error = %v, want errIndexOutOfRange", err)
	}
}
