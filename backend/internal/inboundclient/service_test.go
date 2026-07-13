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

package inboundclient

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	entitytypepkg "github.com/thunder-id/thunderid/internal/entitytype"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/certmock"
	"github.com/thunder-id/thunderid/tests/mocks/consentmock"
	"github.com/thunder-id/thunderid/tests/mocks/design/layoutmock"
	"github.com/thunder-id/thunderid/tests/mocks/design/thememock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowmgtmock"
)

type InboundClientServiceTestSuite struct {
	suite.Suite
}

func TestInboundClientServiceTestSuite(t *testing.T) {
	suite.Run(t, new(InboundClientServiceTestSuite))
}

func (suite *InboundClientServiceTestSuite) SetupTest() {
	sysconfig.ResetServerRuntime()
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", &sysconfig.Config{}))
}

func newServiceForTest(store inboundClientStoreInterface) InboundClientServiceInterface {
	return newInboundClientService(store, transaction.NewNoOpTransactioner(), nil, nil, nil, nil, nil, nil, nil)
}

func newServiceWithCert(certService cert.CertificateServiceInterface) *inboundClientService {
	svc := newInboundClientService(
		nil, transaction.NewNoOpTransactioner(), certService, nil, nil, nil, nil, nil, nil,
	)
	return svc.(*inboundClientService)
}

func validInboundClient() inboundmodel.InboundClient {
	return inboundmodel.InboundClient{
		ID:                        "p1",
		AuthFlowID:                "flow-1",
		RegistrationFlowID:        "reg-1",
		IsRegistrationFlowEnabled: true,
	}
}

func ptrInboundClient() *inboundmodel.InboundClient {
	c := validInboundClient()
	return &c
}

func validOAuthProfile() *providers.OAuthProfile {
	return &providers.OAuthProfile{
		RedirectURIs:            []string{"https://app.example.com/cb"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}
}

func validOAuthProfileData() *providers.OAuthProfile {
	return &providers.OAuthProfile{
		RedirectURIs:            []string{"https://app.example.com/cb"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
	}
}

// ----- Inbound client CRUD -----

func (suite *InboundClientServiceTestSuite) TestCreateInboundClient_RunsValidationBeforePersist() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	svc := newServiceForTest(store)

	p := validOAuthProfile()
	p.GrantTypes = []string{"not_a_real_grant"}

	err := svc.CreateInboundClient(context.Background(), ptrInboundClient(), p, false, "")

	assert.ErrorIs(suite.T(), err, ErrOAuthInvalidGrantType)
}

func (suite *InboundClientServiceTestSuite) TestCreateInboundClient_PersistsBoth() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	store.EXPECT().CreateInboundClient(mock.Anything, mock.Anything).Return(nil)
	store.EXPECT().CreateOAuthProfile(mock.Anything, "p1", mock.Anything).Return(nil)

	svc := newServiceForTest(store)
	err := svc.CreateInboundClient(context.Background(), ptrInboundClient(),
		validOAuthProfile(), true, "")

	assert.NoError(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestCreateInboundClient_PersistsClientOnlyWhenOAuthNil() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	store.EXPECT().CreateInboundClient(mock.Anything, mock.Anything).Return(nil)

	svc := newServiceForTest(store)
	err := svc.CreateInboundClient(context.Background(), ptrInboundClient(), nil, false, "")

	assert.NoError(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestCreateInboundClient_CertificateRequiresClientID() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	svc := newServiceForTest(store)

	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "private_key_jwt",
		Certificate:             &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
	}

	err := svc.CreateInboundClient(context.Background(), ptrInboundClient(), p, false, "")

	assert.ErrorIs(suite.T(), err, ErrOAuthCertificateRequiresClientID)
}

func (suite *InboundClientServiceTestSuite) TestCreateInboundClient_RefusesDeclarative() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(true)

	svc := newServiceForTest(store)
	err := svc.CreateInboundClient(context.Background(), ptrInboundClient(), nil, false, "")

	assert.ErrorIs(suite.T(), err, ErrCannotModifyDeclarative)
}

func (suite *InboundClientServiceTestSuite) TestUpdateInboundClient_RefusesDeclarative() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(true)

	svc := newServiceForTest(store)
	err := svc.UpdateInboundClient(context.Background(), ptrInboundClient(), nil, false, "", "")

	assert.ErrorIs(suite.T(), err, ErrCannotModifyDeclarative)
}

func (suite *InboundClientServiceTestSuite) TestUpdateInboundClient_CertificateRequiresClientID() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	svc := newServiceForTest(store)

	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "private_key_jwt",
		Certificate:             &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
	}

	err := svc.UpdateInboundClient(context.Background(), ptrInboundClient(), p, false, "", "")

	assert.ErrorIs(suite.T(), err, ErrOAuthCertificateRequiresClientID)
}

func (suite *InboundClientServiceTestSuite) TestDeleteInboundClient_RefusesDeclarative() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(true)

	svc := newServiceForTest(store)
	err := svc.DeleteInboundClient(context.Background(), "p1")

	assert.ErrorIs(suite.T(), err, ErrCannotModifyDeclarative)
}

func (suite *InboundClientServiceTestSuite) TestDelegatesPlainReads() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().GetInboundClientList(mock.Anything, mock.Anything).
		Return([]inboundmodel.InboundClient{validInboundClient()}, nil)
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(true)

	svc := newServiceForTest(store)
	list, err := svc.GetInboundClientList(context.Background())
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), list, 1)

	assert.True(suite.T(), svc.IsDeclarative(context.Background(), "p1"))
}

func (suite *InboundClientServiceTestSuite) TestDeleteInboundClient() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	store.EXPECT().DeleteInboundClient(mock.Anything, "p1").Return(nil)

	svc := newServiceForTest(store)
	assert.NoError(suite.T(), svc.DeleteInboundClient(context.Background(), "p1"))
}

func (suite *InboundClientServiceTestSuite) TestStorePropagatesErrors() {
	storeErr := errors.New("db error")
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	store.EXPECT().CreateInboundClient(mock.Anything, mock.Anything).Return(storeErr)

	svc := newServiceForTest(store)
	err := svc.CreateInboundClient(context.Background(), ptrInboundClient(), nil, false, "")

	assert.ErrorIs(suite.T(), err, storeErr)
}

// ----- ValidateCertificateInput -----

func (suite *InboundClientServiceTestSuite) TestValidateCertificateInput_Empty() {
	c, err := validateCertificateInput("ref-1", "", nil)

	suite.Nil(c)
	suite.Nil(err)
}

func (suite *InboundClientServiceTestSuite) TestValidateCertificateInput_JWKS_Success() {
	c, err := validateCertificateInput("ref-1", "existing",
		&inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: `{"keys":[]}`})

	suite.Nil(err)
	suite.NotNil(c)
	suite.Equal("existing", c.ID)
	suite.Equal(cert.CertificateTypeJWKS, c.Type)
	suite.Equal(cert.CertificateReferenceTypeOAuthApp, c.RefType)
	suite.Equal("ref-1", c.RefID)
}

func (suite *InboundClientServiceTestSuite) TestValidateCertificateInput_JWKS_MissingValue() {
	c, err := validateCertificateInput("ref-1", "",
		&inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: ""})

	suite.Nil(c)
	suite.ErrorIs(err, ErrCertValueRequired)
}

func (suite *InboundClientServiceTestSuite) TestValidateCertificateInput_JWKSURI_Success() {
	c, err := validateCertificateInput("ref-1", "",
		&inboundmodel.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "https://example.com/jwks"})

	suite.Nil(err)
	suite.Equal(cert.CertificateTypeJWKSURI, c.Type)
}

func (suite *InboundClientServiceTestSuite) TestValidateCertificateInput_JWKSURI_Invalid() {
	c, err := validateCertificateInput("ref-1", "",
		&inboundmodel.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "not-a-uri"})

	suite.Nil(c)
	suite.ErrorIs(err, ErrCertInvalidJWKSURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateCertificateInput_InvalidType() {
	c, err := validateCertificateInput("ref-1", "",
		&inboundmodel.Certificate{Type: "bogus", Value: "x"})

	suite.Nil(c)
	suite.ErrorIs(err, ErrCertInvalidType)
}

// ----- CreateCertificate -----

func (suite *InboundClientServiceTestSuite) TestCreateCertificate_Nil() {
	svc := newServiceWithCert(certmock.NewCertificateServiceInterfaceMock(suite.T()))

	out, vErr, opErr := svc.createCertificate(context.Background(), "ref-1", nil)

	suite.Nil(out)
	suite.Nil(vErr)
	suite.Nil(opErr)
}

func (suite *InboundClientServiceTestSuite) TestCreateCertificate_Success() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().CreateCertificate(mock.Anything, mock.Anything).
		Return(&cert.Certificate{}, nil)
	svc := newServiceWithCert(mockCert)

	in := &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: `{}`}
	out, vErr, opErr := svc.createCertificate(context.Background(), "ref-1", in)

	suite.Nil(vErr)
	suite.Nil(opErr)
	suite.Equal(cert.CertificateTypeJWKS, out.Type)
	suite.Equal(`{}`, out.Value)
}

func (suite *InboundClientServiceTestSuite) TestCreateCertificate_InvalidInput() {
	svc := newServiceWithCert(certmock.NewCertificateServiceInterfaceMock(suite.T()))

	in := &inboundmodel.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "not-a-uri"}
	out, vErr, opErr := svc.createCertificate(context.Background(), "ref-1", in)

	suite.Nil(out)
	suite.Nil(opErr)
	suite.ErrorIs(vErr, ErrCertInvalidJWKSURI)
}

func (suite *InboundClientServiceTestSuite) TestCreateCertificate_ServiceError() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	clientErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "C-1"}
	mockCert.EXPECT().CreateCertificate(mock.Anything, mock.Anything).Return(nil, clientErr)
	svc := newServiceWithCert(mockCert)

	in := &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: `{}`}
	out, vErr, opErr := svc.createCertificate(context.Background(), "ref-1", in)

	suite.Nil(out)
	suite.Nil(vErr)
	suite.Equal(CertOpCreate, opErr.Operation)
	suite.Same(clientErr, opErr.Underlying)
	suite.True(opErr.IsClientError())
}

// ----- GetCertificate -----

func (suite *InboundClientServiceTestSuite) TestGetCertificate_NotFound() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, cert.CertificateReferenceTypeOAuthApp, "ref-1").
		Return(nil, &cert.ErrorCertificateNotFound)
	svc := newServiceWithCert(mockCert)

	out, err := svc.GetCertificate(context.Background(), cert.CertificateReferenceTypeOAuthApp, "ref-1")

	suite.Nil(out)
	suite.Nil(err)
}

