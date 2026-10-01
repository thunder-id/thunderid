// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwe

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// algParamsFromMap reconstructs a cryptolib.AlgorithmParams from the generic params map that crosses
// the providers.RuntimeCryptoProvider boundary, mirroring what a real provider implementation (e.g.
// defaultkm) does. Used by test doubles to perform real crypto and validate round trips.
func algParamsFromMap(algorithm string, params map[string]interface{}) cryptolib.AlgorithmParams {
	alg := cryptolib.Algorithm(algorithm)
	algParams := cryptolib.AlgorithmParams{Algorithm: alg}

	encAlg, _ := params[providers.ParamContentEncryptionAlgorithm].(string)

	switch alg {
	case cryptolib.AlgorithmRSAOAEP:
		algParams.RSAOAEP = cryptolib.RSAOAEPParams{ContentEncryptionAlgorithm: cryptolib.Algorithm(encAlg)}
	case cryptolib.AlgorithmRSAOAEP256:
		algParams.RSAOAEP256 = cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: cryptolib.Algorithm(encAlg)}
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		apu, _ := params[providers.ParamAPU].([]byte)
		apv, _ := params[providers.ParamAPV].([]byte)
		algParams.ECDHES = cryptolib.ECDHESParams{
			EPK:                        epkFromParam(params[providers.ParamEPK]),
			ContentEncryptionAlgorithm: cryptolib.Algorithm(encAlg),
			APU:                        apu,
			APV:                        apv,
		}
	}

	return algParams
}

// epkFromParam mirrors defaultkm.epkFromParam: the epk arrives as a JWK map decoded from the JWE
// "epk" header and must be converted to the *ecdh.PublicKey cryptolib expects.
func epkFromParam(epk interface{}) crypto.PublicKey {
	epkMap, ok := epk.(map[string]interface{})
	if !ok {
		return epk
	}

	pub, err := defaultkm.JWKToPublicKey(epkMap)
	if err != nil {
		return nil
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil
	}
	ecdhPub, err := ecdsaPub.ECDH()
	if err != nil {
		return nil
	}
	return ecdhPub
}

type JWEServiceTestSuite struct {
	suite.Suite
	jweService        *jweService
	testRSAPrivateKey *rsa.PrivateKey
	testECPrivateKey  *ecdsa.PrivateKey
}

func TestJWEServiceSuite(t *testing.T) {
	suite.Run(t, new(JWEServiceTestSuite))
}

func (suite *JWEServiceTestSuite) SetupTest() {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	suite.testRSAPrivateKey = rsaKey

	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.testECPrivateKey = ecKey
}

// newRoundTripMockProvider returns a mock RuntimeCryptoProvider that performs real crypto for Encrypt
// and Decrypt (delegating to cryptolib using privateKey), so Encrypt/Decrypt round trips can be
// exercised through jweService without depending on a concrete provider implementation.
func (suite *JWEServiceTestSuite) newRoundTripMockProvider(
	privateKey any, times int,
) *cryptomock.RuntimeCryptoProviderMock {
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	mockProvider.EXPECT().
		Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{},
			content []byte,
		) ([]byte, *providers.CryptoDetails, error) {
			algParams := algParamsFromMap(algorithm, params)
			return cryptolib.Encrypt(keyRef.PublicKey, &algParams, content)
		}).Times(times)
	mockProvider.EXPECT().
		Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{},
			content []byte,
		) ([]byte, error) {
			algParams := algParamsFromMap(algorithm, params)
			return cryptolib.Decrypt(privateKey, algParams, content)
		}).Times(times)
	return mockProvider
}

func (suite *JWEServiceTestSuite) TestEncryptDecrypt_RSA() {
	encAlgs := []ContentEncAlgorithm{A128GCM, A192GCM, A256GCM}

	mockProvider := suite.newRoundTripMockProvider(suite.testRSAPrivateKey, len(encAlgs))

	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         providers.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	payload := []byte("Hello, RSA JWE!")
	recipientPublicKey := &providers.KeyRef{PublicKey: &suite.testRSAPrivateKey.PublicKey}

	for _, enc := range encAlgs {
		jweToken, sErr := suite.jweService.Encrypt(
			context.Background(),
			payload,
			recipientPublicKey,
			string(RSAOAEP256),
			enc,
			"",
			"")
		assert.Nil(suite.T(), sErr)
		decrypted, sErr := suite.jweService.Decrypt(context.Background(), jweToken)
		assert.Nil(suite.T(), sErr)
		assert.Equal(suite.T(), payload, decrypted)
	}
}

