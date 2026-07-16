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

// Package tokenstatus implements the IETF OAuth Token Status List
// (draft-ietf-oauth-status-list-21) as an Authorization-Server-independent subsystem: it allocates a
// status index at issuance, records token status, and produces the signed Status List Token. It depends
// only on system-level services (database, JWT signing, config, logging) and must never import the AS
// packages (tokenservice, revocation, discovery); the Authorization Server consumes it through narrow
// interfaces it owns, wired at the composition root, so the subsystem can run in-process today or behind
// a remote Status Provider later without changing any caller.
package tokenstatus

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// ServiceInterface is the outward API of the Token Status List subsystem. The Authorization Server
// consumes it through its own narrow interfaces (matching subsets of these methods), so the AS never
// imports this package directly and the subsystem can run in-process or behind a remote Status Provider.
type ServiceInterface interface {
	// IssueReference allocates the next status index and returns it with the list's public URI, ready
	// to stamp into a token's status claim.
	IssueReference(ctx context.Context) (idx int64, uri string, err error)
	// SetStatus records the status of the token referenced by (uri, idx), expiring at expiry. Used by
	// the revocation write path.
	SetStatus(ctx context.Context, uri string, idx int64, status int, expiry time.Time) error
	// GetStatus returns the stored status of the token referenced by (uri, idx). Used by AS-internal
	// enforcement. An in-range index with no recorded entry is VALID; a nonexistent list or an index
	// out of the list's bounds is not resolvable and returns an error (the caller fails closed), per
	// draft-ietf-oauth-status-list §8.3.
	GetStatus(ctx context.Context, uri string, idx int64) (int, error)
	// Produce builds and signs the Status List Token for listID, returning the serialized token and the
	// ttl (seconds) that bounds its freshness. Used by the publish endpoint and, in-process, as the
	// Resource Server cache's source of the artifact it decodes. Returns ErrListNotFound when the list
	// does not exist.
	Produce(ctx context.Context, listID string) (token string, ttl int, err error)
}

// service is the in-process implementation of ServiceInterface over the operation-database store.
type service struct {
	store      statusStoreInterface
	jwtService jwt.JWTServiceInterface
	baseURL    string
	ttlSeconds int
}

// IssueReference allocates the next index and pairs it with the owning list's URI.
func (s *service) IssueReference(ctx context.Context) (int64, string, error) {
	listID, idx, err := s.store.allocateIndex(ctx)
	if err != nil {
		return 0, "", err
	}
	return idx, s.listURI(listID), nil
}

// SetStatus resolves the list id from the URI and records the token's status.
func (s *service) SetStatus(ctx context.Context, uri string, idx int64, status int, expiry time.Time) error {
	listID, err := ListIDFromURI(uri)
	if err != nil {
		return err
	}
	return s.store.setStatus(ctx, listID, idx, byte(status), expiry)
}

// GetStatus resolves the list from the URI, verifies the list exists and the index is within its
// bounds, then reads the token's status. A nonexistent list yields ErrListNotFound and an out-of-range
// index yields errIndexOutOfRange, so an unresolvable reference is rejected rather than defaulting to
// VALID (draft-ietf-oauth-status-list §8.3). An in-range index with no entry is VALID.
func (s *service) GetStatus(ctx context.Context, uri string, idx int64) (int, error) {
	listID, err := ListIDFromURI(uri)
	if err != nil {
		return 0, err
	}
	rec, found, err := s.store.getList(ctx, listID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrListNotFound
	}
	if idx < 0 || idx >= rec.capacity {
		return 0, errIndexOutOfRange
	}
	status, err := s.store.getStatus(ctx, listID, idx)
	if err != nil {
		return 0, err
	}
	return int(status), nil
}

// DecodeStatusListToken reverses Produce for a relying party: it reads the status_list claim from a
// Status List Token, inflates its bit array, and returns the non-VALID entries keyed by index together
// with the entry capacity the array covers. It decodes the payload only and does not verify the
// signature — the caller is responsible for verifying the token (the in-process Resource Server cache
// trusts the signer it just invoked; a remote reader must verify before decoding). capacity is the
// byte-padded array size, a multiple of the per-byte entry count, so it may exceed the list's configured
// capacity; the surplus indices are unallocated and read VALID.
func DecodeStatusListToken(token string) (statuses map[int64]int, capacity int64, err error) {
	payload, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, 0, err
	}
	claim, ok := payload[claimStatusList].(map[string]interface{})
	if !ok {
		return nil, 0, errMalformedStatusListToken
	}
	lst, ok := claim[claimLst].(string)
	if !ok {
		return nil, 0, errMalformedStatusListToken
	}
	bitsVal, ok := claim[claimBits].(float64)
	if !ok {
		return nil, 0, errMalformedStatusListToken
	}
	bits := int(bitsVal)
	list, err := decodeStatusList(lst, bits)
	if err != nil {
		return nil, 0, err
	}
	capacity = int64(len(list.data) * (8 / bits))
	statuses = make(map[int64]int)
	for i := int64(0); i < capacity; i++ {
		v, err := list.get(int(i))
		if err != nil {
			return nil, 0, err
		}
		if v != statusValid {
			statuses[i] = int(v)
		}
	}
	return statuses, capacity, nil
}