func (suite *InboundClientServiceTestSuite) TestGetCertificate_Success() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, cert.CertificateReferenceTypeOAuthApp, "ref-1").
		Return(&cert.Certificate{Type: cert.CertificateTypeJWKS, Value: `{}`}, nil)
	svc := newServiceWithCert(mockCert)

	out, err := svc.GetCertificate(context.Background(), cert.CertificateReferenceTypeOAuthApp, "ref-1")

	suite.Nil(err)
	suite.Equal(cert.CertificateTypeJWKS, out.Type)
}

func (suite *InboundClientServiceTestSuite) TestGetCertificate_ServerError() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	srvErr := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "S-1"}
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, cert.CertificateReferenceTypeOAuthApp, "ref-1").
		Return(nil, srvErr)
	svc := newServiceWithCert(mockCert)

	out, err := svc.GetCertificate(context.Background(), cert.CertificateReferenceTypeOAuthApp, "ref-1")

	suite.Nil(out)
	suite.Equal(CertOpRetrieve, err.Operation)
	suite.False(err.IsClientError())
}

// ----- DeleteCertificate -----

func (suite *InboundClientServiceTestSuite) TestDeleteCertificate_Success() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		DeleteCertificateByReference(mock.Anything, cert.CertificateReferenceTypeOAuthApp, "ref-1").
		Return(nil)
	svc := newServiceWithCert(mockCert)

	err := svc.deleteCertificate(context.Background(), "ref-1")

	suite.Nil(err)
}

func (suite *InboundClientServiceTestSuite) TestDeleteCertificate_Error() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	clientErr := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "D-1"}
	mockCert.EXPECT().
		DeleteCertificateByReference(mock.Anything, cert.CertificateReferenceTypeOAuthApp, "ref-1").
		Return(clientErr)
	svc := newServiceWithCert(mockCert)

	err := svc.deleteCertificate(context.Background(), "ref-1")

	suite.NotNil(err)
	suite.Equal(CertOpDelete, err.Operation)
	suite.Equal(cert.CertificateReferenceTypeOAuthApp, err.RefType)
}

// ----- SyncCertificate -----

func (suite *InboundClientServiceTestSuite) TestSyncCertificate_NoOp_NoExistingNoInput() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &cert.ErrorCertificateNotFound)
	svc := newServiceWithCert(mockCert)

	out, vErr, opErr := svc.syncCertificate(context.Background(), "ref-1", nil)

	suite.Nil(out)
	suite.Nil(vErr)
	suite.Nil(opErr)
}

func (suite *InboundClientServiceTestSuite) TestSyncCertificate_CreateWhenAbsent() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &cert.ErrorCertificateNotFound)
	mockCert.EXPECT().CreateCertificate(mock.Anything, mock.Anything).
		Return(&cert.Certificate{}, nil)
	svc := newServiceWithCert(mockCert)

	out, vErr, opErr := svc.syncCertificate(context.Background(), "ref-1",
		&inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: `{}`})

	suite.Nil(vErr)
	suite.Nil(opErr)
	suite.NotNil(out)
}

func (suite *InboundClientServiceTestSuite) TestSyncCertificate_UpdateWhenPresent() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, mock.Anything, mock.Anything).
		Return(&cert.Certificate{ID: "cert-1"}, nil)
	mockCert.EXPECT().UpdateCertificateByID(mock.Anything, "cert-1", mock.Anything).
		Return(&cert.Certificate{}, nil)
	svc := newServiceWithCert(mockCert)

	out, vErr, opErr := svc.syncCertificate(context.Background(), "ref-1",
		&inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: `{}`})

	suite.Nil(vErr)
	suite.Nil(opErr)
	suite.NotNil(out)
}

func (suite *InboundClientServiceTestSuite) TestSyncCertificate_DeleteWhenInputEmpty() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, mock.Anything, mock.Anything).
		Return(&cert.Certificate{ID: "cert-1"}, nil)
	mockCert.EXPECT().
		DeleteCertificateByReference(mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	svc := newServiceWithCert(mockCert)

	out, vErr, opErr := svc.syncCertificate(context.Background(), "ref-1", nil)

	suite.Nil(out)
	suite.Nil(vErr)
	suite.Nil(opErr)
}

func (suite *InboundClientServiceTestSuite) TestSyncCertificate_ValidationError() {
	mockCert := certmock.NewCertificateServiceInterfaceMock(suite.T())
	mockCert.EXPECT().
		GetCertificateByReference(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &cert.ErrorCertificateNotFound)
	svc := newServiceWithCert(mockCert)

	out, vErr, opErr := svc.syncCertificate(context.Background(), "ref-1",
		&inboundmodel.Certificate{Type: "bogus", Value: "x"})

	suite.Nil(out)
	suite.Nil(opErr)
	suite.ErrorIs(vErr, ErrCertInvalidType)
}

func (suite *InboundClientServiceTestSuite) TestGetInboundClientByEntityID_Delegates() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	want := &inboundmodel.InboundClient{ID: "p1"}
	store.EXPECT().GetInboundClientByEntityID(mock.Anything, "p1").Return(want, nil)

	svc := newServiceForTest(store)
	got, err := svc.GetInboundClientByEntityID(context.Background(), "p1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "p1", got.ID)
}

func (suite *InboundClientServiceTestSuite) TestGetOAuthProfileByEntityID_Delegates() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	want := &providers.OAuthProfile{GrantTypes: []string{"authorization_code"}}
	store.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "p1").Return(want, nil)

	svc := newServiceForTest(store)
	got, err := svc.GetOAuthProfileByEntityID(context.Background(), "p1")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), want, got)
}

func (suite *InboundClientServiceTestSuite) TestUpdateInboundClient_ValidationFails() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	svc := newServiceForTest(store)

	p := validOAuthProfile()
	p.GrantTypes = []string{"not_a_real_grant"}

	err := svc.UpdateInboundClient(context.Background(), ptrInboundClient(), p, false, "", "")
	assert.ErrorIs(suite.T(), err, ErrOAuthInvalidGrantType)
}

func (suite *InboundClientServiceTestSuite) TestUpdateInboundClient_Succeeds() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	store.EXPECT().UpdateInboundClient(mock.Anything, mock.Anything).Return(nil)
	// syncOAuthProfile path: GetOAuthProfileByEntityID returns not found → CreateOAuthProfile
	store.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "p1").Return(nil, ErrInboundClientNotFound)
	store.EXPECT().CreateOAuthProfile(mock.Anything, "p1", mock.Anything).Return(nil)

	svc := newInboundClientService(store, transaction.NewNoOpTransactioner(), nil, nil, nil, nil, nil, nil, nil)
	err := svc.UpdateInboundClient(context.Background(), ptrInboundClient(), validOAuthProfile(), true, "", "")
	assert.NoError(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestValidate_ValidProfile() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	svc := newServiceForTest(store)

	err := svc.Validate(context.Background(), ptrInboundClient(), validOAuthProfile(), true)
	assert.NoError(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestValidate_InvalidGrantType() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	svc := newServiceForTest(store)

	p := validOAuthProfile()
	p.GrantTypes = []string{"bogus_grant"}

	err := svc.Validate(context.Background(), ptrInboundClient(), p, false)
	assert.ErrorIs(suite.T(), err, ErrOAuthInvalidGrantType)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_WildcardInHost_Rejected() {
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://*.example.com/cb"},
		GrantTypes:   []string{"authorization_code"},
	}
	err := validateRedirectURIs(p)
	assert.ErrorIs(suite.T(), err, ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_WildcardInQuery_Rejected() {
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://app.example.com/cb?foo=*"},
		GrantTypes:   []string{"authorization_code"},
	}
	err := validateRedirectURIs(p)
	assert.ErrorIs(suite.T(), err, ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidatePublicClient_PKCENotRequired_Fails() {
	p := &providers.OAuthProfile{
		PublicClient:            true,
		PKCERequired:            false,
		TokenEndpointAuthMethod: "none",
	}
	err := validatePublicClient(p)
	assert.ErrorIs(suite.T(), err, ErrOAuthPublicClientMustHavePKCE)
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpointAuthMethod_InvalidMethod() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "bogus_method",
	}
	err := validateTokenEndpointAuthMethod(p, false)
	assert.ErrorIs(suite.T(), err, ErrOAuthInvalidTokenEndpointAuthMethod)
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpoint_CertAllowedWhenUserInfoNeedsIt() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "client_secret_basic",
		Certificate:             &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		UserInfo:                &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256"},
	}
	assert.NoError(suite.T(), validateTokenEndpointAuthMethod(p, true))
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpoint_CertRejectedWhenUserInfoDoesNotNeedIt() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "client_secret_basic",
		Certificate:             &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
	}
	err := validateTokenEndpointAuthMethod(p, true)
	assert.ErrorIs(suite.T(), err, ErrOAuthClientSecretCannotHaveCertificate)
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpointAuthMethod_PrivateKeyJWTHappy() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "private_key_jwt",
		Certificate:             &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
	}
	assert.NoError(suite.T(), validateTokenEndpointAuthMethod(p, false))
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpointAuthMethod_PrivateKeyJWTMissingCert() {
	p := &providers.OAuthProfile{TokenEndpointAuthMethod: "private_key_jwt"}
	err := validateTokenEndpointAuthMethod(p, false)
	assert.ErrorIs(suite.T(), err, ErrOAuthPrivateKeyJWTRequiresCertificate)
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpointAuthMethod_PrivateKeyJWTWithSecret() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "private_key_jwt",
		Certificate:             &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
	}
	err := validateTokenEndpointAuthMethod(p, true)
	assert.ErrorIs(suite.T(), err, ErrOAuthPrivateKeyJWTCannotHaveClientSecret)
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpointAuthMethod_NoneRequiresPublicClient() {
	p := &providers.OAuthProfile{TokenEndpointAuthMethod: "none"}
	err := validateTokenEndpointAuthMethod(p, false)
	assert.ErrorIs(suite.T(), err, ErrOAuthNoneAuthRequiresPublicClient)
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpointAuthMethod_NoneRejectsCertOrSecret() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "none",
		PublicClient:            true,
		Certificate:             &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
	}
	err := validateTokenEndpointAuthMethod(p, false)
	assert.ErrorIs(suite.T(), err, ErrOAuthNoneAuthCannotHaveCertOrSecret)
}

