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

package openid4vci

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
	"github.com/thunder-id/thunderid/tests/mocks/vc/credentialmock"
)

type ServiceTestSuite struct {
	suite.Suite
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) TestNewOpenID4VCIService() {
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	store := newOpenID4VCIStoreInterfaceMock(s.T())
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())

	s.Run("Success", func() {
		svc, err := newOpenID4VCIService(
			serviceConfig{CredentialIssuer: testIssuer},
			provider, kmprovider.KeyRef{}, "ES256", "", nil,
			store, jwtSvc, userSvc, creds)
		s.Require().NoError(err)
		s.Require().NotNil(svc)
	})

	s.Run("MissingDependency", func() {
		svc, err := newOpenID4VCIService(
			serviceConfig{CredentialIssuer: testIssuer},
			nil, kmprovider.KeyRef{}, "ES256", "", nil,
			store, jwtSvc, userSvc, creds)
		s.ErrorIs(err, ErrPolicy)
		s.Nil(svc)
	})

	s.Run("MissingCredentialIssuer", func() {
		svc, err := newOpenID4VCIService(
			serviceConfig{},
			provider, kmprovider.KeyRef{}, "ES256", "", nil,
			store, jwtSvc, userSvc, creds)
		s.ErrorIs(err, ErrPolicy)
		s.Nil(svc)
	})
}

func (s *ServiceTestSuite) TestGetMetadata() {
	s.Run("Success", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(context.Background()).
			Return([]credential.CredentialConfigurationDTO{{Handle: "h", VCT: "v"}}, nil)
		svc := &openid4vciService{cfg: serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i"}, creds: creds}

		md := svc.GetMetadata(context.Background())
		s.Equal(testIssuer, md["credential_issuer"])
		configs := md["credential_configurations_supported"].(map[string]interface{})
		s.Contains(configs, "h")
	})

	s.Run("ListError", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(context.Background()).
			Return(nil, &tidcommon.ServiceError{Code: "boom"})
		svc := &openid4vciService{cfg: serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i"}, creds: creds}

		md := svc.GetMetadata(context.Background())
		s.Equal(testIssuer, md["credential_issuer"])
	})
}

func (s *ServiceTestSuite) TestGenerateNonce() {
	s.Run("Success", func() {
		store := newStatefulStore(s.T())
		svc := &openid4vciService{cfg: serviceConfig{NonceTTL: time.Minute}, store: store}
		nonce, err := svc.GenerateNonce(context.Background())
		s.Require().NoError(err)
		s.NotEmpty(nonce)
		rec, ok := store.GetNonce(context.Background(), nonce)
		s.True(ok)
		s.Require().NotNil(rec)
	})

	s.Run("SaveError", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().SaveNonce(context.Background(), mock.Anything, mock.Anything).Return(errors.New("save failed"))
		svc := &openid4vciService{cfg: serviceConfig{NonceTTL: time.Minute}, store: store}
		_, err := svc.GenerateNonce(context.Background())
		s.Error(err)
	})
}

func (s *ServiceTestSuite) TestRandomToken() {
	a, err := randomToken()
	s.Require().NoError(err)
	b, err := randomToken()
	s.Require().NoError(err)
	s.NotEmpty(a)
	s.NotEqual(a, b)
}

type MetadataTestSuite struct {
	suite.Suite
}

func TestMetadataTestSuite(t *testing.T) {
	suite.Run(t, new(MetadataTestSuite))
}

func (s *MetadataTestSuite) TestBuildMetadataFull() {
	cfg := serviceConfig{
		CredentialIssuer:     testIssuer,
		BaseURL:              "https://issuer.example",
		AuthorizationServers: []string{"https://as.example"},
		BatchSize:            5,
	}
	creds := []credential.CredentialConfigurationDTO{
		{
			Handle: "eudi-pid",
			VCT:    "urn:eudi:pid:1",
			Format: "",
			Name:   "PID",
			Claims: []credential.ClaimMapping{
				{Name: "given_name", DisplayName: "Given Name"},
				{Name: "no_display"},
			},
			Display: &credential.CredentialDisplay{Locale: "en", LogoURI: "https://logo"},
		},
	}

	md := buildMetadata(cfg, creds)
	s.Equal(testIssuer, md["credential_issuer"])
	s.Equal("https://issuer.example"+credentialPath, md["credential_endpoint"])
	s.Equal("https://issuer.example"+noncePath, md["nonce_endpoint"])
	s.Equal([]string{"https://as.example"}, md["authorization_servers"])
	s.Equal(map[string]interface{}{"batch_size": 5}, md["batch_credential_issuance"])

	configs := md["credential_configurations_supported"].(map[string]interface{})
	entry := configs["eudi-pid"].(map[string]interface{})
	s.Equal(credential.DefaultCredentialFormat, entry["format"])
	s.Equal("eudi-pid", entry["scope"])
	s.Equal("urn:eudi:pid:1", entry["vct"])
	s.NotNil(entry["display"])
	s.NotNil(entry["claims"])
}

