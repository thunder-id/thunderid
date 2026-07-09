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

package cryptolib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToParamsMap_RSAOAEP256(t *testing.T) {
	alg, params := AlgorithmParams{
		Algorithm:  AlgorithmRSAOAEP256,
		RSAOAEP256: RSAOAEP256Params{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()
	assert.Equal(t, string(AlgorithmRSAOAEP256), alg)
	assert.Equal(t, "A256GCM", params["contentEncryptionAlgorithm"])
}

func TestToParamsMap_RSAOAEP(t *testing.T) {
	alg, params := AlgorithmParams{
		Algorithm: AlgorithmRSAOAEP,
		RSAOAEP:   RSAOAEPParams{ContentEncryptionAlgorithm: "A128GCM"},
	}.ToParamsMap()
	assert.Equal(t, string(AlgorithmRSAOAEP), alg)
	assert.Equal(t, "A128GCM", params["contentEncryptionAlgorithm"])
}

func TestToParamsMap_ECDHES_MinimalParams(t *testing.T) {
	alg, params := AlgorithmParams{
		Algorithm: AlgorithmECDHES,
		ECDHES:    ECDHESParams{ContentEncryptionAlgorithm: "A256GCM"},
	}.ToParamsMap()
	assert.Equal(t, string(AlgorithmECDHES), alg)
	assert.Equal(t, "A256GCM", params["contentEncryptionAlgorithm"])
	assert.NotContains(t, params, "epk")
	assert.NotContains(t, params, "apu")
	assert.NotContains(t, params, "apv")
}

func TestToParamsMap_ECDHES_FullParams(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	alg, params := AlgorithmParams{
		Algorithm: AlgorithmECDHESA128KW,
		ECDHES: ECDHESParams{
			EPK:                        &ecKey.PublicKey,
			ContentEncryptionAlgorithm: "A128GCM",
			APU:                        []byte("apu-value"),
			APV:                        []byte("apv-value"),
		},
	}.ToParamsMap()
	assert.Equal(t, string(AlgorithmECDHESA128KW), alg)
	assert.Equal(t, "A128GCM", params["contentEncryptionAlgorithm"])
	assert.Equal(t, &ecKey.PublicKey, params["epk"])
	assert.Equal(t, []byte("apu-value"), params["apu"])
	assert.Equal(t, []byte("apv-value"), params["apv"])
}

func TestToParamsMap_AESKW(t *testing.T) {
	for _, alg := range []Algorithm{AlgorithmA128KW, AlgorithmA192KW, AlgorithmA256KW} {
		algStr, params := AlgorithmParams{
			Algorithm: alg,
			AESKW:     AESKWParams{ContentEncryptionAlgorithm: "A256GCM"},
		}.ToParamsMap()
		assert.Equal(t, string(alg), algStr)
		assert.Equal(t, "A256GCM", params["contentEncryptionAlgorithm"])
	}
}

func TestToParamsMap_Default(t *testing.T) {
	alg, params := AlgorithmParams{Algorithm: AlgorithmAESGCM}.ToParamsMap()
	assert.Equal(t, string(AlgorithmAESGCM), alg)
	assert.Nil(t, params)
}

func TestAlgorithmParamsFromMap_AESGCM(t *testing.T) {
	params, err := AlgorithmParamsFromMap(string(AlgorithmAESGCM), nil)
	require.NoError(t, err)
	assert.Equal(t, AlgorithmAESGCM, params.Algorithm)
}

func TestAlgorithmParamsFromMap_RSAOAEP256(t *testing.T) {
	params, err := AlgorithmParamsFromMap(string(AlgorithmRSAOAEP256), map[string]interface{}{
		"contentEncryptionAlgorithm": "A256GCM",
	})
	require.NoError(t, err)
	assert.Equal(t, AlgorithmRSAOAEP256, params.Algorithm)
	assert.Equal(t, Algorithm("A256GCM"), params.RSAOAEP256.ContentEncryptionAlgorithm)
}

func TestAlgorithmParamsFromMap_RSAOAEP256_MissingParam(t *testing.T) {
	_, err := AlgorithmParamsFromMap(string(AlgorithmRSAOAEP256), nil)
	assert.EqualError(t, err, "missing or invalid contentEncryptionAlgorithm for RSA-OAEP-256")
}

func TestAlgorithmParamsFromMap_RSAOAEP(t *testing.T) {
	params, err := AlgorithmParamsFromMap(string(AlgorithmRSAOAEP), map[string]interface{}{
		"contentEncryptionAlgorithm": "A128GCM",
	})
	require.NoError(t, err)
	assert.Equal(t, AlgorithmRSAOAEP, params.Algorithm)
	assert.Equal(t, Algorithm("A128GCM"), params.RSAOAEP.ContentEncryptionAlgorithm)
}

func TestAlgorithmParamsFromMap_RSAOAEP_MissingParam(t *testing.T) {
	_, err := AlgorithmParamsFromMap(string(AlgorithmRSAOAEP), nil)
	assert.EqualError(t, err, "missing or invalid contentEncryptionAlgorithm for RSA-OAEP")
}

func TestAlgorithmParamsFromMap_ECDHES(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	params, err := AlgorithmParamsFromMap(string(AlgorithmECDHES), map[string]interface{}{
		"contentEncryptionAlgorithm": "A256GCM",
		"epk":                        &ecKey.PublicKey,
		"apu":                        []byte("apu-value"),
		"apv":                        []byte("apv-value"),
	})
	require.NoError(t, err)
	assert.Equal(t, AlgorithmECDHES, params.Algorithm)
	assert.Equal(t, &ecKey.PublicKey, params.ECDHES.EPK)
	assert.Equal(t, []byte("apu-value"), params.ECDHES.APU)
	assert.Equal(t, []byte("apv-value"), params.ECDHES.APV)
}

func TestAlgorithmParamsFromMap_ECDHES_MissingContentEncryptionAlgorithm(t *testing.T) {
	_, err := AlgorithmParamsFromMap(string(AlgorithmECDHES), nil)
	assert.EqualError(t, err, "missing or invalid contentEncryptionAlgorithm for ECDH-ES")
}

func TestAlgorithmParamsFromMap_ECDHES_InvalidAPU(t *testing.T) {
	_, err := AlgorithmParamsFromMap(string(AlgorithmECDHES), map[string]interface{}{
		"contentEncryptionAlgorithm": "A256GCM",
		"apu":                        "not-bytes",
	})
	assert.EqualError(t, err, "invalid apu for ECDH-ES")
}

func TestAlgorithmParamsFromMap_ECDHES_InvalidAPV(t *testing.T) {
	_, err := AlgorithmParamsFromMap(string(AlgorithmECDHES), map[string]interface{}{
		"contentEncryptionAlgorithm": "A256GCM",
		"apv":                        "not-bytes",
	})
	assert.EqualError(t, err, "invalid apv for ECDH-ES")
}

func TestAlgorithmParamsFromMap_AESKW(t *testing.T) {
	for _, alg := range []Algorithm{AlgorithmA128KW, AlgorithmA192KW, AlgorithmA256KW} {
		params, err := AlgorithmParamsFromMap(string(alg), map[string]interface{}{
			"contentEncryptionAlgorithm": "A256GCM",
		})
		require.NoError(t, err)
		assert.Equal(t, alg, params.Algorithm)
		assert.Equal(t, Algorithm("A256GCM"), params.AESKW.ContentEncryptionAlgorithm)
	}
}

func TestAlgorithmParamsFromMap_AESKW_MissingParam(t *testing.T) {
	_, err := AlgorithmParamsFromMap(string(AlgorithmA128KW), nil)
	assert.EqualError(t, err, "missing or invalid contentEncryptionAlgorithm for A128KW")
}

func TestAlgorithmParamsFromMap_UnsupportedAlgorithm(t *testing.T) {
	_, err := AlgorithmParamsFromMap("UNSUPPORTED", nil)
	assert.EqualError(t, err, "unsupported algorithm: UNSUPPORTED")
}