func (suite *InboundClientServiceTestSuite) TestValidateTokenEndpointAuthMethod_NoneClientCredentialsRejected() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "none",
		PublicClient:            true,
		GrantTypes:              []string{"client_credentials"},
	}
	err := validateTokenEndpointAuthMethod(p, false)
	assert.ErrorIs(suite.T(), err, ErrOAuthClientCredentialsCannotUseNoneAuth)
}

// validateUserInfoConfig — happy paths

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_NilUserInfo() {
	assert.NoError(suite.T(), validateUserInfoConfig(&providers.OAuthProfile{}))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_PlainJSON() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{ResponseType: providers.UserInfoResponseTypeJSON},
	}
	assert.NoError(suite.T(), validateUserInfoConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_JWSHappy() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{
			ResponseType: providers.UserInfoResponseTypeJWS,
			SigningAlg:   "RS256",
		},
	}
	assert.NoError(suite.T(), validateUserInfoConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_JWEHappy() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		UserInfo: &providers.UserInfoConfig{
			ResponseType:  providers.UserInfoResponseTypeJWE,
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "A256GCM",
		},
	}
	assert.NoError(suite.T(), validateUserInfoConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_NestedJWTHappy() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		UserInfo: &providers.UserInfoConfig{
			ResponseType:  providers.UserInfoResponseTypeNESTEDJWT,
			SigningAlg:    "RS256",
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "A256GCM",
		},
	}
	assert.NoError(suite.T(), validateUserInfoConfig(p))
}

// validateUserInfoConfig — error paths

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_UnsupportedSigningAlg() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{SigningAlg: "BOGUS"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoUnsupportedSigningAlg)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_EncryptionEncWithoutAlg() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{EncryptionEnc: "A256GCM"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoEncryptionEncRequiresAlg)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_UnsupportedEncryptionAlg() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{EncryptionAlg: "BOGUS", EncryptionEnc: "A256GCM"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoUnsupportedEncryptionAlg)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_EncryptionAlgWithoutEnc() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoEncryptionAlgRequiresEnc)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_UnsupportedEncryptionEnc() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "BOGUS"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoUnsupportedEncryptionEnc)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_EncryptionRequiresCertificate() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoEncryptionRequiresCertificate)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_JWKSURISSRFRejection() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "http://127.0.0.1/jwks"},
		UserInfo: &providers.UserInfoConfig{
			EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM",
		},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoJWKSURINotSSRFSafe)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_JWSMissingSigningAlg() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{ResponseType: providers.UserInfoResponseTypeJWS},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoJWSRequiresSigningAlg)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_JWEMissingEncryption() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{ResponseType: providers.UserInfoResponseTypeJWE},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoJWERequiresEncryption)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_NestedJWTMissingFields() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{ResponseType: providers.UserInfoResponseTypeNESTEDJWT},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoNestedJWTRequiresAll)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_UnsupportedResponseType() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{ResponseType: "BOGUS"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoUnsupportedResponseType)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_SigningAlgRequiresResponseType() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{SigningAlg: "RS256"},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoAlgRequiresResponseType)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_EncryptionAlgRequiresResponseType() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		UserInfo: &providers.UserInfoConfig{
			EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM",
		},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoAlgRequiresResponseType)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserInfoConfig_AllAlgsRequireResponseType() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		UserInfo: &providers.UserInfoConfig{
			SigningAlg: "RS256", EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM",
		},
	}
	assert.ErrorIs(suite.T(), validateUserInfoConfig(p), ErrOAuthUserInfoAlgRequiresResponseType)
}

// validateIDTokenConfig — happy paths

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_NilToken() {
	assert.NoError(suite.T(), validateIDTokenConfig(&providers.OAuthProfile{}))
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_NilIDToken() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{},
	}
	assert.NoError(suite.T(), validateIDTokenConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_NoEncryption() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{ValidityPeriod: 3600}},
	}
	assert.NoError(suite.T(), validateIDTokenConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_ValidAlgEncWithCert() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "A256GCM",
		}},
	}
	assert.NoError(suite.T(), validateIDTokenConfig(p))
}

// validateIDTokenConfig — error paths

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_EncryptionEncWithoutAlg() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionEnc: "A256GCM",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenEncryptionAlgRequiresEnc)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_EncryptionAlgWithoutEnc() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionAlg: "RSA-OAEP-256",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenEncryptionAlgRequiresEnc)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_UnsupportedEncryptionAlg() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionAlg: "BOGUS",
			EncryptionEnc: "A256GCM",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenUnsupportedEncryptionAlg)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_UnsupportedEncryptionEnc() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "BOGUS",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenUnsupportedEncryptionEnc)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_EncryptionRequiresCertificate() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "A256GCM",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenEncryptionRequiresCertificate)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_JWKSURISSRFRejection() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKSURI, Value: "http://127.0.0.1/jwks"},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "A256GCM",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenJWKSURINotSSRFSafe)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_EmptyResponseType_DefaultsToJWT() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{ValidityPeriod: 3600}},
	}
	assert.NoError(suite.T(), validateIDTokenConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_JWTResponseType_NoEncryption() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType: providers.IDTokenResponseTypeJWT,
		}},
	}
	assert.NoError(suite.T(), validateIDTokenConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_JWTResponseType_WithEncryptionAlg() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWT,
			EncryptionAlg: "RSA-OAEP-256",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenEncryptionFieldsNotAllowed)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_NESTEDJWTResponseType_ValidFullConfig() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeNESTEDJWT,
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "A256GCM",
		}},
	}
	assert.NoError(suite.T(), validateIDTokenConfig(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_NESTEDJWTResponseType_MissingAlg() {
	p := &providers.OAuthProfile{
		Certificate: &inboundmodel.Certificate{Type: cert.CertificateTypeJWKS, Value: "{}"},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeNESTEDJWT,
			EncryptionEnc: "A256GCM",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenEncryptionAlgRequiresEnc)
}

func (suite *InboundClientServiceTestSuite) TestValidateIDTokenConfig_UnsupportedResponseType() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType: "INVALID",
		}},
	}
	assert.ErrorIs(suite.T(), validateIDTokenConfig(p), ErrOAuthIDTokenUnsupportedResponseType)
}

func (suite *InboundClientServiceTestSuite) TestResolveUserInfo_DefaultsResponseTypeToJSON() {
	out := resolveUserInfo(nil, nil)
	assert.Equal(suite.T(), providers.UserInfoResponseTypeJSON, out.ResponseType)
}

func (suite *InboundClientServiceTestSuite) TestResolveUserInfo_DefaultsResponseTypeToJSONForPartialConfig() {
	out := resolveUserInfo(&providers.UserInfoConfig{UserAttributes: []string{"email"}}, nil)
	assert.Equal(suite.T(), providers.UserInfoResponseTypeJSON, out.ResponseType)
}

func (suite *InboundClientServiceTestSuite) TestResolveUserInfo_PreservesExplicitResponseType() {
	in := &providers.UserInfoConfig{ResponseType: providers.UserInfoResponseTypeJWS, SigningAlg: "RS256"}
	out := resolveUserInfo(in, nil)
	assert.Equal(suite.T(), providers.UserInfoResponseTypeJWS, out.ResponseType)
}

func (suite *InboundClientServiceTestSuite) TestResolveUserInfo_FallsBackToIDTokenAttributes() {
	idToken := &providers.IDTokenConfig{UserAttributes: []string{"email"}}
	out := resolveUserInfo(&providers.UserInfoConfig{}, idToken)
	assert.Equal(suite.T(), []string{"email"}, out.UserAttributes)
	assert.Equal(suite.T(), providers.UserInfoResponseTypeJSON, out.ResponseType)
}

func (suite *InboundClientServiceTestSuite) TestResolveUserInfo_PreservesUserAttributesOverIDToken() {
	idToken := &providers.IDTokenConfig{UserAttributes: []string{"sub"}}
	out := resolveUserInfo(&providers.UserInfoConfig{UserAttributes: []string{"email"}}, idToken)
	assert.Equal(suite.T(), []string{"email"}, out.UserAttributes)
}

// validateOAuthProfile — verifies UserInfo validation is wired in.

func (suite *InboundClientServiceTestSuite) TestValidateOAuthProfile_PropagatesUserInfoErrors() {
	p := &providers.OAuthProfile{
		RedirectURIs:            []string{"https://app.example.com/cb"},
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "client_secret_basic",
		UserInfo:                &providers.UserInfoConfig{SigningAlg: "BOGUS"},
	}
	assert.ErrorIs(suite.T(), validateOAuthProfile(p, true), ErrOAuthUserInfoUnsupportedSigningAlg)
}

func (suite *InboundClientServiceTestSuite) TestValidateOAuthProfile_NilProfile() {
	assert.NoError(suite.T(), validateOAuthProfile(nil, false))
}

// ----- BuildOAuthClient -----

func (suite *InboundClientServiceTestSuite) TestBuildOAuthClient_MapsAllFields() {
	dao := &providers.OAuthProfile{
		RedirectURIs:                       []string{"https://app/cb"},
		GrantTypes:                         []string{"authorization_code", "refresh_token"},
		ResponseTypes:                      []string{"code"},
		TokenEndpointAuthMethod:            "client_secret_basic",
		PKCERequired:                       true,
		PublicClient:                       false,
		RequirePushedAuthorizationRequests: true,
		IncludeActClaim:                    true,
		Scopes:                             []string{"openid"},
		ScopeClaims:                        map[string][]string{"profile": {"name"}},
	}
	client := BuildOAuthClient("entity-1", "client-1", "ou-1", providers.EntityCategoryApp, dao)

	assert.Equal(suite.T(), "entity-1", client.ID)
	assert.Equal(suite.T(), "client-1", client.ClientID)
	assert.Equal(suite.T(), "ou-1", client.OUID)
	assert.Equal(suite.T(), providers.EntityCategoryApp, client.EntityCategory)
	assert.True(suite.T(), client.IncludeActClaim)
	assert.Equal(suite.T(), []string{"https://app/cb"}, client.RedirectURIs)
	assert.Equal(suite.T(), providers.TokenEndpointAuthMethod("client_secret_basic"), client.TokenEndpointAuthMethod)
	assert.True(suite.T(), client.PKCERequired)
	assert.True(suite.T(), client.RequirePushedAuthorizationRequests)
	assert.Equal(suite.T(), []providers.GrantType{"authorization_code", "refresh_token"}, client.GrantTypes)
	assert.Equal(suite.T(), []providers.ResponseType{"code"}, client.ResponseTypes)
}