func (s *MetadataTestSuite) TestBuildMetadataMinimal() {
	cfg := serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i", BatchSize: 1}
	creds := []credential.CredentialConfigurationDTO{
		{Handle: "h", VCT: "v", Format: "dc+sd-jwt"},
	}
	md := buildMetadata(cfg, creds)
	_, hasAuth := md["authorization_servers"]
	s.False(hasAuth)
	_, hasBatch := md["batch_credential_issuance"]
	s.False(hasBatch)

	configs := md["credential_configurations_supported"].(map[string]interface{})
	entry := configs["h"].(map[string]interface{})
	_, hasDisplay := entry["display"]
	s.False(hasDisplay)
	_, hasClaims := entry["claims"]
	s.False(hasClaims)
}

func (s *MetadataTestSuite) TestCredentialClaims() {
	out := credentialClaims([]credential.ClaimMapping{
		{Name: "given_name", DisplayName: "Given Name"},
		{Name: "skip"},
	})
	s.Require().NotNil(out)
	s.Contains(out, "given_name")
	s.NotContains(out, "skip")

	s.Nil(credentialClaims([]credential.ClaimMapping{{Name: "skip"}}))
	s.Nil(credentialClaims(nil))
}

func (s *MetadataTestSuite) TestCredentialDisplay() {
	s.Nil(credentialDisplay("", "", nil))
	s.Nil(credentialDisplay("", "", &credential.CredentialDisplay{}))

	out := credentialDisplay(
		"PID", "A PID credential", &credential.CredentialDisplay{Locale: "en", LogoURI: "https://logo"},
	)
	s.Require().Len(out, 1)
	entry := out[0].(map[string]interface{})
	s.Equal("PID", entry["name"])
	s.Equal("A PID credential", entry["description"])
	s.Equal("en", entry["locale"])
	s.Equal(map[string]interface{}{"uri": "https://logo"}, entry["logo"])
}

type OfferTestSuite struct {
	suite.Suite
}

func TestOfferTestSuite(t *testing.T) {
	suite.Run(t, new(OfferTestSuite))
}

func (s *OfferTestSuite) newOfferStore() *openID4VCIStoreInterfaceMock {
	m := newOpenID4VCIStoreInterfaceMock(s.T())
	offers := map[string]*offerRecord{}
	m.EXPECT().SaveOffer(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string, rec *offerRecord) error {
			offers[id] = rec
			return nil
		}).Maybe()
	m.EXPECT().GetOffer(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (*offerRecord, bool) {
			rec, ok := offers[id]
			return rec, ok
		}).Maybe()
	return m
}

func (s *OfferTestSuite) TestGenerateCredentialOfferSuccess() {
	ctx := context.Background()
	store := s.newOfferStore()
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	svc := &openid4vciService{
		cfg:   serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://issuer.example"},
		store: store,
		creds: creds,
	}

	offer, deepLink, err := svc.GenerateCredentialOffer(ctx, "eudi-pid")
	s.Require().NoError(err)
	s.Equal(testIssuer, offer["credential_issuer"])
	s.Equal([]string{"eudi-pid"}, offer["credential_configuration_ids"])
	s.Contains(deepLink, credentialOfferScheme)
	s.Contains(deepLink, "credential_offer_uri=")
}

func (s *OfferTestSuite) TestGenerateCredentialOfferUnknownConfig() {
	ctx := context.Background()
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "missing").
		Return(nil, &tidcommon.ServiceError{Code: "not-found"})
	svc := &openid4vciService{cfg: serviceConfig{CredentialIssuer: testIssuer}, creds: creds}

	_, _, err := svc.GenerateCredentialOffer(ctx, "missing")
	s.ErrorIs(err, ErrUnsupportedCredential)
}

