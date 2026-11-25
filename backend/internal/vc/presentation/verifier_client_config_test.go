// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/vc/truststore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/vc/truststoremock"
)

type VerifierClientConfigTestSuite struct {
	suite.Suite
}

func TestVerifierClientConfigTestSuite(t *testing.T) {
	suite.Run(t, new(VerifierClientConfigTestSuite))
}

func (suite *VerifierClientConfigTestSuite) TestGetVCVerifierClientConfigByID() {
	svc, store := newTestDefinitionService(suite.T())
	anchors := map[string]*x509.Certificate{"AQIDBA": {}}
	trust := truststoremock.NewTrustStoreInterfaceMock(suite.T())
	trust.EXPECT().AnchorsFor([]string{"root-a"}).Return(anchors, nil)
	svc.trustStore = trust
	ctx := context.Background()
	enforce := true
	suite.Require().NoError(store.CreatePresentationDefinition(ctx, PresentationDefinitionDTO{
		ID:                   "pd-1",
		Handle:               "eudi-pid",
		VCT:                  "urn:eudi:pid:de:1",
		Format:               DefaultCredentialFormat,
		MandatoryClaims:      []string{"given_name"},
		OptionalClaims:       []string{"address.locality"},
		ClaimValues:          map[string][]string{"given_name": {"Erika", "Max"}},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   []string{"root-a"},
	}))

	cfg, svcErr := svc.GetVCVerifierClientConfigByID(ctx, "eudi-pid")
	suite.Require().Nil(svcErr)
	suite.Equal(&providers.VCVerifierClientConfig{
		VerifierID: "eudi-pid",
		DCQL: providers.DCQLQuery{Credentials: []providers.DCQLCredentialQuery{{
			ID:     "eudi-pid",
			Format: DefaultCredentialFormat,
			Meta:   &providers.DCQLMeta{VCTValues: []string{"urn:eudi:pid:de:1"}},
			Claims: []providers.DCQLClaimsQuery{
				{ID: "claim_0", Path: []interface{}{"given_name"}, Values: []interface{}{"Erika", "Max"}},
				{ID: "claim_1", Path: []interface{}{"address", "locality"}},
			},
			ClaimSets:          [][]string{{"claim_0", "claim_1"}, {"claim_0"}},
			TrustedAuthorities: []providers.DCQLTrustedAuthority{{Type: "aki", Values: []string{"AQIDBA"}}},
		}}},
		TrustedAuthorities:   anchors,
		EnforceTrustedIssuer: &enforce,
	}, cfg)
}

func (suite *VerifierClientConfigTestSuite) TestRequestedClaimsTakePrecedence() {
	cfg, err := toVCVerifierClientConfig(PresentationDefinitionDTO{
		Handle:          "pid",
		VCT:             "urn:vct",
		Format:          DefaultCredentialFormat,
		RequestedClaims: []string{"birthdate"},
		MandatoryClaims: []string{"given_name"},
	}, nil, config.OpenID4VPConfig{})
	suite.Require().NoError(err)
	suite.Equal([]providers.DCQLClaimsQuery{{Path: []interface{}{"birthdate"}}}, cfg.DCQL.Credentials[0].Claims)
	suite.Nil(cfg.DCQL.Credentials[0].TrustedAuthorities)
}

func (suite *VerifierClientConfigTestSuite) TestTrustedAuthoritiesEmittedAsSortedAKI() {
	cfg, err := toVCVerifierClientConfig(PresentationDefinitionDTO{
		Handle:          "pid",
		VCT:             "urn:vct",
		Format:          DefaultCredentialFormat,
		MandatoryClaims: []string{"given_name"},
	}, map[string]*x509.Certificate{"kid-b": {}, "kid-a": {}}, config.OpenID4VPConfig{})
	suite.Require().NoError(err)
	suite.Equal([]providers.DCQLTrustedAuthority{{Type: "aki", Values: []string{"kid-a", "kid-b"}}},
		cfg.DCQL.Credentials[0].TrustedAuthorities)
}