// ----- resolveAssertion -----

func (suite *InboundClientServiceTestSuite) TestResolveAssertion_NilInputUsesDefault() {
	out := resolveAssertion(nil, &inboundmodel.AssertionConfig{ValidityPeriod: 3600})
	assert.Equal(suite.T(), int64(3600), out.ValidityPeriod)
	assert.NotNil(suite.T(), out.UserAttributes)
}

func (suite *InboundClientServiceTestSuite) TestResolveAssertion_BothNilZeroValues() {
	out := resolveAssertion(nil, nil)
	assert.Equal(suite.T(), int64(0), out.ValidityPeriod)
	assert.NotNil(suite.T(), out.UserAttributes)
}

func (suite *InboundClientServiceTestSuite) TestResolveAssertion_InputZeroValidityFallsBack() {
	out := resolveAssertion(
		&inboundmodel.AssertionConfig{ValidityPeriod: 0, UserAttributes: []string{"sub"}},
		&inboundmodel.AssertionConfig{ValidityPeriod: 600},
	)
	assert.Equal(suite.T(), int64(600), out.ValidityPeriod)
	assert.Equal(suite.T(), []string{"sub"}, out.UserAttributes)
}

func (suite *InboundClientServiceTestSuite) TestResolveAssertion_InputOverridesDefault() {
	out := resolveAssertion(
		&inboundmodel.AssertionConfig{ValidityPeriod: 1200, UserAttributes: []string{"email"}},
		&inboundmodel.AssertionConfig{ValidityPeriod: 600},
	)
	assert.Equal(suite.T(), int64(1200), out.ValidityPeriod)
}

// ----- resolveOAuthTokens -----

func (suite *InboundClientServiceTestSuite) TestResolveOAuthTokens_NilInputUsesAssertion() {
	sysconfig.GetServerRuntime().Config.OAuth.RefreshToken.ValidityPeriod = 86400

	assertion := &inboundmodel.AssertionConfig{ValidityPeriod: 900, UserAttributes: []string{"email"}}
	at, idt, rt := resolveOAuthTokens(nil, assertion)

	assert.Equal(suite.T(), int64(900), at.UserConfig.ValidityPeriod)
	assert.Equal(suite.T(), []string{"email"}, at.UserConfig.Attributes)
	assert.Equal(suite.T(), int64(900), idt.ValidityPeriod)
	assert.Equal(suite.T(), int64(86400), rt.ValidityPeriod)
}

func (suite *InboundClientServiceTestSuite) TestResolveOAuthTokens_InputOverrides() {
	in := &providers.OAuthTokenConfig{
		AccessToken: &providers.AccessTokenConfig{
			UserConfig: &providers.AccessTokenSubConfig{ValidityPeriod: 60, Attributes: []string{"sub"}},
		},
		IDToken:      &providers.IDTokenConfig{ValidityPeriod: 120, UserAttributes: []string{"email"}},
		RefreshToken: &providers.RefreshTokenConfig{ValidityPeriod: 1800},
	}
	at, idt, rt := resolveOAuthTokens(in, &inboundmodel.AssertionConfig{ValidityPeriod: 900})
	assert.Equal(suite.T(), int64(60), at.UserConfig.ValidityPeriod)
	assert.Equal(suite.T(), int64(120), idt.ValidityPeriod)
	assert.Equal(suite.T(), int64(1800), rt.ValidityPeriod)
}

func (suite *InboundClientServiceTestSuite) TestResolveOAuthTokens_NilAssertionDoesNotPanic() {
	at, idt, rt := resolveOAuthTokens(nil, nil)
	assert.NotNil(suite.T(), at)
	assert.NotNil(suite.T(), idt)
	assert.NotNil(suite.T(), rt)
}

func (suite *InboundClientServiceTestSuite) TestResolveOAuthTokens_ZeroValidityFallsBack() {
	sysconfig.GetServerRuntime().Config.OAuth.RefreshToken.ValidityPeriod = 86400

	in := &providers.OAuthTokenConfig{
		AccessToken: &providers.AccessTokenConfig{
			UserConfig: &providers.AccessTokenSubConfig{ValidityPeriod: 0},
		},
		IDToken:      &providers.IDTokenConfig{ValidityPeriod: 0},
		RefreshToken: &providers.RefreshTokenConfig{ValidityPeriod: 0},
	}
	at, idt, rt := resolveOAuthTokens(in, &inboundmodel.AssertionConfig{ValidityPeriod: 1800})
	assert.Equal(suite.T(), int64(1800), at.UserConfig.ValidityPeriod)
	assert.Equal(suite.T(), int64(1800), idt.ValidityPeriod)
	assert.Equal(suite.T(), int64(86400), rt.ValidityPeriod)
}

// ----- resolveScopeClaims -----

func (suite *InboundClientServiceTestSuite) TestResolveScopeClaims_NilReturnsEmptyMap() {
	out := resolveScopeClaims(nil)
	assert.NotNil(suite.T(), out)
	assert.Empty(suite.T(), out)
}

func (suite *InboundClientServiceTestSuite) TestResolveScopeClaims_PassesThroughExistingMap() {
	in := map[string][]string{"profile": {"given_name"}}
	out := resolveScopeClaims(in)
	assert.Equal(suite.T(), in, out)
}