func (s *OfferTestSuite) TestGenerateCredentialOfferStoreError() {
	ctx := context.Background()
	store := newOpenID4VCIStoreInterfaceMock(s.T())
	store.EXPECT().SaveOffer(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("store failed"))
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	svc := &openid4vciService{
		cfg:   serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i"},
		store: store,
		creds: creds,
	}

	_, _, err := svc.GenerateCredentialOffer(ctx, "eudi-pid")
	s.ErrorIs(err, ErrIssuance)
}

func (s *OfferTestSuite) TestGetCredentialOffer() {
	ctx := context.Background()

	s.Run("Success", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().GetOffer(ctx, "o1").Return(
			&offerRecord{
				Offer: map[string]interface{}{"k": "v"}, ExpiresAt: time.Now().Add(time.Minute),
			}, true)
		svc := &openid4vciService{store: store}
		offer, err := svc.GetCredentialOffer(ctx, "o1")
		s.Require().NoError(err)
		s.Equal("v", offer["k"])
	})

	s.Run("NotFound", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().GetOffer(ctx, "missing").Return(nil, false)
		svc := &openid4vciService{store: store}
		_, err := svc.GetCredentialOffer(ctx, "missing")
		s.ErrorIs(err, ErrUnsupportedCredential)
	})

	s.Run("Expired", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().GetOffer(ctx, "old").Return(
			&offerRecord{Offer: map[string]interface{}{}, ExpiresAt: time.Now().Add(-time.Minute)}, true)
		svc := &openid4vciService{store: store}
		_, err := svc.GetCredentialOffer(ctx, "old")
		s.ErrorIs(err, ErrUnsupportedCredential)
	})
}

const testIssuer = "https://issuer.example"

// newStatefulStore returns a openID4VCIStoreInterface mock backed by an in-memory
// map, so tests can seed nonces and observe consumption across a round-trip.
func newStatefulStore(t *testing.T) *openID4VCIStoreInterfaceMock {
	t.Helper()
	m := newOpenID4VCIStoreInterfaceMock(t)
	entries := map[string]*nonceRecord{}
	m.EXPECT().SaveNonce(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, nonce string, rec *nonceRecord) error {
			entries[nonce] = rec
			return nil
		}).Maybe()
	m.EXPECT().GetNonce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, nonce string) (*nonceRecord, bool) {
			rec, ok := entries[nonce]
			return rec, ok
		}).Maybe()
	m.EXPECT().DeleteNonce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, nonce string) error {
			delete(entries, nonce)
			return nil
		}).Maybe()
	return m
}

// signProofJWT builds a signed OpenID4VCI holder proof JWT (typ
// openid4vci-proof+jwt) carrying key's public JWK in the header and the given
// audience/nonce/iat in the payload, signed ES256 in JWS P1363 form.
func signProofJWT(t *testing.T, key *ecdsa.PrivateKey, aud, nonce string, iat time.Time) string {
	t.Helper()
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.FillBytes(make([]byte, 32))),
		"y":   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.FillBytes(make([]byte, 32))),
	}
	header := map[string]interface{}{"alg": "ES256", "typ": proofType, "jwk": jwk}
	payload := map[string]interface{}{"aud": aud, "nonce": nonce, "iat": iat.Unix()}

	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)

	p1363, err := cryptolib.Generate([]byte(signingInput), cryptolib.ECDSASHA256, key)
	if err != nil {
		t.Fatalf("sign proof: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(p1363)
}

func newTestService(t *testing.T, store openID4VCIStoreInterface) *openid4vciService {
	t.Helper()
	return &openid4vciService{
		cfg:   serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		store: store,
	}
}

// encodeJWT assembles a compact JWS from a header and payload, appending sig as
// the (already base64url-encoded) signature segment. Useful for crafting proofs
// whose header/payload exercise specific validation branches.
func encodeJWT(t *testing.T, header, payload map[string]interface{}, sig string) string {
	t.Helper()
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(hb) + "." +
		base64.RawURLEncoding.EncodeToString(pb) + "." + sig
}

func validJWK(key *ecdsa.PrivateKey) map[string]interface{} {
	return map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.FillBytes(make([]byte, 32))),
		"y":   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.FillBytes(make([]byte, 32))),
	}
}

