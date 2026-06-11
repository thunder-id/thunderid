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

package jwe

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
)

// fabricateECDHESJWE builds an ECDH-ES / A128GCM JWE for recipientPub, mirroring
// what an OpenID4VP wallet produces for an ephemeral verifier key.
func fabricateECDHESJWE(t *testing.T, recipientPub *ecdsa.PublicKey, payload []byte) string {
	t.Helper()
	params := cryptolib.AlgorithmParams{
		Algorithm: cryptolib.AlgorithmECDHES,
		ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: cryptolib.Algorithm(A128GCM)},
	}
	encryptedKey, details, err := cryptolib.Encrypt(recipientPub, &params, nil)
	require.NoError(t, err)

	epkMap, err := epkToMap(details.EPK)
	require.NoError(t, err)
	header := map[string]interface{}{
		"typ": "JWE", "alg": string(ECDHES), "enc": string(A128GCM), "epk": epkMap,
	}
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	iv, ciphertext, tag, err := encryptContent(payload, details.CEK, A128GCM, []byte(headerB64))
	require.NoError(t, err)

	return strings.Join([]string{
		headerB64,
		base64.RawURLEncoding.EncodeToString(encryptedKey),
		base64.RawURLEncoding.EncodeToString(iv),
		base64.RawURLEncoding.EncodeToString(ciphertext),
		base64.RawURLEncoding.EncodeToString(tag),
	}, ".")
}

func TestDecryptWithKey_ECDHESRoundTrip(t *testing.T) {
	ephemeral, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	payload := []byte(`{"vp_token":{"pid-sd-jwt":["abc~def~"]},"state":"xyz"}`)
	jweToken := fabricateECDHESJWE(t, &ephemeral.PublicKey, payload)

	got, err := DecryptWithKey(jweToken, ephemeral)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestDecryptWithKey_WrongKeyFails(t *testing.T) {
	ephemeral, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jweToken := fabricateECDHESJWE(t, &ephemeral.PublicKey, []byte("secret"))

	wrong, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	_, err = DecryptWithKey(jweToken, wrong)
	assert.Error(t, err)
}

func TestDecryptWithKey_MalformedToken(t *testing.T) {
	ephemeral, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	_, err = DecryptWithKey("not.a.valid.jwe", ephemeral)
	assert.Error(t, err)
}