// ----- validateRedirectURIs error branches -----

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_SchemeWildcardRejected() {
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"htt*://app/cb"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_FragmentRejected() {
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://app/cb#frag"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthRedirectURIFragmentNotAllowed)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardRejected() {
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://*.app.com/cb"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_QueryWildcardRejected() {
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://app/cb?x=*"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthInvalidRedirectURI)
}

// ----- Host wildcard registration with allow_wildcard_redirect_uri = true -----

func (suite *InboundClientServiceTestSuite) enableWildcardConfig() {
	sysconfig.ResetServerRuntime()
	cfg := &sysconfig.Config{}
	cfg.OAuth.AllowWildcardRedirectURI = true
	suite.Require().NoError(sysconfig.InitializeServerRuntime("/tmp/test", cfg))
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardLabelInternal_Accepted() {
	suite.enableWildcardConfig()
	p := &providers.OAuthProfile{
		RedirectURIs:  []string{"https://tenant-app-*-*.gateway.example.com/cb"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
	}
	assert.NoError(suite.T(), validateRedirectURIs(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardSimplePattern_Accepted() {
	suite.enableWildcardConfig()
	p := &providers.OAuthProfile{
		RedirectURIs:  []string{"https://app-*.example.com/cb"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
	}
	assert.NoError(suite.T(), validateRedirectURIs(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardWholeLabel_Rejected() {
	suite.enableWildcardConfig()
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://*.example.com/cb"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardInPort_Rejected() {
	suite.enableWildcardConfig()
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://app.example.com:80*0/cb"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardWithPort_Accepted() {
	suite.enableWildcardConfig()
	p := &providers.OAuthProfile{
		RedirectURIs:  []string{"https://app-*.example.com:8443/cb"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
	}
	assert.NoError(suite.T(), validateRedirectURIs(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardFlagOff_Rejected() {
	// SetupTest already initializes with AllowWildcardRedirectURI = false.
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"https://app-*.example.com/cb"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_HostWildcardMixedWithPath_Accepted() {
	suite.enableWildcardConfig()
	p := &providers.OAuthProfile{
		RedirectURIs:  []string{"https://app-*.example.com/cb/*"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
	}
	assert.NoError(suite.T(), validateRedirectURIs(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_MissingSchemeRejected() {
	p := &providers.OAuthProfile{
		RedirectURIs: []string{"//app/cb"},
		GrantTypes:   []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthInvalidRedirectURI)
}

func (suite *InboundClientServiceTestSuite) TestValidateRedirectURIs_AuthCodeWithoutURIs() {
	p := &providers.OAuthProfile{
		GrantTypes: []string{"authorization_code"},
	}
	assert.ErrorIs(suite.T(), validateRedirectURIs(p), ErrOAuthAuthCodeRequiresRedirectURIs)
}

// ----- containsInvalidWildcardSegment -----

func (suite *InboundClientServiceTestSuite) TestContainsInvalidWildcardSegment_PartialWildcard() {
	assert.True(suite.T(), containsInvalidWildcardSegment("/foo*"))
}

func (suite *InboundClientServiceTestSuite) TestContainsInvalidWildcardSegment_RegexMetachars() {
	assert.True(suite.T(), containsInvalidWildcardSegment("/[a-z]+"))
	assert.True(suite.T(), containsInvalidWildcardSegment("/foo|bar"))
	assert.True(suite.T(), containsInvalidWildcardSegment("/foo(x)"))
	assert.True(suite.T(), containsInvalidWildcardSegment("/foo$"))
}

func (suite *InboundClientServiceTestSuite) TestContainsInvalidWildcardSegment_Allowed() {
	assert.False(suite.T(), containsInvalidWildcardSegment("/foo/*/bar"))
	assert.False(suite.T(), containsInvalidWildcardSegment("/foo/**"))
	assert.False(suite.T(), containsInvalidWildcardSegment("/plain/path"))
}

// ----- FK validators -----

func (suite *InboundClientServiceTestSuite) TestValidateAuthFlowID_EmptyOrNoMgtIsNoOp() {
	svc := &inboundClientService{}
	assert.NoError(suite.T(), svc.validateAuthFlowID(context.Background(), ""))
}

func (suite *InboundClientServiceTestSuite) TestValidateAuthFlowID_InvalidReturnsError() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "bad-flow", providers.FlowTypeAuthentication).
		Return(false, nil)
	svc := &inboundClientService{flowMgt: flowMgt}
	assert.ErrorIs(suite.T(), svc.validateAuthFlowID(context.Background(), "bad-flow"), ErrFKInvalidAuthFlow)
}

func (suite *InboundClientServiceTestSuite) TestValidateAuthFlowID_ServerErrorPropagated() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "fid", providers.FlowTypeAuthentication).
		Return(false, &tidcommon.ServiceError{Code: "X"})
	svc := &inboundClientService{flowMgt: flowMgt}
	assert.ErrorIs(suite.T(), svc.validateAuthFlowID(context.Background(), "fid"), ErrFKFlowServerError)
}

func (suite *InboundClientServiceTestSuite) TestValidateAuthFlowID_ValidNoError() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "good", providers.FlowTypeAuthentication).
		Return(true, nil)
	svc := &inboundClientService{flowMgt: flowMgt}
	assert.NoError(suite.T(), svc.validateAuthFlowID(context.Background(), "good"))
}

func (suite *InboundClientServiceTestSuite) testValidateFlowID(
	flowType providers.FlowType,
	validateFn func(*inboundClientService, context.Context, string) error,
	invalidErr, serverErr error,
) {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "x", flowType).Return(false, nil).Once()
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "y", flowType).
		Return(false, &tidcommon.ServiceError{Code: "E"}).Once()
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "z", flowType).Return(true, nil).Once()
	svc := &inboundClientService{flowMgt: flowMgt}
	assert.ErrorIs(suite.T(), validateFn(svc, context.Background(), "x"), invalidErr)
	assert.ErrorIs(suite.T(), validateFn(svc, context.Background(), "y"), serverErr)
	assert.NoError(suite.T(), validateFn(svc, context.Background(), "z"))
	assert.NoError(suite.T(), validateFn(&inboundClientService{}, context.Background(), ""))
}

func (suite *InboundClientServiceTestSuite) TestValidateRegistrationFlowID_AllBranches() {
	suite.testValidateFlowID(
		providers.FlowTypeRegistration,
		(*inboundClientService).validateRegistrationFlowID,
		ErrFKInvalidRegistrationFlow,
		ErrFKFlowServerError,
	)
}

func (suite *InboundClientServiceTestSuite) TestValidateRecoveryFlowID_AllBranches() {
	suite.testValidateFlowID(
		providers.FlowTypeRecovery,
		(*inboundClientService).validateRecoveryFlowID,
		ErrFKInvalidRecoveryFlow,
		ErrFKFlowServerError,
	)
}

//nolint:dupl // Theme and layout validators share the same branch structure with type-specific services.
func (suite *InboundClientServiceTestSuite) TestValidateThemeID_AllBranches() {
	tm := thememock.NewThemeMgtServiceInterfaceMock(suite.T())
	tm.EXPECT().IsThemeExist(mock.Anything, "missing").Return(false, nil).Once()
	tm.EXPECT().IsThemeExist(mock.Anything, "err").Return(false, &tidcommon.ServiceError{Code: "X"}).Once()
	tm.EXPECT().IsThemeExist(mock.Anything, "ok").Return(true, nil).Once()
	svc := &inboundClientService{themeMgt: tm}
	assert.ErrorIs(suite.T(), svc.validateThemeID(context.Background(), "missing"), ErrFKThemeNotFound)
	assert.ErrorIs(suite.T(), svc.validateThemeID(context.Background(), "err"), ErrFKThemeNotFound)
	assert.NoError(suite.T(), svc.validateThemeID(context.Background(), "ok"))
	assert.NoError(suite.T(), (&inboundClientService{}).validateThemeID(context.Background(), ""))
}

//nolint:dupl // Theme and layout validators share the same branch structure with type-specific services.
func (suite *InboundClientServiceTestSuite) TestValidateLayoutID_AllBranches() {
	lm := layoutmock.NewLayoutMgtServiceInterfaceMock(suite.T())
	lm.EXPECT().IsLayoutExist(mock.Anything, "missing").Return(false, nil).Once()
	lm.EXPECT().IsLayoutExist(mock.Anything, "err").Return(false, &tidcommon.ServiceError{Code: "X"}).Once()
	lm.EXPECT().IsLayoutExist(mock.Anything, "ok").Return(true, nil).Once()
	svc := &inboundClientService{layoutMgt: lm}
	assert.ErrorIs(suite.T(), svc.validateLayoutID(context.Background(), "missing"), ErrFKLayoutNotFound)
	assert.ErrorIs(suite.T(), svc.validateLayoutID(context.Background(), "err"), ErrFKLayoutNotFound)
	assert.NoError(suite.T(), svc.validateLayoutID(context.Background(), "ok"))
	assert.NoError(suite.T(), (&inboundClientService{}).validateLayoutID(context.Background(), ""))
}

func (suite *InboundClientServiceTestSuite) TestValidateAllowedUserTypes_NoOpWhenEmpty() {
	svc := &inboundClientService{}
	assert.NoError(suite.T(), svc.validateAllowedUserTypes(context.Background(), nil))
}

func (suite *InboundClientServiceTestSuite) TestValidateAllowedUserTypes_AllExist() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetEntityTypeList(mock.Anything, mock.Anything, mock.Anything, 0, false).Return(
		&entitytypepkg.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytypepkg.EntityTypeListItem{{Name: "person"}},
		}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}
	assert.NoError(suite.T(), svc.validateAllowedUserTypes(context.Background(), []string{"person"}))
}

func (suite *InboundClientServiceTestSuite) TestValidateAllowedUserTypes_MissingType() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetEntityTypeList(mock.Anything, mock.Anything, mock.Anything, 0, false).Return(
		&entitytypepkg.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytypepkg.EntityTypeListItem{{Name: "person"}},
		}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}
	err := svc.validateAllowedUserTypes(context.Background(), []string{"ghost"})
	assert.ErrorIs(suite.T(), err, ErrFKInvalidUserType)
}

func (suite *InboundClientServiceTestSuite) TestValidateAllowedUserTypes_EmptyTypeRejected() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetEntityTypeList(mock.Anything, mock.Anything, mock.Anything, 0, false).Return(
		&entitytypepkg.EntityTypeListResponse{TotalResults: 0}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}
	assert.ErrorIs(suite.T(), svc.validateAllowedUserTypes(context.Background(), []string{""}), ErrFKInvalidUserType)
}

func (suite *InboundClientServiceTestSuite) TestValidateAllowedUserTypes_ServiceErrorPropagated() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetEntityTypeList(mock.Anything, mock.Anything, mock.Anything, 0, false).
		Return(nil, &tidcommon.ServiceError{Code: "ERR"})
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}
	err := svc.validateAllowedUserTypes(context.Background(), []string{"a"})
	assert.ErrorIs(suite.T(), err, ErrUserSchemaLookupFailed)
}

// ----- resolveFlowDefaults -----

func (suite *InboundClientServiceTestSuite) TestResolveFlowDefaults_NilOrNoMgtIsNoOp() {
	svc := &inboundClientService{}
	c := validInboundClient()
	assert.NoError(suite.T(), svc.resolveFlowDefaults(context.Background(), &c))

	svc2 := &inboundClientService{flowMgt: nil}
	c2 := validInboundClient()
	assert.NoError(suite.T(), svc2.resolveFlowDefaults(context.Background(), &c2))
}

func (suite *InboundClientServiceTestSuite) TestResolveFlowDefaults_RecoveryFlowDisabledWhenEmpty() {
	c := &inboundmodel.InboundClient{
		ID:             "p1",
		AuthFlowID:     "auth-1",
		RecoveryFlowID: "",
	}
	svc := &inboundClientService{}
	err := svc.resolveFlowDefaults(context.Background(), c)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), c.IsRecoveryFlowEnabled)
}

func (suite *InboundClientServiceTestSuite) TestResolveFlowDefaults_RecoveryFlowEnabledWhenPopulated() {
	c := &inboundmodel.InboundClient{
		ID:                    "p1",
		AuthFlowID:            "auth-1",
		RecoveryFlowID:        "recovery-1",
		IsRecoveryFlowEnabled: true,
	}
	svc := &inboundClientService{}
	err := svc.resolveFlowDefaults(context.Background(), c)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), c.IsRecoveryFlowEnabled)
	assert.Equal(suite.T(), "recovery-1", c.RecoveryFlowID)
}

// ----- ResolveInboundAuthProfileHandles -----

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_NilFlowMgtIsNoOp() {
	svc := &inboundClientService{}
	profile := &providers.InboundAuthProfile{AuthFlowHandle: "some-handle"}
	assert.NoError(suite.T(), svc.ResolveInboundAuthProfileHandles(context.Background(), profile))
	assert.Empty(suite.T(), profile.AuthFlowID)
}

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_ResolvesAuthFlowHandle() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().GetFlowByHandle(mock.Anything, "auth-handle", providers.FlowTypeAuthentication).
		Return(&providers.CompleteFlowDefinition{ID: "auth-id"}, nil).Once()
	svc := &inboundClientService{flowMgt: flowMgt}
	profile := &providers.InboundAuthProfile{AuthFlowHandle: "auth-handle"}
	assert.NoError(suite.T(), svc.ResolveInboundAuthProfileHandles(context.Background(), profile))
	assert.Equal(suite.T(), "auth-id", profile.AuthFlowID)
}

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_AuthFlowHandleNotFound() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().GetFlowByHandle(mock.Anything, "bad-handle", providers.FlowTypeAuthentication).
		Return(nil, &tidcommon.ServiceError{Code: "NOT_FOUND"}).Once()
	svc := &inboundClientService{flowMgt: flowMgt}
	profile := &providers.InboundAuthProfile{AuthFlowHandle: "bad-handle"}
	assert.ErrorIs(suite.T(), svc.ResolveInboundAuthProfileHandles(context.Background(), profile), ErrFKInvalidAuthFlow)
}

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_ResolvesRegistrationFlowHandle() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().GetFlowByHandle(mock.Anything, "reg-handle", providers.FlowTypeRegistration).
		Return(&providers.CompleteFlowDefinition{ID: "reg-id"}, nil).Once()
	svc := &inboundClientService{flowMgt: flowMgt}
	profile := &providers.InboundAuthProfile{RegistrationFlowHandle: "reg-handle"}
	assert.NoError(suite.T(), svc.ResolveInboundAuthProfileHandles(context.Background(), profile))
	assert.Equal(suite.T(), "reg-id", profile.RegistrationFlowID)
}

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_RegistrationFlowHandleNotFound() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().GetFlowByHandle(mock.Anything, "bad-reg", providers.FlowTypeRegistration).
		Return(nil, &tidcommon.ServiceError{Code: "NOT_FOUND"}).Once()
	svc := &inboundClientService{flowMgt: flowMgt}
	profile := &providers.InboundAuthProfile{RegistrationFlowHandle: "bad-reg"}
	err := svc.ResolveInboundAuthProfileHandles(context.Background(), profile)
	assert.ErrorIs(suite.T(), err, ErrFKInvalidRegistrationFlow)
}

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_ResolvesRecoveryFlowHandle() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().GetFlowByHandle(mock.Anything, "rec-handle", providers.FlowTypeRecovery).
		Return(&providers.CompleteFlowDefinition{ID: "rec-id"}, nil).Once()
	svc := &inboundClientService{flowMgt: flowMgt}
	profile := &providers.InboundAuthProfile{RecoveryFlowHandle: "rec-handle"}
	assert.NoError(suite.T(), svc.ResolveInboundAuthProfileHandles(context.Background(), profile))
	assert.Equal(suite.T(), "rec-id", profile.RecoveryFlowID)
}

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_RecoveryFlowHandleNotFound() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().GetFlowByHandle(mock.Anything, "bad-rec", providers.FlowTypeRecovery).
		Return(nil, &tidcommon.ServiceError{Code: "NOT_FOUND"}).Once()
	svc := &inboundClientService{flowMgt: flowMgt}
	profile := &providers.InboundAuthProfile{RecoveryFlowHandle: "bad-rec"}
	err := svc.ResolveInboundAuthProfileHandles(context.Background(), profile)
	assert.ErrorIs(suite.T(), err, ErrFKInvalidRecoveryFlow)
}