func (suite *JWEServiceTestSuite) TestEncryptDecrypt_ECDH() {
	testCases := []struct {
		alg KeyEncAlgorithm
		enc ContentEncAlgorithm
	}{
		{ECDHES, A128GCM},
		{ECDHES, A192GCM},
		{ECDHES, A256GCM},
		{ECDHESA128KW, A128GCM},
		{ECDHESA256KW, A256GCM},
	}

	mockProvider := suite.newRoundTripMockProvider(suite.testECPrivateKey, len(testCases))

	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         providers.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	payload := []byte("Hello, ECDH JWE!")
	recipientPublicKey := &providers.KeyRef{PublicKey: &suite.testECPrivateKey.PublicKey}

	for _, tc := range testCases {
		jweToken, sErr := suite.jweService.Encrypt(
			context.Background(),
			payload,
			recipientPublicKey,
			string(tc.alg),
			tc.enc,
			"",
			"")
		assert.Nil(suite.T(), sErr)
		decrypted, sErr := suite.jweService.Decrypt(context.Background(), jweToken)
		assert.Nil(suite.T(), sErr)
		assert.Equal(suite.T(), payload, decrypted)
	}
}

func (suite *JWEServiceTestSuite) TestEncrypt_Errors() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	// Unsupported Encryption algorithm — rejected locally before the provider is consulted.
	_, sErr := suite.jweService.Encrypt(
		context.Background(),
		[]byte("p"),
		&providers.KeyRef{PublicKey: &suite.testRSAPrivateKey.PublicKey},
		string(RSAOAEP256),
		"INVALID",
		"",
		"")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedEncryptionAlgorithm, *sErr)

	// Provider rejects the key establishment (e.g. RSA algorithm with an EC key).
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	mockProvider.EXPECT().
		Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, errors.New("key is not an RSA public key"))
	suite.jweService = &jweService{cryptoProvider: mockProvider, logger: log.GetLogger()}

	_, sErr = suite.jweService.Encrypt(
		context.Background(),
		[]byte("p"),
		&providers.KeyRef{PublicKey: &suite.testECPrivateKey.PublicKey},
		string(RSAOAEP256),
		A128GCM,
		"",
		"")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedJWEAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestEncrypt_NoCryptoProvider() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	_, sErr := suite.jweService.Encrypt(
		context.Background(),
		[]byte("p"),
		&providers.KeyRef{PublicKey: &suite.testRSAPrivateKey.PublicKey},
		string(RSAOAEP256),
		A128GCM,
		"",
		"")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedJWEAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestDecrypt_Errors() {
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         providers.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	// Invalid JWE format — no provider call
	_, sErr := suite.jweService.Decrypt(context.Background(), "invalid.jwe")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Encrypt a valid token via the provider.
	mockProvider.EXPECT().
		Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{},
			content []byte,
		) ([]byte, *providers.CryptoDetails, error) {
			algParams := algParamsFromMap(algorithm, params)
			return cryptolib.Encrypt(keyRef.PublicKey, &algParams, content)
		}).Once()
	payload := []byte("data")
	jweToken, _ := suite.jweService.Encrypt(
		context.Background(),
		payload,
		&providers.KeyRef{PublicKey: &suite.testRSAPrivateKey.PublicKey},
		string(RSAOAEP256),
		A128GCM,
		"",
		"")

	// DecryptKey failure: provider returns an error
	mockProvider.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("key decryption error")).Once()
	_, sErr = suite.jweService.Decrypt(context.Background(), jweToken)
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorJWEDecryptionFailed, *sErr)

	// DecryptContent failure (tampered tag): provider returns correct CEK but tag is wrong
	mockProvider.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{},
			content []byte,
		) ([]byte, error) {
			algParams := algParamsFromMap(algorithm, params)
			return cryptolib.Decrypt(suite.testRSAPrivateKey, algParams, content)
		}).Once()
	parts := strings.Split(jweToken, ".")
	parts[4] = base64.RawURLEncoding.EncodeToString([]byte("wrong-tag"))
	tamperedToken := strings.Join(parts, ".")
	_, sErr = suite.jweService.Decrypt(context.Background(), tamperedToken)
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorJWEDecryptionFailed, *sErr)
}

