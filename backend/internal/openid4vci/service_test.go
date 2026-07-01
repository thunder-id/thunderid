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
	"errors"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/openid4vci/credential"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

type ServiceTestSuite struct {
	suite.Suite
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) TestNewOpenID4VCIService() {
	signer := &issuerSigner{}
	store := newOpenID4VCIStoreInterfaceMock(s.T())
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	creds := newCredentialReaderMock(s.T())

	s.Run("Success applies defaults", func() {
		svc, err := newOpenID4VCIService(
			serviceConfig{CredentialIssuer: testIssuer}, signer, store, jwtSvc, userSvc, creds)
		s.Require().NoError(err)
		s.Require().NotNil(svc)
		impl := svc.(*service)
		s.Equal(defaultNonceTTL, impl.cfg.NonceTTL)
		s.Equal(defaultProofMaxAge, impl.cfg.ProofMaxAge)
		s.Equal(defaultCredValidity, impl.cfg.CredentialValidity)
		s.Equal(defaultBatchSize, impl.cfg.BatchSize)
	})

	s.Run("MissingDependency", func() {
		svc, err := newOpenID4VCIService(
			serviceConfig{CredentialIssuer: testIssuer}, nil, store, jwtSvc, userSvc, creds)
		s.ErrorIs(err, ErrPolicy)
		s.Nil(svc)
	})

	s.Run("MissingCredentialIssuer", func() {
		svc, err := newOpenID4VCIService(
			serviceConfig{}, signer, store, jwtSvc, userSvc, creds)
		s.ErrorIs(err, ErrPolicy)
		s.Nil(svc)
	})
}

func (s *ServiceTestSuite) TestGetMetadata() {
	s.Run("Success", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(context.Background()).
			Return([]credential.CredentialConfigurationDTO{{Handle: "h", VCT: "v"}}, nil)
		svc := &service{cfg: serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i"}, creds: creds}

		md := svc.GetMetadata(context.Background())
		s.Equal(testIssuer, md["credential_issuer"])
		configs := md["credential_configurations_supported"].(map[string]interface{})
		s.Contains(configs, "h")
	})

	s.Run("ListError", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(context.Background()).
			Return(nil, &tidcommon.ServiceError{Code: "boom"})
		svc := &service{cfg: serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i"}, creds: creds}

		md := svc.GetMetadata(context.Background())
		s.Equal(testIssuer, md["credential_issuer"])
	})
}

func (s *ServiceTestSuite) TestGenerateNonce() {
	s.Run("Success", func() {
		store := newStatefulStore(s.T())
		svc := &service{cfg: serviceConfig{NonceTTL: time.Minute}, store: store}
		nonce, err := svc.GenerateNonce(context.Background())
		s.Require().NoError(err)
		s.NotEmpty(nonce)
		rec, ok := store.GetNonce(context.Background(), nonce)
		s.True(ok)
		s.Require().NotNil(rec)
	})

	s.Run("SaveError", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().SaveNonce(context.Background(), mock.Anything).Return(errors.New("save failed"))
		svc := &service{cfg: serviceConfig{NonceTTL: time.Minute}, store: store}
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