func (suite *InboundClientServiceTestSuite) TestResolveInboundAuthProfileHandles_SkipsWhenIDAlreadySet() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	svc := &inboundClientService{flowMgt: flowMgt}
	profile := &providers.InboundAuthProfile{
		AuthFlowID:             "existing-auth",
		AuthFlowHandle:         "auth-handle",
		RegistrationFlowID:     "existing-reg",
		RegistrationFlowHandle: "reg-handle",
		RecoveryFlowID:         "existing-rec",
		RecoveryFlowHandle:     "rec-handle",
	}
	assert.NoError(suite.T(), svc.ResolveInboundAuthProfileHandles(context.Background(), profile))
	assert.Equal(suite.T(), "existing-auth", profile.AuthFlowID)
	assert.Equal(suite.T(), "existing-reg", profile.RegistrationFlowID)
	assert.Equal(suite.T(), "existing-rec", profile.RecoveryFlowID)
	flowMgt.AssertNotCalled(suite.T(), "GetFlowByHandle", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *InboundClientServiceTestSuite) TestCreateInboundClient_WithoutRecoveryFlow() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	store.EXPECT().CreateInboundClient(mock.Anything, mock.MatchedBy(func(c inboundmodel.InboundClient) bool {
		// Verify that when RecoveryFlowID is empty, IsRecoveryFlowEnabled is false
		return c.RecoveryFlowID == "" && !c.IsRecoveryFlowEnabled
	})).Return(nil)

	svc := newServiceForTest(store)
	client := ptrInboundClient()
	client.RecoveryFlowID = ""
	client.IsRecoveryFlowEnabled = false
	err := svc.CreateInboundClient(context.Background(), client, nil, false, "")

	assert.NoError(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestUpdateInboundClient_WithRecoveryFlow() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)
	store.EXPECT().UpdateInboundClient(mock.Anything, mock.MatchedBy(func(c inboundmodel.InboundClient) bool {
		// Verify that when RecoveryFlowID is set, IsRecoveryFlowEnabled can be true
		return c.RecoveryFlowID == "recovery-1" && c.IsRecoveryFlowEnabled
	})).Return(nil)
	store.EXPECT().GetOAuthProfileByEntityID(mock.Anything, "p1").Return(nil, ErrInboundClientNotFound)

	svc := newInboundClientService(store, transaction.NewNoOpTransactioner(), nil, nil, nil, nil, nil, nil, nil)
	client := ptrInboundClient()
	client.RecoveryFlowID = "recovery-1"
	client.IsRecoveryFlowEnabled = true
	err := svc.UpdateInboundClient(context.Background(), client, nil, false, "", "")

	assert.NoError(suite.T(), err)
}

// ----- validateFKs aggregate -----

func (suite *InboundClientServiceTestSuite) TestValidateFKs_NilNoOp() {
	svc := &inboundClientService{}
	assert.NoError(suite.T(), svc.validateFKs(context.Background(), nil))
}

// ----- error wrappers -----

func TestCertOperationError_ErrorAndIsClientError(t *testing.T) {
	e := &CertOperationError{Underlying: &tidcommon.ServiceError{
		Type:             tidcommon.ClientErrorType,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "bad cert"},
	}}
	assert.Equal(t, "bad cert", e.Error())
	assert.True(t, e.IsClientError())

	empty := &CertOperationError{}
	assert.Equal(t, "certificate operation failed", empty.Error())
	assert.False(t, empty.IsClientError())
}

func (suite *InboundClientServiceTestSuite) TestConsentSyncError_ErrorAndIsClientError() {
	e := &ConsentSyncError{Underlying: &tidcommon.ServiceError{
		Type:             tidcommon.ServerErrorType,
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "consent down"},
	}}
	assert.Equal(suite.T(), "consent down", e.Error())
	assert.False(suite.T(), e.IsClientError())

	empty := &ConsentSyncError{}
	assert.Equal(suite.T(), "consent sync failed", empty.Error())
	assert.False(suite.T(), empty.IsClientError())
}

// ----- validateGrantAndResponseTypes branch coverage -----

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_InvalidResponseType() {
	p := &providers.OAuthProfile{
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"bogus_rt"},
	}
	assert.ErrorIs(suite.T(), validateGrantAndResponseTypes(p), ErrOAuthInvalidResponseType)
}

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_ClientCredsWithResponseType() {
	p := &providers.OAuthProfile{
		GrantTypes:    []string{"client_credentials"},
		ResponseTypes: []string{"code"},
	}
	assert.ErrorIs(suite.T(), validateGrantAndResponseTypes(p),
		ErrOAuthClientCredentialsCannotUseResponseTypes)
}

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_AuthCodeMissingCodeRT() {
	p := &providers.OAuthProfile{
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{},
	}
	assert.ErrorIs(suite.T(), validateGrantAndResponseTypes(p),
		ErrOAuthAuthCodeRequiresCodeResponseType)
}

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_RefreshTokenSole() {
	p := &providers.OAuthProfile{
		GrantTypes: []string{"refresh_token"},
	}
	assert.ErrorIs(suite.T(), validateGrantAndResponseTypes(p),
		ErrOAuthRefreshTokenCannotBeSoleGrant)
}

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_PKCEWithoutAuthCode() {
	p := &providers.OAuthProfile{
		GrantTypes:   []string{"client_credentials"},
		PKCERequired: true,
	}
	assert.ErrorIs(suite.T(), validateGrantAndResponseTypes(p), ErrOAuthPKCERequiresAuthCode)
}

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_ResponseTypeWithoutAuthCode() {
	p := &providers.OAuthProfile{
		GrantTypes:    []string{"client_credentials"},
		ResponseTypes: []string{"code"},
	}
	// client_credentials + response_types triggers the earlier rule
	assert.Error(suite.T(), validateGrantAndResponseTypes(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_HappyAuthCode() {
	p := &providers.OAuthProfile{
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
	}
	assert.NoError(suite.T(), validateGrantAndResponseTypes(p))
}

func (suite *InboundClientServiceTestSuite) TestValidateGrantAndResponseTypes_HappyClientCredentials() {
	p := &providers.OAuthProfile{
		GrantTypes: []string{"client_credentials"},
	}
	assert.NoError(suite.T(), validateGrantAndResponseTypes(p))
}

// ----- validatePublicClient branch coverage -----

func (suite *InboundClientServiceTestSuite) TestValidatePublicClient_NonNoneAuthMethod() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "client_secret_basic",
		PKCERequired:            true,
	}
	assert.ErrorIs(suite.T(), validatePublicClient(p), ErrOAuthPublicClientMustUseNoneAuth)
}

func (suite *InboundClientServiceTestSuite) TestValidatePublicClient_HappyPath() {
	p := &providers.OAuthProfile{
		TokenEndpointAuthMethod: "none",
		PKCERequired:            true,
	}
	assert.NoError(suite.T(), validatePublicClient(p))
}

// ----- validateFKs aggregate paths -----

func (suite *InboundClientServiceTestSuite) TestValidateFKs_AuthFlowErrorPropagated() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "bad", providers.FlowTypeAuthentication).Return(false, nil)
	svc := &inboundClientService{flowMgt: flowMgt}
	c := &inboundmodel.InboundClient{AuthFlowID: "bad"}
	assert.ErrorIs(suite.T(), svc.validateFKs(context.Background(), c), ErrFKInvalidAuthFlow)
}

func (suite *InboundClientServiceTestSuite) TestValidateFKs_RecoveryFlowErrorPropagated() {
	flowMgt := flowmgtmock.NewFlowMgtServiceInterfaceMock(suite.T())
	flowMgt.EXPECT().IsValidFlow(mock.Anything, "bad", providers.FlowTypeRecovery).Return(false, nil)
	svc := &inboundClientService{flowMgt: flowMgt}
	c := &inboundmodel.InboundClient{RecoveryFlowID: "bad"}
	assert.ErrorIs(suite.T(), svc.validateFKs(context.Background(), c), ErrFKInvalidRecoveryFlow)
}

func (suite *InboundClientServiceTestSuite) TestValidateFKs_AllPassWithEmptyOptionals() {
	svc := &inboundClientService{}
	c := &inboundmodel.InboundClient{}
	assert.NoError(suite.T(), svc.validateFKs(context.Background(), c))
}

// ----- consent helpers -----

func TestExtractRequestedAttributesFromInbound_AllNil(t *testing.T) {
	out := extractRequestedAttributesFromInbound(nil, nil)
	assert.Empty(t, out)
}

func TestExtractRequestedAttributesFromInbound_FromAssertionOnly(t *testing.T) {
	c := &inboundmodel.InboundClient{
		Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "sub"}},
	}
	out := extractRequestedAttributesFromInbound(c, nil)
	assert.Len(t, out, 2)
	assert.True(t, out["email"])
	assert.True(t, out["sub"])
}

func TestExtractRequestedAttributesFromInbound_DedupsAcrossSources(t *testing.T) {
	c := &inboundmodel.InboundClient{
		Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}},
	}
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"email", "given_name"}},
			},
			IDToken: &providers.IDTokenConfig{UserAttributes: []string{"family_name"}},
		},
		UserInfo: &providers.UserInfoConfig{UserAttributes: []string{"email", "picture"}},
	}
	out := extractRequestedAttributesFromInbound(c, p)
	assert.Len(t, out, 4)
	assert.True(t, out["email"])
	assert.True(t, out["given_name"])
	assert.True(t, out["family_name"])
	assert.True(t, out["picture"])
}

func TestExtractRequestedAttributesFromInbound_NilSubFields(t *testing.T) {
	p := &providers.OAuthProfile{
		Token:    &providers.OAuthTokenConfig{},
		UserInfo: nil,
	}
	out := extractRequestedAttributesFromInbound(nil, p)
	assert.Empty(t, out)
}