type CredentialTestSuite struct {
	suite.Suite
}

func TestCredentialTestSuite(t *testing.T) {
	suite.Run(t, new(CredentialTestSuite))
}

func (s *CredentialTestSuite) TestScopeString() {
	s.Equal("a b", scopeString(map[string]interface{}{"scope": "a b"}))
	s.Equal("a b", scopeString(map[string]interface{}{"scope": []interface{}{"a", "b"}}))
	s.Equal("a", scopeString(map[string]interface{}{"scope": []interface{}{"a", 1}}))
	s.Equal("", scopeString(map[string]interface{}{"scope": 42}))
	s.Equal("", scopeString(map[string]interface{}{}))
}

func (s *CredentialTestSuite) TestDtoToCredentialConfig() {
	validity := 3600
	cfg := dtoToCredentialConfig(credential.CredentialConfigurationDTO{
		Format:          "",
		VCT:             "urn:v",
		Claims:          []credential.ClaimMapping{{Name: "given_name"}, {Name: "family_name"}},
		ValiditySeconds: &validity,
	})
	s.Equal(credential.DefaultCredentialFormat, cfg.Format)
	s.Equal("urn:v", cfg.VCT)
	s.Equal([]string{"given_name", "family_name"}, cfg.SDClaims)
	s.Equal(time.Hour, cfg.Validity)

	cfg = dtoToCredentialConfig(credential.CredentialConfigurationDTO{Format: "custom", VCT: "v"})
	s.Equal("custom", cfg.Format)
	s.Zero(cfg.Validity)
}

func (s *CredentialTestSuite) TestAuthorizedCredentialByConfigID() {
	ctx := context.Background()

	s.Run("Success", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &openid4vciService{cfg: serviceConfig{}, creds: creds}
		cfg, err := svc.authorizedCredential(ctx, "eudi-pid", nil)
		s.Require().NoError(err)
		s.Equal("v", cfg.VCT)
	})

	s.Run("Unknown", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "missing").
			Return(nil, &tidcommon.ServiceError{Code: "x"})
		svc := &openid4vciService{cfg: serviceConfig{}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "missing", nil)
		s.ErrorIs(err, ErrUnsupportedCredential)
	})

	s.Run("ScopeEnforcedNotAuthorized", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &openid4vciService{cfg: serviceConfig{EnforceScope: true}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "eudi-pid", []string{"other"})
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("ScopeEnforcedAuthorized", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &openid4vciService{cfg: serviceConfig{EnforceScope: true}, creds: creds}
		cfg, err := svc.authorizedCredential(ctx, "eudi-pid", []string{"eudi-pid"})
		s.Require().NoError(err)
		s.Equal("v", cfg.VCT)
	})
}

func (s *CredentialTestSuite) TestAuthorizedCredentialByScope() {
	ctx := context.Background()

	s.Run("MatchesScope", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(ctx).Return([]credential.CredentialConfigurationDTO{
			{Handle: "a", VCT: "va"},
			{Handle: "b", VCT: "vb"},
		}, nil)
		svc := &openid4vciService{cfg: serviceConfig{}, creds: creds}
		cfg, err := svc.authorizedCredential(ctx, "", []string{"b"})
		s.Require().NoError(err)
		s.Equal("vb", cfg.VCT)
	})

	s.Run("NoScopeMatch", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(ctx).Return([]credential.CredentialConfigurationDTO{
			{Handle: "a", VCT: "va"},
		}, nil)
		svc := &openid4vciService{cfg: serviceConfig{}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "", []string{"z"})
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("ListError", func() {
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(ctx).Return(nil, &tidcommon.ServiceError{Code: "x"})
		svc := &openid4vciService{cfg: serviceConfig{}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "", []string{"a"})
		s.ErrorIs(err, ErrIssuance)
	})
}