func (suite *VerifierClientConfigTestSuite) TestGetVCVerifierClientConfigByIDErrors() {
	ctx := context.Background()

	suite.Run("empty verifier id", func() {
		svc, _ := newTestDefinitionService(suite.T())
		_, svcErr := svc.GetVCVerifierClientConfigByID(ctx, " ")
		suite.Require().NotNil(svcErr)
		suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)
		suite.Equal(tidcommon.ClientErrorType, svcErr.Type)
	})

	suite.Run("unknown handle", func() {
		svc, _ := newTestDefinitionService(suite.T())
		_, svcErr := svc.GetVCVerifierClientConfigByID(ctx, "missing")
		suite.Require().NotNil(svcErr)
		suite.Equal(ErrorDefinitionNotFound.Code, svcErr.Code)
		suite.Equal(tidcommon.ClientErrorType, svcErr.Type)
	})

	suite.Run("store failure", func() {
		svc := newPresentationDefinitionService(
			newErroringDefinitionStore(suite.T(), errors.New("db down")),
			nil, nil, config.OpenID4VPConfig{}, "")
		_, svcErr := svc.GetVCVerifierClientConfigByID(ctx, "eudi-pid")
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})

	suite.Run("trusted authorities without trust store", func() {
		svc, store := newTestDefinitionService(suite.T())
		suite.Require().NoError(store.CreatePresentationDefinition(ctx, PresentationDefinitionDTO{
			ID: "pd-1", Handle: "eudi-pid", VCT: "urn:vct", Format: DefaultCredentialFormat,
			MandatoryClaims: []string{"given_name"}, TrustedAuthorities: []string{"root-a"},
		}))
		_, svcErr := svc.GetVCVerifierClientConfigByID(ctx, "eudi-pid")
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})

	suite.Run("unknown trusted authority", func() {
		svc, store := newTestDefinitionService(suite.T())
		trust := truststoremock.NewTrustStoreInterfaceMock(suite.T())
		trust.EXPECT().AnchorsFor([]string{"gone"}).Return(nil, truststore.ErrUnknownAnchor)
		svc.trustStore = trust
		suite.Require().NoError(store.CreatePresentationDefinition(ctx, PresentationDefinitionDTO{
			ID: "pd-1", Handle: "eudi-pid", VCT: "urn:vct", Format: DefaultCredentialFormat,
			MandatoryClaims: []string{"given_name"}, TrustedAuthorities: []string{"gone"},
		}))
		_, svcErr := svc.GetVCVerifierClientConfigByID(ctx, "eudi-pid")
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})

	suite.Run("unreadable registration certificate", func() {
		store := newStatefulDefinitionStore(suite.T())
		cfg := config.OpenID4VPConfig{}
		cfg.RegistrationCertFile = filepath.Join(suite.T().TempDir(), "missing.jwt")
		svc := newPresentationDefinitionService(store, nil, nil, cfg, "")
		suite.Require().NoError(store.CreatePresentationDefinition(ctx, PresentationDefinitionDTO{
			ID: "pd-1", Handle: "eudi-pid", VCT: "urn:vct", Format: DefaultCredentialFormat,
			MandatoryClaims: []string{"given_name"},
		}))
		_, svcErr := svc.GetVCVerifierClientConfigByID(ctx, "eudi-pid")
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})

	suite.Run("malformed claim path", func() {
		svc, store := newTestDefinitionService(suite.T())
		suite.Require().NoError(store.CreatePresentationDefinition(ctx, PresentationDefinitionDTO{
			ID: "pd-1", Handle: "eudi-pid", VCT: "urn:vct", Format: DefaultCredentialFormat,
			RequestedClaims: []string{"address."},
		}))
		_, svcErr := svc.GetVCVerifierClientConfigByID(ctx, "eudi-pid")
		suite.Require().NotNil(svcErr)
		suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	})
}