func TestAttributesToPurposeElements_EmptyMap(t *testing.T) {
	out := attributesToPurposeElements(map[string]bool{})
	assert.Empty(t, out)
}

func TestAttributesToPurposeElements_PopulatedMap(t *testing.T) {
	out := attributesToPurposeElements(map[string]bool{"email": true, "sub": true})
	assert.Len(t, out, 2)
	for _, el := range out {
		assert.False(t, el.IsMandatory)
	}
}

// ----- wrapConsentServiceError -----

func TestWrapConsentServiceError_NilReturnsNil(t *testing.T) {
	s := &inboundClientService{}
	assert.Nil(t, s.wrapConsentServiceError(nil))
}

func TestWrapConsentServiceError_WrapsServiceError(t *testing.T) {
	s := &inboundClientService{}
	se := &tidcommon.ServiceError{Code: "X", Type: tidcommon.ClientErrorType}
	wrapped := s.wrapConsentServiceError(se)
	var ce *ConsentSyncError
	assert.True(t, errors.As(wrapped, &ce))
	assert.Equal(t, se, ce.Underlying)
}

// ----- validateUniqueInboundClientID -----

func (suite *InboundClientServiceTestSuite) TestValidateUniqueInboundClientID_NotExisting() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().InboundClientExists(mock.Anything, "x").Return(false, nil)
	c := &inboundmodel.InboundClient{ID: "x"}
	assert.NoError(suite.T(), validateUniqueInboundClientID(context.Background(), store, c))
}

func (suite *InboundClientServiceTestSuite) TestValidateUniqueInboundClientID_DuplicateRejected() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().InboundClientExists(mock.Anything, "x").Return(true, nil)
	c := &inboundmodel.InboundClient{ID: "x"}
	err := validateUniqueInboundClientID(context.Background(), store, c)
	assert.ErrorContains(suite.T(), err, "duplicate entity ID")
}

func (suite *InboundClientServiceTestSuite) TestValidateUniqueInboundClientID_StoreErrorPropagated() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().InboundClientExists(mock.Anything, "x").Return(false, errors.New("db down"))
	c := &inboundmodel.InboundClient{ID: "x"}
	err := validateUniqueInboundClientID(context.Background(), store, c)
	assert.ErrorContains(suite.T(), err, "failed to check inbound client existence")
}

// ----- GetOAuthClientByClientID -----

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_NoEntityProvider() {
	svc := newServiceForTest(newInboundClientStoreInterfaceMock(suite.T())).(*inboundClientService)
	got, err := svc.GetOAuthClientByClientID(context.Background(), "client-1")
	assert.ErrorContains(suite.T(), err, "entity provider not configured")
	assert.Nil(suite.T(), got)
}

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_EmptyClientID() {
	ep := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	svc := &inboundClientService{
		entityProvider: ep,
		store:          newInboundClientStoreInterfaceMock(suite.T()),
	}
	got, err := svc.GetOAuthClientByClientID(context.Background(), "")
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), got)
}

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_EntityNotFound() {
	ep := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	ep.EXPECT().IdentifyEntity(mock.Anything).Return(nil, &entityprovider.EntityProviderError{
		Code: entityprovider.ErrorCodeEntityNotFound,
	})
	svc := &inboundClientService{entityProvider: ep, store: newInboundClientStoreInterfaceMock(suite.T())}
	got, err := svc.GetOAuthClientByClientID(context.Background(), "missing")
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), got)
}

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_IdentifyErrorPropagated() {
	ep := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	ep.EXPECT().IdentifyEntity(mock.Anything).Return(nil, &entityprovider.EntityProviderError{
		Code: entityprovider.ErrorCodeSystemError, Message: "boom",
	})
	svc := &inboundClientService{entityProvider: ep, store: newInboundClientStoreInterfaceMock(suite.T())}
	got, err := svc.GetOAuthClientByClientID(context.Background(), "x")
	assert.ErrorContains(suite.T(), err, "failed to resolve client_id")
	assert.Nil(suite.T(), got)
}

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_NilEntityID() {
	ep := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	ep.EXPECT().IdentifyEntity(mock.Anything).Return(nil, nil)
	svc := &inboundClientService{entityProvider: ep, store: newInboundClientStoreInterfaceMock(suite.T())}
	got, err := svc.GetOAuthClientByClientID(context.Background(), "x")
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), got)
}

const testServiceEntityID = "ent-1"

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_GetEntityNotFound() {
	id := testServiceEntityID
	ep := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	ep.EXPECT().IdentifyEntity(mock.Anything).Return(&id, nil)
	ep.EXPECT().GetEntity(id).Return(nil, &entityprovider.EntityProviderError{
		Code: entityprovider.ErrorCodeEntityNotFound,
	})
	svc := &inboundClientService{entityProvider: ep, store: newInboundClientStoreInterfaceMock(suite.T())}
	got, err := svc.GetOAuthClientByClientID(context.Background(), "x")
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), got)
}

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_OAuthProfileNotFoundReturnsNil() {
	id := testServiceEntityID
	ep := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	ep.EXPECT().IdentifyEntity(mock.Anything).Return(&id, nil)
	ep.EXPECT().GetEntity(id).Return(&providers.Entity{ID: id, OUID: "ou-1"}, nil)

	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().GetOAuthProfileByEntityID(mock.Anything, id).Return(nil, ErrInboundClientNotFound)

	svc := &inboundClientService{entityProvider: ep, store: store}
	got, err := svc.GetOAuthClientByClientID(context.Background(), "x")
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), got)
}

func (suite *InboundClientServiceTestSuite) TestGetOAuthClientByClientID_StoreErrorPropagated() {
	id := testServiceEntityID
	ep := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	ep.EXPECT().IdentifyEntity(mock.Anything).Return(&id, nil)
	ep.EXPECT().GetEntity(id).Return(&providers.Entity{ID: id, OUID: "ou-1"}, nil)

	storeErr := errors.New("db down")
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().GetOAuthProfileByEntityID(mock.Anything, id).Return(nil, storeErr)

	svc := &inboundClientService{entityProvider: ep, store: store}
	got, err := svc.GetOAuthClientByClientID(context.Background(), "x")
	assert.ErrorIs(suite.T(), err, storeErr)
	assert.Nil(suite.T(), got)
}

func (suite *InboundClientServiceTestSuite) TestCollectConfiguredUserAttributes_AllNil() {
	out := collectConfiguredUserAttributes(nil, nil)
	assert.Empty(suite.T(), out)
}

func (suite *InboundClientServiceTestSuite) TestCollectConfiguredUserAttributes_AssertionOnly() {
	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "name"}}
	out := collectConfiguredUserAttributes(assertion, nil)
	assert.Len(suite.T(), out, 2)
	assert.True(suite.T(), out["email"])
	assert.True(suite.T(), out["name"])
}

func (suite *InboundClientServiceTestSuite) TestCollectConfiguredUserAttributes_AccessTokenOnly() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"email"}},
			},
		},
	}
	out := collectConfiguredUserAttributes(nil, p)
	assert.Len(suite.T(), out, 1)
	assert.True(suite.T(), out["email"])
}

func (suite *InboundClientServiceTestSuite) TestCollectConfiguredUserAttributes_IDTokenOnly() {
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			IDToken: &providers.IDTokenConfig{UserAttributes: []string{"sub"}},
		},
	}
	out := collectConfiguredUserAttributes(nil, p)
	assert.Len(suite.T(), out, 1)
	assert.True(suite.T(), out["sub"])
}

func (suite *InboundClientServiceTestSuite) TestCollectConfiguredUserAttributes_UserInfoOnly() {
	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{UserAttributes: []string{"phone"}},
	}
	out := collectConfiguredUserAttributes(nil, p)
	assert.Len(suite.T(), out, 1)
	assert.True(suite.T(), out["phone"])
}

func (suite *InboundClientServiceTestSuite) TestCollectConfiguredUserAttributes_DedupsAcrossAllSources() {
	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}}
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"email", "name"}},
			},
			IDToken: &providers.IDTokenConfig{UserAttributes: []string{"name", "phone"}},
		},
		UserInfo: &providers.UserInfoConfig{UserAttributes: []string{"email", "picture"}},
	}
	out := collectConfiguredUserAttributes(assertion, p)
	assert.Len(suite.T(), out, 4)
	assert.True(suite.T(), out["email"])
	assert.True(suite.T(), out["name"])
	assert.True(suite.T(), out["phone"])
	assert.True(suite.T(), out["picture"])
}

func (suite *InboundClientServiceTestSuite) TestCollectConfiguredUserAttributes_NilSubFields() {
	p := &providers.OAuthProfile{
		Token:    &providers.OAuthTokenConfig{},
		UserInfo: nil,
	}
	out := collectConfiguredUserAttributes(nil, p)
	assert.Empty(suite.T(), out)
}

// ----- validateUserAttributesAgainstAllowedTypes -----

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_NoOpWhenNoAllowedTypes() {
	svc := &inboundClientService{}
	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}}
	assert.NoError(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), nil, assertion, nil))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_NoOpWhenNoEntityTypeService() {
	svc := &inboundClientService{}
	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}}
	assert.NoError(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, assertion, nil))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_NoOpWhenNoAttributesConfigured() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}
	assert.NoError(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, nil, nil))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_ValidAssertionAttribute() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}, {Attribute: "name"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}}
	assert.NoError(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, assertion, nil))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_InvalidAssertionAttribute() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"banana"}}
	assert.ErrorIs(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, assertion, nil), ErrInvalidUserAttribute)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_ValidAccessTokenAttribute() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"email"}},
			},
		},
	}
	assert.NoError(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, nil, p))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_InvalidAccessTokenAttribute() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"unknown_attr"}},
			},
		},
	}
	assert.ErrorIs(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, nil, p), ErrInvalidUserAttribute)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_InvalidIDTokenAttribute() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			IDToken: &providers.IDTokenConfig{UserAttributes: []string{"ghost"}},
		},
	}
	assert.ErrorIs(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, nil, p), ErrInvalidUserAttribute)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_InvalidUserInfoAttribute() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	p := &providers.OAuthProfile{
		UserInfo: &providers.UserInfoConfig{UserAttributes: []string{"ghost"}},
	}
	assert.ErrorIs(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, nil, p), ErrInvalidUserAttribute)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_ClientErrorMapsToFKError() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ERR"})
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}}
	assert.ErrorIs(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, assertion, nil), ErrFKInvalidUserType)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_ServerErrorMapsToLookupFailed() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SRV"})
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}}
	assert.ErrorIs(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, assertion, nil), ErrUserSchemaLookupFailed)
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_UnionAcrossMultipleTypes() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "contractor", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "agency_name"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	// "agency_name" only exists in contractor — still valid because union semantics are used.
	assertion := &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "agency_name"}}
	assert.NoError(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee", "contractor"}, assertion, nil))
}

