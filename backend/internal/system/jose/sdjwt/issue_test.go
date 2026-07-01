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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
)

// es256Signer returns a SignFunc that signs with key. cryptolib.Generate returns
// the fixed-length P1363 (r||s) form the JWS wire format and verifyJWS require.
func es256Signer(t *testing.T, key *ecdsa.PrivateKey) SignFunc {
	t.Helper()
	return func(signingInput string) ([]byte, error) {
		return cryptolib.Generate([]byte(signingInput), cryptolib.ECDSASHA256, key)
	}
}

func holderJWK(t *testing.T, pub *ecdsa.PublicKey) map[string]interface{} {
	t.Helper()
	return map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(pub.X.FillBytes(make([]byte, 32))),
		"y":   base64.RawURLEncoding.EncodeToString(pub.Y.FillBytes(make([]byte, 32))),
	}
}

func TestIssueRoundTripsThroughVerify(t *testing.T) {
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate issuer key: %v", err)
	}
	holderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate holder key: %v", err)
	}
	cnf := holderJWK(t, &holderKey.PublicKey)

	combined, disclosures, err := Issue(IssueParams{
		Header:    map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"},
		Issuer:    "https://issuer.example",
		VCT:       "urn:eudi:pid:de:1",
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		SelectiveClaims: map[string]interface{}{
			"given_name":  "Erika",
			"family_name": "Mustermann",
			"birthdate":   "1986-03-22",
		},
		ConfirmationJWK: cnf,
	}, es256Signer(t, issuerKey))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if len(disclosures) != 3 {
		t.Fatalf("expected 3 disclosures, got %d", len(disclosures))
	}
	if !strings.HasSuffix(combined, separator) {
		t.Fatalf("issued credential must end with a trailing separator (no KB-JWT)")
	}

	p, err := Parse(combined)
	if err != nil {
		t.Fatalf("Parse issued credential: %v", err)
	}
	if p.HasKeyBinding() {
		t.Fatalf("issued credential must not carry a Key Binding JWT")
	}

	cred, err := Verify(p, VerifyOptions{IssuerKey: &issuerKey.PublicKey})
	if err != nil {
		t.Fatalf("Verify issued credential: %v", err)
	}

	if got, _ := cred.Claims["vct"].(string); got != "urn:eudi:pid:de:1" {
		t.Errorf("vct = %q, want urn:eudi:pid:de:1", got)
	}
	for name, want := range map[string]string{
		"given_name": "Erika", "family_name": "Mustermann", "birthdate": "1986-03-22",
	} {
		if got, _ := cred.Claims[name].(string); got != want {
			t.Errorf("claim %q = %q, want %q", name, got, want)
		}
	}
	if cred.ConfirmationKey == nil {
		t.Fatalf("expected cnf confirmation key to be present")
	}
	if got, _ := cred.ConfirmationKey["x"].(string); got != cnf["x"] {
		t.Errorf("cnf.jwk x mismatch: got %q, want %q", got, cnf["x"])
	}
}

func TestIssueRequiresIssuerAndVCT(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	sign := es256Signer(t, key)
	header := map[string]interface{}{"alg": "ES256", "typ": "dc+sd-jwt"}

	if _, _, err := Issue(IssueParams{Header: header, VCT: "x"}, sign); err == nil {
		t.Error("expected error when issuer is empty")
	}
	if _, _, err := Issue(IssueParams{Header: header, Issuer: "x"}, sign); err == nil {
		t.Error("expected error when vct is empty")
	}
	params := IssueParams{Header: map[string]interface{}{"typ": "dc+sd-jwt"}, Issuer: "i", VCT: "v"}
	if _, _, err := Issue(params, sign); err == nil {
		t.Error("expected error when header alg is missing")
	}
}