func (s *CredentialTestSuite) TestIssueCredentialErrors() {
	ctx := context.Background()
	svc := &openid4vciService{cfg: serviceConfig{BatchSize: 5}}

	s.Run("MissingToken", func() {
		_, err := svc.IssueCredential(ctx, "", nil)
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("VerifyFails", func() {
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, "tok", "", "").Return(&tidcommon.ServiceError{Code: "x"})
		svc := &openid4vciService{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
		_, err := svc.IssueCredential(ctx, "tok", nil)
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("MissingSubject", func() {
		token := makeToken(s.T(), map[string]any{})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		svc := &openid4vciService{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
		_, err := svc.IssueCredential(ctx, token, []byte("{}"))
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("BadBody", func() {
		token := makeToken(s.T(), map[string]any{"sub": "u1"})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		svc := &openid4vciService{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
		_, err := svc.IssueCredential(ctx, token, []byte("not-json"))
		s.ErrorIs(err, ErrInvalidRequest)
	})

	s.Run("MissingProof", func() {
		token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &openid4vciService{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc, creds: creds}
		body, _ := json.Marshal(CredentialRequest{CredentialConfigurationID: "eudi-pid"})
		_, err := svc.IssueCredential(ctx, token, body)
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("BatchSizeExceeded", func() {
		token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &openid4vciService{cfg: serviceConfig{BatchSize: 1}, jwtService: jwtSvc, creds: creds}
		body, _ := json.Marshal(CredentialRequest{
			CredentialConfigurationID: "eudi-pid",
			Proofs:                    &Proofs{JWT: []string{"a", "b"}},
		})
		_, err := svc.IssueCredential(ctx, token, body)
		s.ErrorIs(err, ErrInvalidRequest)
	})
}

func (s *CredentialTestSuite) TestIssueCredentialBadTokenPayload() {
	ctx := context.Background()
	token := "e30.!!!.sig"
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	svc := &openid4vciService{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
	_, err := svc.IssueCredential(ctx, token, []byte("{}"))
	s.ErrorIs(err, ErrInvalidToken)
}

func (s *CredentialTestSuite) TestIssueCredentialUnauthorizedCredential() {
	ctx := context.Background()
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "missing"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "missing").
		Return(nil, &tidcommon.ServiceError{Code: "x"})
	svc := &openid4vciService{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc, creds: creds}
	body, _ := json.Marshal(CredentialRequest{CredentialConfigurationID: "missing"})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrUnsupportedCredential)
}

func (s *CredentialTestSuite) TestIssueCredentialVerifyProofsError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	svc := &openid4vciService{
		cfg:        serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		store:      store,
		jwtService: jwtSvc,
		creds:      creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, "unknown-nonce", time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrInvalidNonce)
}

func (s *CredentialTestSuite) TestIssueCredentialResolveClaimsError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	nonce := "n"
	s.Require().NoError(store.SaveNonce(ctx, nonce, &nonceRecord{ExpiresAt: time.Now().Add(time.Minute)}))
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).Return(nil, &tidcommon.ServiceError{Code: "x"})
	svc := &openid4vciService{
		cfg:         serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		store:       store,
		jwtService:  jwtSvc,
		userService: userSvc,
		creds:       creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, nonce, time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrUserNotFound)
}

func (s *CredentialTestSuite) TestIssueCredentialSignError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	nonce := "n"
	s.Require().NoError(store.SaveNonce(ctx, nonce, &nonceRecord{ExpiresAt: time.Now().Add(time.Minute)}))
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)

	validity := 3600
	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{
			Handle: "eudi-pid", VCT: "urn:v", Format: credential.DefaultCredentialFormat,
			ValiditySeconds: &validity,
		}, nil)
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	attrs, _ := json.Marshal(map[string]interface{}{})
	userSvc.EXPECT().GetUser(ctx, "u1", false).Return(&user.User{ID: "u1", Attributes: attrs}, nil)

	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().Sign(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("sign failed"))

	svc := &openid4vciService{
		cfg:            serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		cryptoProvider: provider,
		signingAlg:     "ES256",
		store:          store,
		jwtService:     jwtSvc,
		userService:    userSvc,
		creds:          creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, nonce, time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrIssuance)
}

func (s *CredentialTestSuite) TestIssueCredentialSuccess() {
	ctx := context.Background()

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().Sign(mock.Anything, mock.Anything, "ES256", mock.Anything).
		RunAndReturn(func(_ context.Context, _ kmprovider.KeyRef, _ string, content []byte) ([]byte, error) {
			digest := sha256.Sum256(content)
			return ecdsa.SignASN1(rand.Reader, key, digest[:])
		}).Maybe()

	store := newStatefulStore(s.T())
	nonce := "the-nonce"
	s.Require().NoError(store.SaveNonce(ctx, nonce, &nonceRecord{ExpiresAt: time.Now().Add(time.Minute)}))

	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)

	creds := credentialmock.NewCredentialConfigurationServiceInterfaceMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{
			Handle: "eudi-pid", VCT: "urn:v", Format: credential.DefaultCredentialFormat,
			Claims: []credential.ClaimMapping{{Name: "given_name"}},
		}, nil)

	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	attrs, _ := json.Marshal(map[string]interface{}{"given_name": "Ada"})
	userSvc.EXPECT().GetUser(ctx, "u1", false).Return(&user.User{ID: "u1", Attributes: attrs}, nil)

	svc := &openid4vciService{
		cfg:            serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		cryptoProvider: provider,
		signingKeyRef:  kmprovider.KeyRef{KeyID: "kid"},
		signingAlg:     "ES256",
		kid:            "kid",
		x5c:            []string{base64.StdEncoding.EncodeToString([]byte("cert"))},
		store:          store,
		jwtService:     jwtSvc,
		userService:    userSvc,
		creds:          creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, nonce, time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})

	resp, err := svc.IssueCredential(ctx, token, body)
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().Len(resp.Credentials, 1)
	s.NotEmpty(resp.Credentials[0].Credential)
}