func (suite *JWEServiceTestSuite) TestInitialize() {
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())

	cfg := joseconfig.Config{PreferredKeyID: "test-kid"}
	service, err := Initialize(mockProvider, cfg)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
}

func (suite *JWEServiceTestSuite) TestDecrypt_EdgeCases() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	// Test with malformed JWE (wrong number of parts)
	_, sErr := suite.jweService.Decrypt(context.Background(), "malformed.jwe")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Test with invalid base64 in header
	_, sErr = suite.jweService.Decrypt(context.Background(), "invalid-base64.key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Test with invalid JSON in header
	invalidHeader := base64.RawURLEncoding.EncodeToString([]byte("{invalid json"))
	_, sErr = suite.jweService.Decrypt(context.Background(), invalidHeader+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Test with missing required header fields
	headerMissingAlg := base64.RawURLEncoding.EncodeToString([]byte(`{"enc":"A128GCM"}`))
	_, sErr = suite.jweService.Decrypt(context.Background(), headerMissingAlg+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedJWEAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestIsSupportedEnc() {
	valid := []ContentEncAlgorithm{A128GCM, A192GCM, A256GCM, A128CBCHS256, A192CBCHS384, A256CBCHS512}
	for _, enc := range valid {
		assert.True(suite.T(), isSupportedEnc(enc), "expected %s to be supported", enc)
	}
	assert.False(suite.T(), isSupportedEnc("INVALID"))
	assert.False(suite.T(), isSupportedEnc(""))
}

func (suite *JWEServiceTestSuite) TestSupportedContentEncryptionAlgorithms() {
	svc := &jweService{}
	algs := svc.SupportedContentEncryptionAlgorithms()
	expected := []string{
		string(A128CBCHS256), string(A192CBCHS384), string(A256CBCHS512),
		string(A128GCM), string(A192GCM), string(A256GCM),
	}
	assert.ElementsMatch(suite.T(), expected, algs)
}

func (suite *JWEServiceTestSuite) TestSupportedKeyEncryptionAlgorithms() {
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	mockProvider.EXPECT().GetSupportedEncryptionAlgorithms().Return([]string{
		string(RSAOAEP), string(RSAOAEP256), string(ECDHES), "SOME-UNKNOWN-ALG",
	}).Once()
	svc := &jweService{cryptoProvider: mockProvider}

	algs := svc.SupportedKeyEncryptionAlgorithms()
	assert.ElementsMatch(suite.T(), []string{string(RSAOAEP), string(RSAOAEP256), string(ECDHES)}, algs)
}

func (suite *JWEServiceTestSuite) TestBuildDecryptParams() {
	// RSAOAEP — no EPK needed
	header := map[string]interface{}{"alg": "RSA-OAEP", "enc": "A128GCM"}
	params, err := buildDecryptParams(RSAOAEP, A128GCM, header)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), cryptolib.AlgorithmRSAOAEP, params.Algorithm)

	// ECDHESA192KW — needs a valid EPK in the header
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecdhPub, _ := privKey.PublicKey.ECDH()
	epkMap, _ := epkToMap(ecdhPub)
	headerWithEPK := map[string]interface{}{
		"alg": "ECDH-ES+A192KW",
		"enc": "A256GCM",
		"epk": epkMap,
	}
	params, err = buildDecryptParams(ECDHESA192KW, A256GCM, headerWithEPK)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), cryptolib.AlgorithmECDHESA192KW, params.Algorithm)

	// Missing EPK for all ECDH-ES variants
	headerNoEPK := map[string]interface{}{"alg": "ECDH-ES", "enc": "A128GCM"}
	for _, alg := range []KeyEncAlgorithm{ECDHES, ECDHESA128KW, ECDHESA192KW, ECDHESA256KW} {
		_, err = buildDecryptParams(alg, A128GCM, headerNoEPK)
		assert.Error(suite.T(), err, "expected error for %s with missing EPK", alg)
	}

	// Unsupported algorithm (default branch)
	_, err = buildDecryptParams("A128KW", A128GCM, header)
	assert.Error(suite.T(), err)
}