func (suite *InboundClientServiceTestSuite) TestValidateUserAttributes_ComputedAttributesSkipSchemaCheck() {
	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)
	svc := &inboundClientService{entityType: us, logger: log.GetLogger()}

	// Computed attributes (groups, roles, ouId, ouName, ouHandle, userType) are derived at runtime
	// and are not in the entity schema — they must be accepted without failing validation.
	p := &providers.OAuthProfile{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{
					Attributes: []string{"email", "groups", "ouId", "ouName", "ouHandle", "roles", "userType"},
				},
			},
			IDToken: &providers.IDTokenConfig{
				UserAttributes: []string{"groups", "ouId"},
			},
		},
		UserInfo: &providers.UserInfoConfig{
			UserAttributes: []string{"groups", "roles"},
		},
	}
	assert.NoError(suite.T(), svc.validateUserAttributesAgainstAllowedTypes(
		context.Background(), []string{"employee"}, nil, p))
}

// ----- CreateInboundClient / UpdateInboundClient / Validate — user attribute validation wired in -----

func (suite *InboundClientServiceTestSuite) TestCreateInboundClient_RejectsInvalidUserAttribute() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)

	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	// validateAllowedUserTypes (called by validateFKs) checks entity type existence via GetEntityTypeList.
	us.EXPECT().GetEntityTypeList(mock.Anything, mock.Anything, mock.Anything, 0, false).Return(
		&entitytypepkg.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytypepkg.EntityTypeListItem{{Name: "employee"}},
		}, nil)
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)

	svc := newInboundClientService(store, transaction.NewNoOpTransactioner(), nil, nil, nil, nil, nil, us, nil)

	c := validInboundClient()
	c.AllowedUserTypes = []string{"employee"}
	c.Assertion = &inboundmodel.AssertionConfig{UserAttributes: []string{"not_a_real_attr"}}

	err := svc.CreateInboundClient(context.Background(), &c, nil, false, "")
	assert.ErrorIs(suite.T(), err, ErrInvalidUserAttribute)
}

func (suite *InboundClientServiceTestSuite) TestUpdateInboundClient_RejectsInvalidUserAttribute() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().IsDeclarative(mock.Anything, "p1").Return(false)

	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	// validateAllowedUserTypes (called by validateFKs) checks entity type existence via GetEntityTypeList.
	us.EXPECT().GetEntityTypeList(mock.Anything, mock.Anything, mock.Anything, 0, false).Return(
		&entitytypepkg.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytypepkg.EntityTypeListItem{{Name: "employee"}},
		}, nil)
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)

	svc := newInboundClientService(store, transaction.NewNoOpTransactioner(), nil, nil, nil, nil, nil, us, nil)

	c := validInboundClient()
	c.AllowedUserTypes = []string{"employee"}
	p := validOAuthProfileData()
	p.UserInfo = &providers.UserInfoConfig{UserAttributes: []string{"ghost"}}

	err := svc.UpdateInboundClient(context.Background(), &c, p, true, "", "")
	assert.ErrorIs(suite.T(), err, ErrInvalidUserAttribute)
}

func (suite *InboundClientServiceTestSuite) TestValidate_RejectsInvalidUserAttribute() {
	store := newInboundClientStoreInterfaceMock(suite.T())

	us := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	// validateAllowedUserTypes (called by validateFKs) checks entity type existence via GetEntityTypeList.
	us.EXPECT().GetEntityTypeList(mock.Anything, mock.Anything, mock.Anything, 0, false).Return(
		&entitytypepkg.EntityTypeListResponse{
			TotalResults: 1,
			Types:        []entitytypepkg.EntityTypeListItem{{Name: "employee"}},
		}, nil)
	us.EXPECT().GetAttributes(mock.Anything, entitytypepkg.TypeCategoryUser, "employee", false, true, false).
		Return([]entitytypepkg.AttributeInfo{{Attribute: "email"}}, nil)

	svc := newInboundClientService(store, transaction.NewNoOpTransactioner(), nil, nil, nil, nil, nil, us, nil)

	c := validInboundClient()
	c.AllowedUserTypes = []string{"employee"}
	p := validOAuthProfileData()
	p.Token = &providers.OAuthTokenConfig{
		AccessToken: &providers.AccessTokenConfig{
			UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"bad_attr"}},
		},
	}

	err := svc.Validate(context.Background(), &c, p, true)
	assert.ErrorIs(suite.T(), err, ErrInvalidUserAttribute)
}

func newInboundClientServiceWithConsent(consentSvc consent.ConsentServiceInterface) *inboundClientService {
	svc := newInboundClientService(
		nil, transaction.NewNoOpTransactioner(), nil, nil, nil, nil, nil, nil, consentSvc,
	)
	return svc.(*inboundClientService)
}

// ----- syncConsentOnUpdate filters to attribute purposes only -----

func (suite *InboundClientServiceTestSuite) TestSyncConsentOnUpdate_IgnoresPermissionPurposeWhenSearchingForExisting() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	// ListConsentPurposes returns a permission purpose for the same app — must be filtered out.
	cm.EXPECT().ListConsentPurposes(mock.Anything, "default", "app1").Return([]consent.ConsentPurpose{
		{ID: "perm-p", Namespace: providers.NamespacePermission},
	}, nil)
	cm.EXPECT().ValidateConsentElements(mock.Anything, "default", []string{"email"}).
		Return([]string{"email"}, nil)
	// Since no attribute purpose exists, a NEW one must be created (Create, not Update).
	cm.EXPECT().CreateConsentPurpose(mock.Anything, "default",
		mock.MatchedBy(func(input *consent.ConsentPurposeInput) bool {
			return input.GroupID == "app1" && input.Name == consent.AttributesPurposeName("app1")
		})).Return(&consent.ConsentPurpose{ID: "attr-new"}, nil)

	svc := newInboundClientServiceWithConsent(cm)
	client := &inboundmodel.InboundClient{Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email"}}}
	err := svc.syncConsentOnUpdate(context.Background(), "app1", "App 1", client, nil)
	assert.NoError(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestSyncConsentOnUpdate_SkipsUpdateWhenAttributeSetUnchanged() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.MatchedBy(func(names []string) bool {
		if len(names) != 2 {
			return false
		}
		got := map[string]bool{}
		for _, n := range names {
			got[n] = true
		}
		return got["email"] && got["given_name"]
	})).Return([]string{"email", "given_name"}, nil)
	cm.EXPECT().ListConsentPurposes(mock.Anything, "default", "app1").Return([]consent.ConsentPurpose{
		{
			ID:        "attr-p",
			Namespace: providers.NamespaceAttribute,
			// Elements returned by the consent service do not carry a per-element Namespace.
			Elements: []consent.PurposeElement{
				{Name: "email"},
				{Name: "given_name"},
			},
		},
	}, nil)
	// Crucially, no UpdateConsentPurpose expectation — the mock would fail if it were called.

	svc := newInboundClientServiceWithConsent(cm)
	client := &inboundmodel.InboundClient{
		Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "given_name"}},
	}
	err := svc.syncConsentOnUpdate(context.Background(), "app1", "App 1", client, nil)
	assert.NoError(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestSyncConsentOnUpdate_UpdatesWhenAttributeSetChanged() {
	cm := consentmock.NewConsentServiceInterfaceMock(suite.T())
	cm.EXPECT().ValidateConsentElements(mock.Anything, "default", mock.Anything).
		Return([]string{"email", "family_name"}, nil)
	cm.EXPECT().ListConsentPurposes(mock.Anything, "default", "app1").Return([]consent.ConsentPurpose{
		{
			ID:        "attr-p",
			Namespace: providers.NamespaceAttribute,
			Elements: []consent.PurposeElement{
				{Name: "email"},
				{Name: "given_name"},
			},
		},
	}, nil)
	cm.EXPECT().UpdateConsentPurpose(mock.Anything, "default", "attr-p",
		mock.MatchedBy(func(input *consent.ConsentPurposeInput) bool {
			if input.GroupID != "app1" {
				return false
			}
			names := map[string]bool{}
			for _, el := range input.Elements {
				names[el.Name] = true
			}
			return len(names) == 2 && names["email"] && names["family_name"]
		})).Return(&consent.ConsentPurpose{ID: "attr-p"}, nil)

	svc := newInboundClientServiceWithConsent(cm)
	client := &inboundmodel.InboundClient{
		Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "family_name"}},
	}
	err := svc.syncConsentOnUpdate(context.Background(), "app1", "App 1", client, nil)
	assert.NoError(suite.T(), err)
}

// --- GetEntityIDsByThemeID service tests ---

func (suite *InboundClientServiceTestSuite) TestGetEntityIDsByThemeID_NegativeLimit() {
	svc := newServiceForTest(newInboundClientStoreInterfaceMock(suite.T()))
	_, _, err := svc.GetEntityIDsByReference(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1", -1, 0)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "limit")
}

func (suite *InboundClientServiceTestSuite) TestGetEntityIDsByThemeID_NegativeOffset() {
	svc := newServiceForTest(newInboundClientStoreInterfaceMock(suite.T()))
	_, _, err := svc.GetEntityIDsByReference(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1", 10, -1)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "offset")
}

func (suite *InboundClientServiceTestSuite) TestGetEntityIDsByThemeID_StoreError() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().GetEntityIDsByReference(mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", 10, 0).
		Return(nil, 0, errors.New("db error"))
	svc := newServiceForTest(store)
	_, _, err := svc.GetEntityIDsByReference(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1", 10, 0)
	assert.Error(suite.T(), err)
}

func (suite *InboundClientServiceTestSuite) TestGetEntityIDsByThemeID_Success() {
	store := newInboundClientStoreInterfaceMock(suite.T())
	store.EXPECT().GetEntityIDsByReference(mock.Anything, resourcedependency.ResourceTypeTheme, "theme-1", 10, 0).
		Return([]string{"app-1", "app-2"}, 2, nil)
	svc := newServiceForTest(store)
	ids, total, err := svc.GetEntityIDsByReference(
		context.Background(), resourcedependency.ResourceTypeTheme, "theme-1", 10, 0)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 2, total)
	assert.Equal(suite.T(), []string{"app-1", "app-2"}, ids)
}