type ClaimsTestSuite struct {
	suite.Suite
}

func TestClaimsTestSuite(t *testing.T) {
	suite.Run(t, new(ClaimsTestSuite))
}

func (s *ClaimsTestSuite) TestResolveClaimsSelectsConfiguredClaims() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	attrs, _ := json.Marshal(map[string]interface{}{
		"given_name":  "Ada",
		"family_name": "Lovelace",
		"extra":       "ignored",
	})
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(&user.User{ID: "u1", Attributes: attrs}, nil)
	svc := &openid4vciService{userService: userSvc}

	claims, err := svc.resolveClaims(ctx, "u1", []string{"given_name", "family_name", "missing"})
	s.Require().NoError(err)
	s.Equal("Ada", claims["given_name"])
	s.Equal("Lovelace", claims["family_name"])
	s.NotContains(claims, "extra")
	s.NotContains(claims, "missing")
}

func (s *ClaimsTestSuite) TestResolveClaimsUserNotFound() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(nil, &tidcommon.ServiceError{Code: "not-found"})
	svc := &openid4vciService{userService: userSvc}

	_, err := svc.resolveClaims(ctx, "u1", []string{"given_name"})
	s.ErrorIs(err, ErrUserNotFound)
}

func (s *ClaimsTestSuite) TestResolveClaimsBadAttributes() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(&user.User{ID: "u1", Attributes: json.RawMessage("not-json")}, nil)
	svc := &openid4vciService{userService: userSvc}

	_, err := svc.resolveClaims(ctx, "u1", []string{"given_name"})
	s.ErrorIs(err, ErrIssuance)
}

func (s *ClaimsTestSuite) TestResolveClaimsNoAttributes() {
	ctx := context.Background()
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).
		Return(&user.User{ID: "u1"}, nil)
	svc := &openid4vciService{userService: userSvc}

	claims, err := svc.resolveClaims(ctx, "u1", []string{"given_name"})
	s.Require().NoError(err)
	s.Empty(claims)
}

func (s *CredentialTestSuite) TestECDSADERToJWS() {
	der, err := asn1.Marshal(struct{ R, S *big.Int }{big.NewInt(1234567), big.NewInt(7654321)})
	s.Require().NoError(err)

	s.Len(ecdsaDERToJWS(der, "ES256"), 64)
	s.Len(ecdsaDERToJWS(der, "ES384"), 96)
	s.Len(ecdsaDERToJWS(der, "ES512"), 132)

	raw := []byte("not-der-at-all")
	s.Equal(raw, ecdsaDERToJWS(raw, "ES256"))
	s.Equal(der, ecdsaDERToJWS(der, "unknown"))
}

type ProofTestSuite struct {
	suite.Suite
}

func TestProofTestSuite(t *testing.T) {
	suite.Run(t, new(ProofTestSuite))
}