func (suite *JWEServiceTestSuite) TestEncrypt_WithKidAndCty() {
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	mockProvider.EXPECT().
		Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{},
			content []byte,
		) ([]byte, *providers.CryptoDetails, error) {
			algParams := algParamsFromMap(algorithm, params)
			return cryptolib.Encrypt(keyRef.PublicKey, &algParams, content)
		}).Once()

	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         providers.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	token, sErr := suite.jweService.Encrypt(context.Background(),
		[]byte("payload"), &providers.KeyRef{PublicKey: &suite.testRSAPrivateKey.PublicKey},
		string(RSAOAEP256), A128GCM, "JWT", "my-kid")
	assert.Nil(suite.T(), sErr)

	parsedHeader, _, _, _, _, _, err := DecodeJWE(token)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "my-kid", parsedHeader["kid"])
	assert.Equal(suite.T(), "JWT", parsedHeader["cty"])
}

func (suite *JWEServiceTestSuite) TestDecrypt_MissingEncField() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	headerNoEnc := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RSA-OAEP-256"}`))
	_, sErr := suite.jweService.Decrypt(context.Background(), headerNoEnc+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedEncryptionAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestDecrypt_UnsupportedAlgorithmForDecrypt() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	// A128KW is valid for encrypt but hits the default branch in buildDecryptParams
	headerAESKW := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"A128KW","enc":"A128GCM"}`))
	_, sErr := suite.jweService.Decrypt(context.Background(), headerAESKW+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedJWEAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestEncryptDecrypt_CBC() {
	encAlgs := []ContentEncAlgorithm{A128CBCHS256, A192CBCHS384, A256CBCHS512}

	mockProvider := suite.newRoundTripMockProvider(suite.testRSAPrivateKey, len(encAlgs))

	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         providers.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	payload := []byte("Hello, CBC JWE!")
	for _, enc := range encAlgs {
		jweToken, sErr := suite.jweService.Encrypt(
			context.Background(),
			payload,
			&providers.KeyRef{PublicKey: &suite.testRSAPrivateKey.PublicKey},
			string(RSAOAEP256),
			enc,
			"",
			"")
		assert.Nil(suite.T(), sErr, "enc=%s", enc)
		decrypted, sErr := suite.jweService.Decrypt(context.Background(), jweToken)
		assert.Nil(suite.T(), sErr, "enc=%s", enc)
		assert.Equal(suite.T(), payload, decrypted, "enc=%s", enc)
	}
}

func (suite *JWEServiceTestSuite) TestEncrypt_ExtraProtectedHeader() {
	suite.jweService = &jweService{
		cryptoProvider: suite.newRoundTripMockProvider(suite.testRSAPrivateKey, 1),
		keyRef:         providers.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}
	payload := []byte("logout token")
	recipientPublicKey := &providers.KeyRef{PublicKey: &suite.testRSAPrivateKey.PublicKey}

	jweToken, sErr := suite.jweService.Encrypt(context.Background(), payload, recipientPublicKey,
		string(RSAOAEP256), A256GCM, "JWT", "rp-kid",
		WithProtectedHeader("iss", "https://op.example.com"),
		WithProtectedHeader("alg", "none"),
		WithProtectedHeader("typ", "JWT"),
		WithProtectedHeader("zip", "DEF"))
	assert.Nil(suite.T(), sErr)

	headerJSON, err := base64.RawURLEncoding.DecodeString(strings.Split(jweToken, ".")[0])
	assert.NoError(suite.T(), err)
	var header map[string]interface{}
	assert.NoError(suite.T(), json.Unmarshal(headerJSON, &header))
	assert.Equal(suite.T(), "https://op.example.com", header["iss"], "the extra header is present")
	assert.Equal(suite.T(), string(RSAOAEP256), header["alg"], "an option cannot replace a computed header")
	assert.Equal(suite.T(), "JWE", header["typ"])
	assert.NotContains(suite.T(), header, "zip", "zip is dropped because Encrypt does not compress")
	assert.Equal(suite.T(), "JWT", header["cty"])
	assert.Equal(suite.T(), "rp-kid", header["kid"])

	decrypted, sErr := suite.jweService.Decrypt(context.Background(), jweToken)
	assert.Nil(suite.T(), sErr)
	assert.Equal(suite.T(), payload, decrypted)
}