func (suite *VerifierClientConfigTestSuite) TestDCQLClaimSets() {
	suite.Nil(dcqlClaimSets(2, 0))
	suite.Equal([][]string{{"claim_0", "claim_1", "claim_2"}, {"claim_0"}}, dcqlClaimSets(1, 2))
	suite.Equal([][]string{{"claim_0", "claim_1"}, {"claim_0"}, {"claim_1"}}, dcqlClaimSets(0, 2))
}

func (suite *VerifierClientConfigTestSuite) TestMandatoryOnlyClaimsHaveNoClaimSets() {
	cfg, err := toVCVerifierClientConfig(PresentationDefinitionDTO{
		Handle:          "pid",
		VCT:             "urn:vct",
		Format:          DefaultCredentialFormat,
		MandatoryClaims: []string{"given_name"},
	}, nil, config.OpenID4VPConfig{})
	suite.Require().NoError(err)
	suite.Equal([]providers.DCQLClaimsQuery{{Path: []interface{}{"given_name"}}}, cfg.DCQL.Credentials[0].Claims)
	suite.Nil(cfg.DCQL.Credentials[0].ClaimSets)
}

func (suite *VerifierClientConfigTestSuite) TestDCQLMeta() {
	suite.Equal(&providers.DCQLMeta{VCTValues: []string{"urn:eudi:pid:de:1"}},
		dcqlMeta(DefaultCredentialFormat, "urn:eudi:pid:de:1"))
	suite.Nil(dcqlMeta("mso_mdoc", "urn:eudi:pid:de:1"))
}

func (suite *VerifierClientConfigTestSuite) TestClaimPathToSegments() {
	segs, err := claimPathToSegments("address.locality.street")
	suite.Require().NoError(err)
	suite.Equal([]interface{}{"address", "locality", "street"}, segs)

	for _, path := range []string{"", "address.", ".locality", "a..b"} {
		_, err := claimPathToSegments(path)
		suite.Error(err, "path %q", path)
	}
}

func (suite *VerifierClientConfigTestSuite) TestLoadVerifierInfoEmptyPath() {
	got, err := loadVerifierInfo("", "")
	suite.Require().NoError(err)
	suite.Nil(got)
}

func (suite *VerifierClientConfigTestSuite) TestLoadVerifierInfoReadsFile() {
	path := filepath.Join(suite.T().TempDir(), "registration.jwt")
	suite.Require().NoError(os.WriteFile(path, []byte("  eyJhbGciOiJFUzI1NiJ9.payload.sig  \n"), 0o600))

	got, err := loadVerifierInfo(path, "")
	suite.Require().NoError(err)
	suite.Require().Len(got, 1)
	entry := got[0].(map[string]interface{})
	suite.Equal("registration_cert", entry["format"])
	suite.Equal("eyJhbGciOiJFUzI1NiJ9.payload.sig", entry["data"])
}

func (suite *VerifierClientConfigTestSuite) TestLoadVerifierInfoResolvesRelativePath() {
	home := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(home, "regcert.jwt"), []byte("jwt-data"), 0o600))

	got, err := loadVerifierInfo("regcert.jwt", home)
	suite.Require().NoError(err)
	suite.Require().Len(got, 1)
	suite.Equal("jwt-data", got[0].(map[string]interface{})["data"])
}

func (suite *VerifierClientConfigTestSuite) TestLoadVerifierInfoMissingFile() {
	_, err := loadVerifierInfo(filepath.Join(suite.T().TempDir(), "missing"), "")
	suite.Error(err)
}

func (suite *VerifierClientConfigTestSuite) TestLoadVerifierInfoEmptyFile() {
	path := filepath.Join(suite.T().TempDir(), "empty.jwt")
	suite.Require().NoError(os.WriteFile(path, []byte("   \n"), 0o600))
	_, err := loadVerifierInfo(path, "")
	suite.Error(err)
}