// Produce builds the Status List Token for a list: it packs the sparse revoked entries into a bit array
// sized to the list capacity, compresses it, and signs it as a JWT with typ statuslist+jwt. The signer
// (GenerateJWT) also sets iss/exp/iat/nbf/jti and requires an aud; a status-list relying party validates
// sub (== the list URI), typ, and exp, and ignores the extra claims.
func (s *service) Produce(ctx context.Context, listID string) (string, int, error) {
	rec, found, err := s.store.getList(ctx, listID)
	if err != nil {
		return "", 0, err
	}
	if !found {
		return "", 0, ErrListNotFound
	}

	list, err := newStatusList(int(rec.capacity), rec.bits)
	if err != nil {
		return "", 0, err
	}

	entries, err := s.store.listEntries(ctx, listID)
	if err != nil {
		return "", 0, err
	}
	for _, e := range entries {
		if err := list.set(int(e.idx), e.status); err != nil {
			return "", 0, err
		}
	}

	lst, err := list.encodeLst()
	if err != nil {
		return "", 0, err
	}

	uri := s.listURI(listID)
	claims := map[string]interface{}{
		claimAud: s.baseURL,
		claimTTL: s.ttlSeconds,
		claimStatusList: map[string]interface{}{
			claimBits: rec.bits,
			claimLst:  lst,
		},
	}

	token, _, svcErr := s.jwtService.GenerateJWT(
		ctx, uri, "", int64(s.ttlSeconds), claims, jwt.TokenTypeStatusList, "")
	if svcErr != nil {
		return "", 0, fmt.Errorf("failed to sign status list token: %v", svcErr.Error)
	}
	return token, s.ttlSeconds, nil
}

// listURI builds the public URI of a list from the configured base URL.
func (s *service) listURI(listID string) string {
	return strings.TrimRight(s.baseURL, "/") + statusListURISegment + listID
}

// ListIDFromURI extracts the list id from a status list URI. It reads the segment after the last
// statusListURISegment so the subsystem does not depend on the caller's base URL matching its own,
// which matters when the URI was stamped by a different issuer instance.
func ListIDFromURI(uri string) (string, error) {
	i := strings.LastIndex(uri, statusListURISegment)
	if i < 0 {
		return "", errInvalidListURI
	}
	id := uri[i+len(statusListURISegment):]
	if id == "" || strings.ContainsAny(id, "/?#") {
		return "", errInvalidListURI
	}
	return id, nil
}

// statusList is a packed bit array holding one status per referenced token. Each entry occupies bits
// bits (1, 2, 4, or 8); entries are packed into each byte from the least significant bit to the most
// significant bit, and bytes increment in index order (spec §4.1). This packing is deterministic and
// format-neutral — the same array underlies the JWT and CWT representations.
type statusList struct {
	bits int
	data []byte
}

// validBits reports whether n is a status-list entry width permitted by the spec (§4.1): 1, 2, 4, or 8.
func validBits(n int) bool {
	return n == 1 || n == 2 || n == 4 || n == 8
}

// newStatusList allocates a status list holding size entries of bits width each, initialized to
// statusValid (all zero). It returns an error for an unsupported width or a negative size.
func newStatusList(size, bits int) (*statusList, error) {
	if !validBits(bits) {
		return nil, errInvalidBits
	}
	if size < 0 {
		return nil, errInvalidSize
	}
	entriesPerByte := 8 / bits
	byteLen := (size + entriesPerByte - 1) / entriesPerByte
	return &statusList{bits: bits, data: make([]byte, byteLen)}, nil
}

// locate maps an entry index to the byte position holding it and the bit shift of its least significant
// bit within that byte. ok is false when idx falls outside the allocated array.
func (l *statusList) locate(idx int) (bytePos, shift int, ok bool) {
	if idx < 0 {
		return 0, 0, false
	}
	entriesPerByte := 8 / l.bits
	bytePos = idx / entriesPerByte
	if bytePos >= len(l.data) {
		return 0, 0, false
	}
	shift = (idx % entriesPerByte) * l.bits
	return bytePos, shift, true
}

// get returns the status value stored at idx. It returns errIndexOutOfRange when idx is outside the
// array, so a reference carrying an index this list does not cover is rejected rather than misread.
func (l *statusList) get(idx int) (byte, error) {
	bytePos, shift, ok := l.locate(idx)
	if !ok {
		return 0, errIndexOutOfRange
	}
	mask := byte((1 << l.bits) - 1)
	return (l.data[bytePos] >> shift) & mask, nil
}

// set writes value at idx, masking it to the entry width. It returns errIndexOutOfRange when idx is
// outside the array.
func (l *statusList) set(idx int, value byte) error {
	bytePos, shift, ok := l.locate(idx)
	if !ok {
		return errIndexOutOfRange
	}
	mask := byte((1 << l.bits) - 1)
	l.data[bytePos] = (l.data[bytePos] &^ (mask << shift)) | ((value & mask) << shift)
	return nil
}

// encodeLst compresses the packed bit array with DEFLATE in the ZLIB data format (spec §4.1, RFC 1950 /
// RFC 1951) and base64url-encodes the result without padding, producing the "lst" value carried in a
// Status List Token. Best compression is used because the array is overwhelmingly zero (VALID) and the
// artifact is cached and served repeatedly, so a smaller payload pays off on every fetch.
func (l *statusList) encodeLst() (string, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(l.data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

// decodeStatusList reverses encodeLst: it base64url-decodes lst, inflates the ZLIB stream, and returns a
// status list of the given entry width over the recovered bytes. It is used by a relying party reading a
// published Status List Token. bits must match the token's declared width. Inflation is bounded to
// maxDecodedListBytes so an adversarial payload cannot exhaust memory.
func decodeStatusList(lst string, bits int) (*statusList, error) {
	if !validBits(bits) {
		return nil, errInvalidBits
	}
	compressed, err := base64.RawURLEncoding.DecodeString(lst)
	if err != nil {
		return nil, err
	}
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(io.LimitReader(r, maxDecodedListBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDecodedListBytes {
		return nil, errListTooLarge
	}
	return &statusList{bits: bits, data: data}, nil
}