// A batch of proofs (one per holder key) sharing a single c_nonce must yield one
// confirmation JWK per proof, and consume the shared nonce exactly once.
func (s *ProofTestSuite) TestVerifyProofsBatchConsumesSharedNonceOnce() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	nonce := "shared-nonce"
	s.Require().NoError(store.SaveNonce(ctx, nonce, &nonceRecord{ExpiresAt: time.Now().Add(time.Minute)}))
	svc := newTestService(s.T(), store)

	proofs := make([]Proof, 0, 3)
	for i := 0; i < 3; i++ {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		s.Require().NoError(err)
		proofs = append(proofs, Proof{
			ProofType: "jwt",
			JWT:       signProofJWT(s.T(), key, testIssuer, nonce, time.Now()),
		})
	}

	jwks, err := svc.verifyProofs(ctx, proofs)
	s.Require().NoError(err)
	s.Require().Len(jwks, 3)
	for i, jwk := range jwks {
		_, ok := jwk["x"].(string)
		s.True(ok, "proof %d: confirmation JWK missing x coordinate", i)
	}
	_, ok := store.GetNonce(ctx, nonce)
	s.False(ok, "shared c_nonce should have been consumed")
}

// An unknown c_nonce is rejected for the whole batch.
func (s *ProofTestSuite) TestVerifyProofsRejectsUnknownNonce() {
	ctx := context.Background()
	svc := newTestService(s.T(), newStatefulStore(s.T()))

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofs := []Proof{{ProofType: "jwt", JWT: signProofJWT(s.T(), key, testIssuer, "never-issued", time.Now())}}

	_, err := svc.verifyProofs(ctx, proofs)
	s.Error(err, "expected error for unknown c_nonce")
}

func (s *ProofTestSuite) TestCheckProofErrors() {
	svc := newTestService(s.T(), newStatefulStore(s.T()))
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := validJWK(key)

	s.Run("NotJWTProofType", func() {
		_, _, err := svc.checkProof(Proof{ProofType: "cwt", JWT: "x"})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("EmptyJWT", func() {
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: ""})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("UndecodableHeader", func() {
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: "!!!.!!!.sig"})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("WrongTyp", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256", "typ": "wrong", "jwk": jwk},
			map[string]interface{}{}, "AA")
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("MissingJWK", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256", "typ": proofType},
			map[string]interface{}{}, "AA")
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("BadSignature", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256", "typ": proofType, "jwk": jwk},
			map[string]interface{}{"aud": testIssuer, "nonce": "n", "iat": float64(time.Now().Unix())},
			base64.RawURLEncoding.EncodeToString(make([]byte, 64)))
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("AudienceMismatch", func() {
		jwt := signProofJWT(s.T(), key, "https://other", "n", time.Now())
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("BadIat", func() {
		jwt := signProofJWT(s.T(), key, testIssuer, "n", time.Now().Add(2*time.Minute))
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("MissingNonce", func() {
		jwt := signProofJWT(s.T(), key, testIssuer, "", time.Now())
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidNonce)
	})
}

func (s *ProofTestSuite) TestCheckProofUndecodablePayload() {
	svc := newTestService(s.T(), newStatefulStore(s.T()))
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := validJWK(key)
	header := map[string]interface{}{"alg": "ES256", "typ": proofType, "jwk": jwk}
	hb, _ := json.Marshal(header)
	headerSeg := base64.RawURLEncoding.EncodeToString(hb)
	signingInput := headerSeg + "." + "!!!"
	p1363, err := cryptolib.Generate([]byte(signingInput), cryptolib.ECDSASHA256, key)
	s.Require().NoError(err)
	jwt := signingInput + "." + base64.RawURLEncoding.EncodeToString(p1363)

	_, _, err = svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
	s.ErrorIs(err, ErrInvalidProof)
}

func (s *ProofTestSuite) TestCheckProofIat() {
	svc := newTestService(s.T(), newStatefulStore(s.T()))

	s.Run("MissingIat", func() {
		s.ErrorIs(svc.checkProofIat(map[string]interface{}{}), ErrInvalidProof)
	})

	s.Run("Future", func() {
		payload := map[string]interface{}{"iat": float64(time.Now().Add(2 * time.Minute).Unix())}
		s.ErrorIs(svc.checkProofIat(payload), ErrInvalidProof)
	})

	s.Run("TooOld", func() {
		payload := map[string]interface{}{"iat": float64(time.Now().Add(-2 * time.Minute).Unix())}
		s.ErrorIs(svc.checkProofIat(payload), ErrInvalidProof)
	})

	s.Run("Valid", func() {
		payload := map[string]interface{}{"iat": float64(time.Now().Unix())}
		s.NoError(svc.checkProofIat(payload))
	})
}

func (s *ProofTestSuite) TestConsumeNonceExpired() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	s.Require().NoError(store.SaveNonce(ctx, "old", &nonceRecord{ExpiresAt: time.Now().Add(-time.Minute)}))
	svc := newTestService(s.T(), store)

	err := svc.consumeNonce(ctx, "old")
	s.ErrorIs(err, ErrInvalidNonce)
	_, ok := store.GetNonce(ctx, "old")
	s.False(ok, "expired nonce should still be deleted")
}

func (s *ProofTestSuite) TestVerifyProofsConsumeNonceError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	svc := newTestService(s.T(), store)

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofs := []Proof{{ProofType: "jwt", JWT: signProofJWT(s.T(), key, testIssuer, "unknown", time.Now())}}

	_, err := svc.verifyProofs(ctx, proofs)
	s.ErrorIs(err, ErrInvalidNonce)
}

func (s *ProofTestSuite) TestVerifyJWSWithJWKErrors() {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := validJWK(key)

	s.Run("BadFormat", func() {
		s.Error(verifyJWSWithJWK("a.b", jwk))
	})

	s.Run("UndecodableHeader", func() {
		s.Error(verifyJWSWithJWK("!!!.payload.sig", jwk))
	})

	s.Run("UnsupportedAlg", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "none"},
			map[string]interface{}{}, "AA")
		s.Error(verifyJWSWithJWK(jwt, jwk))
	})

	s.Run("BadSignatureEncoding", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256"},
			map[string]interface{}{}, "!!!")
		s.Error(verifyJWSWithJWK(jwt, jwk))
	})

	s.Run("ECKeyError", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256"},
			map[string]interface{}{}, base64.RawURLEncoding.EncodeToString(make([]byte, 64)))
		s.Error(verifyJWSWithJWK(jwt, map[string]interface{}{"kty": "EC", "crv": "P-256"}))
	})

	s.Run("NonECKeyError", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "RS256"},
			map[string]interface{}{}, base64.RawURLEncoding.EncodeToString(make([]byte, 8)))
		s.Error(verifyJWSWithJWK(jwt, map[string]interface{}{"kty": "RSA"}))
	})
}

