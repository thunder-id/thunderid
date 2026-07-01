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

package openid4vp

import (
	"context"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
)

const requestObjectType = "oauth-authz-req+jwt"

// cryptoProviderSigner signs request objects with a registered key from the
// runtime crypto provider, embedding the registered X.509 certificate as x5c.
type cryptoProviderSigner struct {
	cryptoProvider kmprovider.RuntimeCryptoProvider
	keyRef         kmprovider.KeyRef
	jwsAlg         string
	x5c            []string
}

// newRequestSigner loads the signing key by id. The key must be certificate-backed (CertificateDER → x5c).
func newRequestSigner(
	ctx context.Context, cryptoProvider kmprovider.RuntimeCryptoProvider, keyID string,
) (requestSigner, error) {
	if cryptoProvider == nil {
		return nil, fmt.Errorf("%w: crypto provider is required", ErrPolicy)
	}
	keys, err := cryptoProvider.GetPublicKeys(ctx, kmprovider.PublicKeyFilter{KeyID: keyID})
	if err != nil {
		return nil, fmt.Errorf("failed to load signing key %q: %w", keyID, err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no signing key found for key id %q", ErrPolicy, keyID)
	}
	key := keys[0]

	if _, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(key.Algorithm)); err != nil {
		return nil, fmt.Errorf("%w: unsupported signing algorithm for key %q", ErrPolicy, keyID)
	}
	if len(key.CertificateDER) == 0 {
		return nil, fmt.Errorf("%w: signing key %q is not certificate-backed (x5c required)", ErrPolicy, keyID)
	}

	chain := key.CertificateChainDER
	if len(chain) == 0 {
		chain = [][]byte{key.CertificateDER}
	}
	x5c := make([]string, 0, len(chain))
	for _, derBytes := range chain {
		x5c = append(x5c, base64.StdEncoding.EncodeToString(derBytes))
	}
	return &cryptoProviderSigner{
		cryptoProvider: cryptoProvider,
		keyRef:         kmprovider.KeyRef{KeyID: keyID},
		jwsAlg:         string(key.Algorithm),
		x5c:            x5c,
	}, nil
}

// signRequestObject signs the request object claims into a compact JWS using the crypto provider.
func (s *cryptoProviderSigner) signRequestObject(ctx context.Context, claims map[string]interface{}) (string, error) {
	// No kid header: for the x509_san_dns client scheme the wallet authenticates
	// the request via the x5c certificate. A stray kid (a JWK thumbprint) alongside
	// x5c trips strict wallets that try to resolve it first, so it is omitted.
	header := map[string]interface{}{
		"alg": s.jwsAlg,
		"typ": requestObjectType,
		"x5c": s.x5c,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request object header: %w", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request object claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	derSig, err := s.cryptoProvider.Sign(ctx, s.keyRef, s.jwsAlg, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("failed to sign request object: %w", err)
	}
	jwsSig := ecdsaDERToJWS(derSig, s.jwsAlg)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(jwsSig), nil
}

// ecdsaDERToJWS converts a DER-encoded ASN.1 ECDSA signature to the raw r||s
// fixed-size format required by JWS (RFC 7518 §3.4).
func ecdsaDERToJWS(derSig []byte, alg string) []byte {
	var sig struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(derSig, &sig); err != nil {
		return derSig // not DER (e.g. Ed25519): return as-is
	}
	var coordLen int
	switch jws.Algorithm(alg) {
	case jws.ES256:
		coordLen = 32
	case jws.ES384:
		coordLen = 48
	case jws.ES512:
		coordLen = 66
	default:
		return derSig
	}
	raw := make([]byte, 2*coordLen)
	rBytes := sig.R.Bytes()
	sBytes := sig.S.Bytes()
	copy(raw[coordLen-len(rBytes):coordLen], rBytes)
	copy(raw[2*coordLen-len(sBytes):], sBytes)
	return raw
}
