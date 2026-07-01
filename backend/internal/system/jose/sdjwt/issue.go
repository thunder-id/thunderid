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

package sdjwt

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrIssue indicates the inputs to Issue were invalid.
var ErrIssue = errors.New("sdjwt: issue failed")

// saltBytes is the length of a disclosure salt (128 bits, base64url-encoded).
const saltBytes = 16

// SignFunc signs the JWS signing input ("<b64url(header)>.<b64url(payload)>")
// and returns the raw signature bytes to place in the third JWS segment. For
// ECDSA it MUST return the fixed-length P1363 (r||s) form per RFC 7518 §3.4 —
// the same form verifyJWS expects — not ASN.1/DER.
type SignFunc func(signingInput string) ([]byte, error)

// IssueParams describes a single SD-JWT VC to issue.
type IssueParams struct {
	// Header is the JWS protected header (must carry "alg" and "typ"; typically
	// also "x5c"/"kid"). It is serialized verbatim.
	Header map[string]interface{}
	// Issuer is the "iss" claim (always visible).
	Issuer string
	// VCT is the "vct" credential type claim (always visible).
	VCT string
	// IssuedAt sets "iat" when non-zero.
	IssuedAt time.Time
	// ExpiresAt sets "exp" when non-zero.
	ExpiresAt time.Time
	// SelectiveClaims are top-level claims made selectively disclosable: each
	// becomes a disclosure and contributes a digest to "_sd".
	SelectiveClaims map[string]interface{}
	// AlwaysVisible are claims embedded directly in the issuer payload.
	AlwaysVisible map[string]interface{}
	// ConfirmationJWK is the holder confirmation key; when set it is embedded as
	// "cnf":{"jwk":...} to bind the credential to the holder.
	ConfirmationJWK map[string]interface{}
	// HashAlg is the "_sd_alg" digest algorithm. Empty defaults to sha-256.
	HashAlg string
}

// Issue builds a combined-format issuer SD-JWT VC: the issuer-signed JWT
// followed by one disclosure per selective claim, terminated by a trailing
// separator and NO Key Binding JWT (the holder produces the KB-JWT at
// presentation time). The result is directly parseable by Parse and verifiable
// by Verify using the issuer's public key. The returned disclosures are the
// full set the holder may later present.
func Issue(p IssueParams, sign SignFunc) (string, []Disclosure, error) {
	if sign == nil {
		return "", nil, fmt.Errorf("%w: signer is required", ErrIssue)
	}
	if p.Issuer == "" || p.VCT == "" {
		return "", nil, fmt.Errorf("%w: issuer and vct are required", ErrIssue)
	}
	if alg, _ := p.Header["alg"].(string); alg == "" {
		return "", nil, fmt.Errorf("%w: header alg is required", ErrIssue)
	}

	hashAlg := p.HashAlg
	if hashAlg == "" {
		hashAlg = defaultHashAlg
	}

	disclosures := make([]Disclosure, 0, len(p.SelectiveClaims))
	digests := make([]string, 0, len(p.SelectiveClaims))
	for name, value := range p.SelectiveClaims {
		d, err := newObjectDisclosure(name, value, hashAlg)
		if err != nil {
			return "", nil, err
		}
		disclosures = append(disclosures, d)
		digests = append(digests, d.Digest)
	}
	// Sort digests so their order does not leak the disclosure order.
	sort.Strings(digests)

	payload := make(map[string]interface{}, len(p.AlwaysVisible)+6)
	for k, v := range p.AlwaysVisible {
		payload[k] = v
	}
	payload["iss"] = p.Issuer
	payload["vct"] = p.VCT
	if !p.IssuedAt.IsZero() {
		payload[claimIat] = p.IssuedAt.Unix()
	}
	if !p.ExpiresAt.IsZero() {
		payload["exp"] = p.ExpiresAt.Unix()
	}
	if p.ConfirmationJWK != nil {
		payload[claimCnf] = map[string]interface{}{"jwk": p.ConfirmationJWK}
	}
	if len(digests) > 0 {
		payload[claimSD] = digests
		payload[claimSDAlg] = hashAlg
	}

	headerJSON, err := json.Marshal(p.Header)
	if err != nil {
		return "", nil, fmt.Errorf("%w: marshal header: %w", ErrIssue, err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("%w: marshal payload: %w", ErrIssue, err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig, err := sign(signingInput)
	if err != nil {
		return "", nil, fmt.Errorf("%w: sign: %w", ErrIssue, err)
	}
	issuerJWT := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	combined := issuerJWT
	for _, d := range disclosures {
		combined += separator + d.Raw
	}
	// Trailing separator with an empty final segment signals "no Key Binding JWT".
	combined += separator

	return combined, disclosures, nil
}

// newObjectDisclosure builds an object-property disclosure ["<salt>","<name>",value]
// and computes its digest under hashAlg, mirroring how parseDisclosure decodes it.
func newObjectDisclosure(name string, value interface{}, hashAlg string) (Disclosure, error) {
	salt, err := randomSalt()
	if err != nil {
		return Disclosure{}, fmt.Errorf("%w: %w", ErrIssue, err)
	}
	encoded, err := json.Marshal([]interface{}{salt, name, value})
	if err != nil {
		return Disclosure{}, fmt.Errorf("%w: marshal disclosure %q: %w", ErrIssue, name, err)
	}
	raw := base64.RawURLEncoding.EncodeToString(encoded)
	return Disclosure{
		Raw:       raw,
		Digest:    computeDigest(raw, hashAlg),
		Salt:      salt,
		ClaimName: name,
		Value:     value,
	}, nil
}

// randomSalt returns a base64url-encoded 128-bit random salt.
func randomSalt() (string, error) {
	b := make([]byte, saltBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