func (s *ProofTestSuite) TestECJWKToECDSAPublicKey() {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	x := base64.RawURLEncoding.EncodeToString(key.PublicKey.X.FillBytes(make([]byte, 32)))
	y := base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.FillBytes(make([]byte, 32)))

	s.Run("MissingCoords", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256"})
		s.Error(err)
	})

	s.Run("UnsupportedCurve", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-999", "x": x, "y": y})
		s.Error(err)
	})

	s.Run("BadX", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": "!!!", "y": y})
		s.Error(err)
	})

	s.Run("BadY", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": x, "y": "!!!"})
		s.Error(err)
	})

	s.Run("OversizedCoord", func() {
		big := base64.RawURLEncoding.EncodeToString(make([]byte, 40))
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": big, "y": y})
		s.Error(err)
	})

	s.Run("InvalidPoint", func() {
		zero := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": zero, "y": zero})
		s.Error(err)
	})

	s.Run("ValidP384", func() {
		k384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		pub, err := ecJWKToECDSAPublicKey(map[string]interface{}{
			"crv": "P-384",
			"x":   base64.RawURLEncoding.EncodeToString(k384.PublicKey.X.FillBytes(make([]byte, 48))),
			"y":   base64.RawURLEncoding.EncodeToString(k384.PublicKey.Y.FillBytes(make([]byte, 48))),
		})
		s.Require().NoError(err)
		s.NotNil(pub)
	})
}

func (s *ProofTestSuite) TestHolderProofsPrefersBatchThenSingle() {
	tests := []struct {
		name string
		req  CredentialRequest
		want int
	}{
		{"batch", CredentialRequest{Proofs: &Proofs{JWT: []string{"a", "b", "c"}}}, 3},
		{"single", CredentialRequest{Proof: Proof{ProofType: "jwt", JWT: "a"}}, 1},
		{"batch over single", CredentialRequest{
			Proof:  Proof{ProofType: "jwt", JWT: "single"},
			Proofs: &Proofs{JWT: []string{"a", "b"}},
		}, 2},
		{"empty", CredentialRequest{}, 0},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.want, len(tc.req.holderProofs()))
		})
	}
}
