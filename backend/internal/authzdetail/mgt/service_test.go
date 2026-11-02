// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

// flightSchema is a registered schema for a flight booking type.
const flightSchema = `{
  "type": "object",
  "required": ["route", "maxFare"],
  "properties": {
    "locations": {"type": "array", "items": {"type": "string"}},
    "actions":   {"type": "array", "items": {"type": "string"}},
    "route":   {"type": "string", "pattern": "^[A-Z]{3}-[A-Z]{3}$"},
    "maxFare": {"type": "object", "properties": {
      "amount":   {"type": "number", "title": "Spend cap", "policy": {
        "modification": "decrease", "modifiableBy": ["user", "client"]}},
      "currency": {"type": "string"}}}
  }
}`

const (
	// shortConsentTemplate is a consent template that differs from the flight type's.
	shortConsentTemplate = "Book a {{route}} flight"

	testFlightType   = "https://scheme.example.com/flight/v1"
	testPaymentType  = "https://scheme.example.com/payment/v1"
	testProcessorURL = "https://bank.example.com/rar/v1/process"
)

// paymentSchema is a registered schema whose debtor account is filled by the processor.
const paymentSchema = `{
  "type": "object",
  "required": ["amount"],
  "properties": {
    "amount": {"type": "number"},
    "debtorAccount": {"type": "object", "properties": {
      "iban": {"type": "string", "policy": {"suppliedBy": "processor"}}}}
  }
}`

type ServiceTestSuite struct {
	suite.Suite
	ctx       context.Context
	typeStore *typeStoreInterfaceMock
	service   *authorizationDetailMgtService
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupSuite() {
	if !config.IsServerRuntimeInitialized() {
		s.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
	}
}

func (s *ServiceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.typeStore = newTypeStoreInterfaceMock(s.T())
	s.service = newAuthorizationDetailMgtService(s.typeStore, nil).(*authorizationDetailMgtService)
}

func flightType() providers.AuthorizationDetailType {
	return providers.AuthorizationDetailType{
		ID: "type-1", Type: testFlightType, Name: "Flight booking", Version: 1,
		Schema:          json.RawMessage(flightSchema),
		ConsentTemplate: "Book a {{route}} flight up to {{maxFare.amount}} {{maxFare.currency}}",
		Reuse:           true,
	}
}

func paymentType() providers.AuthorizationDetailType {
	return providers.AuthorizationDetailType{
		ID: "type-2", Type: testPaymentType, Name: "Payment", Version: 3,
		Schema:    json.RawMessage(paymentSchema),
		Processor: &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL},
		Reuse:     true,
	}
}

// apiType returns a type as the management API passes it, without an id.
func apiType(t providers.AuthorizationDetailType) providers.AuthorizationDetailType {
	t.ID = ""
	return t
}

// Validation checks the definition and the processor credential the way the write would, and changes nothing.
func (s *ServiceTestSuite) TestValidateType() {
	withTestCrypto(s.T())
	invalid := flightType()
	invalid.Schema = json.RawMessage(`{"type":"object","properties":{"tags":{"type":"array"}}}`)
	svcErr := s.service.ValidateAuthorizationDetailType(s.ctx, invalid)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidSchema.Code, svcErr.Code)

	masked, maskErr := sentAuthenticationProperties(bearer("******"))
	s.Require().Nil(maskErr)
	created := paymentType()
	created.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL,
		AuthenticationProperties: masked}
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, created.ID).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	svcErr = s.service.ValidateAuthorizationDetailType(s.ctx, created)
	s.Require().NotNil(svcErr, "a new type has no stored credential for a masked one to keep")
	s.Equal(ErrorInvalidProcessorAuthentication.Code, svcErr.Code)
	s.Equal(masked, created.Processor.AuthenticationProperties, "the caller's type is left as it was sent")

	stored := paymentType()
	stored.Processor = storedBearer(s.T())
	kept := paymentType()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, stored.ID).Return(stored, nil).Once()
	s.Nil(s.service.ValidateAuthorizationDetailType(s.ctx, kept), "an omitted credential keeps the stored one")

	moved := paymentType()
	moved.Processor = &providers.AuthorizationDetailProcessor{Endpoint: "https://other.example.com/rar"}
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, stored.ID).Return(stored, nil).Once()
	svcErr = s.service.ValidateAuthorizationDetailType(s.ctx, moved)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProcessorAuthenticationRequired.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCreateType_Success() {
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound)
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.MatchedBy(func(t providers.AuthorizationDetailType) bool {
		return t.ID != "" && t.Version == 1
	})).Return(nil)

	created, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, apiType(flightType()))
	s.Nil(svcErr)
	s.Equal(1, created.Version)
}

func (s *ServiceTestSuite) TestCreateType_KeepsSuppliedID() {
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound)
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(providers.AuthorizationDetailType{},
		errTypeNotFound).Once()
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.MatchedBy(func(t providers.AuthorizationDetailType) bool {
		return t.ID == "type-1"
	})).Return(nil).Once()
	created, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, flightType())
	s.Nil(svcErr)
	s.Equal("type-1", created.ID)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(paymentType(), nil).Once()
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, flightType())
	s.Equal(ErrorTypeIDConflict.Code, svcErr.Code)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").
		Return(providers.AuthorizationDetailType{}, errors.New("db down")).Once()
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, flightType())
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	tooLong := flightType()
	tooLong.ID = strings.Repeat("a", maxTypeIDLength+1)
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, tooLong)
	s.Require().NotNil(svcErr, "an id wider than the store's ID column is refused")
	s.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCreateType_DeclarativeMode() {
	runtime := config.GetServerRuntime()
	original := runtime.Config.AuthorizationDetailType.Store
	runtime.Config.AuthorizationDetailType.Store = "declarative"
	defer func() { runtime.Config.AuthorizationDetailType.Store = original }()

	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, apiType(flightType()))
	s.Equal(ErrorDeclarativeModeCreateNotAllowed.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCreateType_ValidationErrors() {
	mutate := func(fn func(*providers.AuthorizationDetailType)) providers.AuthorizationDetailType {
		t := flightType()
		fn(&t)
		return t
	}
	cases := []struct {
		name string
		t    providers.AuthorizationDetailType
		code string
	}{
		{"type with space", mutate(func(t *providers.AuthorizationDetailType) { t.Type = "a b" }),
			ErrorInvalidTypeValue.Code},
		{"type too long", mutate(func(t *providers.AuthorizationDetailType) { t.Type = strings.Repeat("a", 513) }),
			ErrorInvalidTypeValue.Code},
		{"empty name", mutate(func(t *providers.AuthorizationDetailType) { t.Name = "" }), ErrorInvalidName.Code},
		{"blank name", mutate(func(t *providers.AuthorizationDetailType) { t.Name = "   " }), ErrorInvalidName.Code},
		{"name too long", mutate(func(t *providers.AuthorizationDetailType) { t.Name = strings.Repeat("க", 101) }),
			ErrorInvalidName.Code},
		{"invalid schema",
			mutate(func(t *providers.AuthorizationDetailType) { t.Schema = json.RawMessage(`{"type":"string"}`) }),
			ErrorInvalidSchema.Code},
		{"long template", mutate(func(t *providers.AuthorizationDetailType) {
			t.ConsentTemplate = strings.Repeat("x",
				1001)
		}),
			ErrorInvalidConsentTemplate.Code},
		{"template naming an undeclared field", mutate(func(t *providers.AuthorizationDetailType) {
			t.ConsentTemplate = "Book {{route}} for {{passenger}}"
		}), ErrorInvalidConsentTemplate.Code},
		{"invalid field policy", mutate(func(t *providers.AuthorizationDetailType) {
			t.Schema = json.RawMessage(`{"type":"object","properties":{"a":{"type":"string",` +
				`"policy":{"modification":"decrease","modifiableBy":["client"]}}}}`)
		}), ErrorInvalidSchema.Code},
		{"processor rules without a processor", mutate(func(t *providers.AuthorizationDetailType) {
			t.Schema = json.RawMessage(`{"type":"object","properties":{"iban":{"type":"string",` +
				`"policy":{"suppliedBy":"processor"}}}}`)
			t.ConsentTemplate = ""
		}), ErrorInvalidSchema.Code},
		{"plain http processor", mutate(func(t *providers.AuthorizationDetailType) {
			t.Processor = &providers.AuthorizationDetailProcessor{Endpoint: "http://bank.example.com/p"}
		}), ErrorInvalidProcessor.Code},
		{"processor with userinfo", mutate(func(t *providers.AuthorizationDetailType) {
			t.Processor = &providers.AuthorizationDetailProcessor{Endpoint: "https://u:p@bank.example.com/p"}
		}), ErrorInvalidProcessor.Code},
		{"schema allows another type value", mutate(func(t *providers.AuthorizationDetailType) {
			t.Schema = json.RawMessage(`{"type":"object","properties":{"type":{"const":"other"}}}`)
			t.ConsentTemplate = ""
		}), ErrorInvalidSchema.Code},
		{"schema too large", mutate(func(t *providers.AuthorizationDetailType) {
			t.Schema = json.RawMessage(`{"type":"object","description":"` + strings.Repeat("x", maxSchemaLength) + `"}`)
			t.ConsentTemplate = ""
		}), ErrorInvalidSchema.Code},
		{"text in braces that names no field", mutate(func(t *providers.AuthorizationDetailType) {
			t.ConsentTemplate = "Book {{route}} for {{max fare}}"
		}), ErrorInvalidConsentTemplate.Code},
		{"template with a bidirectional formatting character", mutate(func(t *providers.AuthorizationDetailType) {
			t.ConsentTemplate = "Book {{route}} \u202eexe"
		}), ErrorInvalidConsentTemplate.Code},
	}
	for _, tc := range cases {
		_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, tc.t)
		s.Require().NotNil(svcErr, tc.name)
		s.Equal(tc.code, svcErr.Code, tc.name)
	}

	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, mutate(func(t *providers.AuthorizationDetailType) {
		t.ConsentTemplate = "Book {{passenger}}"
	}))
	s.Equal("The consent template is not valid: {{passenger}} is not a field of the schema",
		svcErr.ErrorDescription.String())
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, mutate(func(t *providers.AuthorizationDetailType) {
		t.Schema = json.RawMessage(`{"type":"object","properties":{"a.b":{"type":"string"}}}`)
	}))
	s.Contains(svcErr.ErrorDescription.String(), "must be non-empty and contain no dot", "the schema error says why")
}

func (s *ServiceTestSuite) TestCreateType_ConflictAndStoreErrors() {
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).Return(flightType(), nil).Once()
	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, flightType())
	s.Equal(ErrorTypeConflict.Code, svcErr.Code)

	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
		Return(providers.AuthorizationDetailType{}, errors.New("db down")).Once()
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, flightType())
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.Anything).Return(errors.New("insert failed")).Once()
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, apiType(flightType()))
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestGetType() {
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
	got, svcErr := s.service.GetAuthorizationDetailType(s.ctx, "type-1")
	s.Nil(svcErr)
	s.Equal(testFlightType, got.Type)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "nope").
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	_, svcErr = s.service.GetAuthorizationDetailType(s.ctx, "nope")
	s.Equal(ErrorTypeNotFound.Code, svcErr.Code)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "err").
		Return(providers.AuthorizationDetailType{}, errors.New("db down")).Once()
	_, svcErr = s.service.GetAuthorizationDetailType(s.ctx, "err")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestListTypes() {
	second := paymentType()
	s.typeStore.EXPECT().CountTypes(mock.Anything).Return(2, nil).Twice()
	s.typeStore.EXPECT().ListTypePage(mock.Anything, 30, 0).
		Return([]providers.AuthorizationDetailType{flightType(), second}, nil).Once()
	s.typeStore.EXPECT().ListTypePage(mock.Anything, 1, 1).
		Return([]providers.AuthorizationDetailType{second}, nil).Once()
	page, svcErr := s.service.ListAuthorizationDetailTypes(s.ctx, 30, 0)
	s.Nil(svcErr)
	s.Equal(2, page.TotalResults)
	s.Len(page.Types, 2)
	page, svcErr = s.service.ListAuthorizationDetailTypes(s.ctx, 1, 1)
	s.Nil(svcErr)
	s.Equal(2, page.TotalResults)
	s.Equal([]providers.AuthorizationDetailType{second}, page.Types)

	for _, bad := range [][2]int{{0, 0}, {101, 0}, {10, -1}} {
		_, svcErr = s.service.ListAuthorizationDetailTypes(s.ctx, bad[0], bad[1])
		s.Equal(ErrorInvalidPagination.Code, svcErr.Code)
	}

	s.typeStore.EXPECT().CountTypes(mock.Anything).Return(0, errors.New("db down")).Once()
	_, svcErr = s.service.ListAuthorizationDetailTypes(s.ctx, 30, 0)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.typeStore.EXPECT().CountTypes(mock.Anything).Return(2, nil).Once()
	s.typeStore.EXPECT().ListTypePage(mock.Anything, 30, 0).Return(nil, errors.New("db down")).Once()
	_, svcErr = s.service.ListAuthorizationDetailTypes(s.ctx, 30, 0)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.typeStore.EXPECT().CountTypes(mock.Anything).Return(0, errResultLimitExceededInCompositeMode).Once()
	_, svcErr = s.service.ListAuthorizationDetailTypes(s.ctx, 30, 0)
	s.Equal(ErrorResultLimitExceededInCompositeMode.Code, svcErr.Code)

	s.typeStore.EXPECT().CountTypes(mock.Anything).Return(2000, nil).Once()
	s.typeStore.EXPECT().ListTypePage(mock.Anything, 30, 0).Return(nil, errResultLimitExceededInCompositeMode).Once()
	_, svcErr = s.service.ListAuthorizationDetailTypes(s.ctx, 30, 0)
	s.Equal(ErrorResultLimitExceededInCompositeMode.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestUpdateType() {
	updated := flightType()
	updated.Version = 2
	changed := flightType()
	changed.ConsentTemplate = shortConsentTemplate
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.MatchedBy(func(t providers.AuthorizationDetailType) bool {
		return t.Version == 1
	}), true).Return(nil).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(updated, nil).Once()
	got, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", changed)
	s.Nil(svcErr)
	s.Equal(2, got.Version)
}

// An update the store refuses because the type's version moved since it was read is a conflict, and one the
// store refuses because the type was deleted meanwhile is not found.
func (s *ServiceTestSuite) TestUpdateType_RefusesATypeChangedMeanwhile() {
	changed := flightType()
	changed.ConsentTemplate = shortConsentTemplate
	moved := flightType()
	moved.Version = 2
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, true).Return(errTypeNotFound).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(moved, nil).Once()
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", changed)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorTypeChanged.Code, svcErr.Code)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, true).Return(errTypeNotFound).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", changed)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorTypeNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestUpdateType_KeepsVersionWhenGrantIsUnchanged() {
	renamed := flightType()
	renamed.Name = renamedTypeName
	renamed.Reuse = false
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Twice()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(nil).Once()
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", renamed)
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestUpdateType_MovesVersionWhenProcessorIsAddedOrRemoved() {
	withProcessor := flightType()
	withProcessor.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Twice()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, true).Return(nil).Once()
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", withProcessor)
	s.Nil(svcErr)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(withProcessor, nil).Twice()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, true).Return(nil).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", flightType())
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestUpdateType_KeepsVersionWhenProcessorEndpointChanges() {
	stored := paymentType()
	moved := paymentType()
	moved.Processor = &providers.AuthorizationDetailProcessor{Endpoint: "https://processor.example.com/v2"}
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-2").Return(stored, nil).Twice()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(nil).Once()
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-2", moved)
	s.Nil(svcErr)
}

// An update that leaves the credential out, as an edited import document may, keeps the stored one at the
// same endpoint and is refused at a new one.
func (s *ServiceTestSuite) TestUpdateType_ResolvesTheProcessorCredential() {
	withTestCrypto(s.T())
	stored := flightType()
	stored.Processor = storedBearer(s.T())
	omitted := flightType()
	omitted.Name = renamedTypeName
	omitted.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}
	var written providers.AuthorizationDetailType
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Twice()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).
		Run(func(_ context.Context, t providers.AuthorizationDetailType, _ bool) { written = t }).Return(nil).Once()
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", omitted)
	s.Require().Nil(svcErr)
	s.Equal(testProcessorToken, storedToken(s.T(), written.Processor.AuthenticationProperties))

	moved := flightType()
	moved.Processor = &providers.AuthorizationDetailProcessor{Endpoint: "https://other.example.com/rar"}
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", moved)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProcessorAuthenticationRequired.Code, svcErr.Code)
}

// A type read back from the store carries the stored credential, which was not sent again, so it cannot follow a
// new endpoint; at the same endpoint it is kept.
func (s *ServiceTestSuite) TestUpdateType_StoredCredentialPassedBackIsNotResent() {
	withTestCrypto(s.T())
	stored := flightType()
	stored.Processor = storedBearer(s.T())

	moved := stored
	moved.Processor = &providers.AuthorizationDetailProcessor{Endpoint: "https://other.example.com/rar",
		AuthenticationProperties: stored.Processor.AuthenticationProperties}
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", moved)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProcessorAuthenticationRequired.Code, svcErr.Code)

	renamed := stored
	renamed.Name = renamedTypeName
	renamed.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL,
		AuthenticationProperties: stored.Processor.AuthenticationProperties}
	var written providers.AuthorizationDetailType
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Twice()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).
		Run(func(_ context.Context, t providers.AuthorizationDetailType, _ bool) { written = t }).Return(nil).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", renamed)
	s.Require().Nil(svcErr)
	s.Equal(testProcessorToken, storedToken(s.T(), written.Processor.AuthenticationProperties))
}

func (s *ServiceTestSuite) TestCreateAndUpdate_LeaveTheCallersProcessorAsSent() {
	withTestCrypto(s.T())
	sent, svcErr := sentAuthenticationProperties(bearer(testProcessorToken))
	s.Require().Nil(svcErr)
	created := paymentType()
	created.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL,
		AuthenticationProperties: sent}
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, created.Type).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, created.ID).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.Anything).Return(nil).Once()
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, created)
	s.Require().Nil(svcErr)
	s.Equal(sent, created.Processor.AuthenticationProperties)

	stored := flightType()
	stored.Processor = storedBearer(s.T())
	masked, svcErr := sentAuthenticationProperties(bearer("******"))
	s.Require().Nil(svcErr)
	updated := flightType()
	updated.Name = renamedTypeName
	updated.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL,
		AuthenticationProperties: masked}
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Twice()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(nil).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", updated)
	s.Require().Nil(svcErr)
	s.Equal(masked, updated.Processor.AuthenticationProperties)
}

func (s *ServiceTestSuite) TestUpdateType_SkipsSaveWhenUnchanged() {
	stored := flightType()
	same := flightType()
	var buf bytes.Buffer
	s.Require().NoError(json.Indent(&buf, stored.Schema, "", "    "))
	same.Schema = buf.Bytes()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	got, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", same)
	s.Nil(svcErr)
	s.Equal(stored.Version, got.Version)
}

func (s *ServiceTestSuite) TestUpdateType_StoresReorderedSchemaWithoutNewVersion() {
	stored := flightType()
	stored.Schema = json.RawMessage(`{"type":"object","properties":{"route":{"type":"string"},` +
		`"maxFare":{"type":"object","properties":{"amount":{"type":"number"},"currency":{"type":"string"}}}}}`)
	reordered := stored
	reordered.Schema = json.RawMessage(`{"type":"object","properties":{` +
		`"maxFare":{"type":"object","properties":{"currency":{"type":"string"},"amount":{"type":"number"}}},` +
		`"route":{"type":"string"}}}`)
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.MatchedBy(func(t providers.AuthorizationDetailType) bool {
		return string(t.Schema) == string(reordered.Schema)
	}), false).Return(nil).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(reordered, nil).Once()
	got, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", reordered)
	s.Nil(svcErr)
	s.Equal(stored.Version, got.Version)
	s.Equal(string(reordered.Schema), string(got.Schema))
}

// An equivalent schema and a whitespace-only consent template change keep the version.
func (s *ServiceTestSuite) TestUpdateType_KeepsVersionForAnEquivalentDefinition() {
	stored := flightType()
	stored.Schema = json.RawMessage(`{"type":"object","required":["type","route","maxFare"],"properties":{` +
		`"route":{"type":"string"},"maxFare":{"type":"number","policy":{"suppliedBy":"client",` +
		`"modification":"decrease","modifiableBy":["user","client"]}}}}`)
	stored.ConsentTemplate = ""
	rewritten := stored
	rewritten.Schema = json.RawMessage(`{"type":"object","required":["maxFare","route"],"properties":{` +
		`"route":{"type":"string"},"maxFare":{"type":"number","policy":{"modification":"decrease",` +
		`"modifiableBy":["client","user"]}}}}`)
	stored.ConsentTemplate = "Book a {{route}} flight up to {{maxFare}}"
	rewritten.ConsentTemplate = "  " + stored.ConsentTemplate + " "
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(nil).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(rewritten, nil).Once()

	got, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", rewritten)

	s.Nil(svcErr)
	s.Equal(stored.Version, got.Version)
}

func (s *ServiceTestSuite) TestCreateType_NamesTheFieldOfASchemaError() {
	t := flightType()
	t.Schema = json.RawMessage(`{"type":"object","properties":{"maxFare":{"type":"object","properties":` +
		`{"amount":{"type":"number","policy":{"modification":"sideways"}}}}}}`)
	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, t)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidSchema.Code, svcErr.Code)
	s.Equal("maxFare.amount", svcErr.ErrorDescription.Params["path"])

	t.Schema = json.RawMessage(`{"type":"object","properties":{"type":{"type":"number"}}}`)
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, t)
	s.Require().NotNil(svcErr)
	s.Equal("type", svcErr.ErrorDescription.Params["path"])
}

func (s *ServiceTestSuite) TestUpdateType_Errors() {
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil)

	changed := flightType()
	changed.Type = "https://other"
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", changed)
	s.Equal(ErrorTypeValueImmutable.Code, svcErr.Code)

	invalid := flightType()
	invalid.Name = ""
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", invalid)
	s.Equal(ErrorInvalidName.Code, svcErr.Code)

	renamed := flightType()
	renamed.Name = renamedTypeName
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(errTypeNotFound).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", renamed)
	s.Equal(ErrorTypeChanged.Code, svcErr.Code)

	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(errors.New("db down")).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", renamed)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "missing").
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "missing", flightType())
	s.Equal(ErrorTypeNotFound.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestDeleteType() {
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil)
	s.typeStore.EXPECT().DeleteType(mock.Anything, "type-1").Return(nil).Once()
	s.Nil(s.service.DeleteAuthorizationDetailType(s.ctx, "type-1"))

	s.typeStore.EXPECT().DeleteType(mock.Anything, "type-1").Return(errors.New("db down")).Once()
	s.Equal(tidcommon.InternalServerError.Code, s.service.DeleteAuthorizationDetailType(s.ctx, "type-1").Code)

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "missing").
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	s.Nil(s.service.DeleteAuthorizationDetailType(s.ctx, "missing"), "deleting a missing type succeeds")
}

func (s *ServiceTestSuite) TestUpdateAndDeleteType_RefuseDeclarativeType() {
	declarative := flightType()
	declarative.IsReadOnly = true
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(declarative, nil)

	renamed := flightType()
	renamed.Name = renamedTypeName
	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", renamed)
	s.Equal(ErrorImmutableType.Code, svcErr.Code)
	s.Equal(ErrorImmutableType.Code, s.service.DeleteAuthorizationDetailType(s.ctx, "type-1").Code)
}

func (s *ServiceTestSuite) TestListAllTypes() {
	s.typeStore.EXPECT().ListTypes(mock.Anything).
		Return([]providers.AuthorizationDetailType{flightType(), paymentType()}, nil).Once()
	types, svcErr := s.service.ListAllAuthorizationDetailTypes(s.ctx)
	s.Nil(svcErr)
	s.Len(types, 2)

	s.typeStore.EXPECT().ListTypes(mock.Anything).Return(nil, errors.New("db down")).Once()
	_, svcErr = s.service.ListAllAuthorizationDetailTypes(s.ctx)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestIsTypeDeclarative() {
	s.typeStore.EXPECT().IsTypeDeclarative(mock.Anything, "type-1").Return(true, nil).Once()
	isDeclarative, svcErr := s.service.IsAuthorizationDetailTypeDeclarative(s.ctx, "type-1")
	s.Nil(svcErr)
	s.True(isDeclarative)

	s.typeStore.EXPECT().IsTypeDeclarative(mock.Anything, "err").Return(false, errors.New("store down")).Once()
	_, svcErr = s.service.IsAuthorizationDetailTypeDeclarative(s.ctx, "err")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestGetAuthorizationDetailTypeByValue() {
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).Return(flightType(), nil).Once()
	t, svcErr := s.service.GetAuthorizationDetailTypeByValue(s.ctx, testFlightType)
	s.Require().Nil(svcErr)
	s.Equal(flightType(), *t)

	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, "unknown").
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	t, svcErr = s.service.GetAuthorizationDetailTypeByValue(s.ctx, "unknown")
	s.Nil(svcErr, "a missing type is not an error")
	s.Nil(t)

	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, "broken").
		Return(providers.AuthorizationDetailType{}, errors.New("store down")).Once()
	_, svcErr = s.service.GetAuthorizationDetailTypeByValue(s.ctx, "broken")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

const renamedTypeName = "Renamed"

// The name limit counts characters, not bytes.
func (s *ServiceTestSuite) TestValidateTypeDefinition_NameLengthCountsCharacters() {
	typ := flightType()
	typ.Name = strings.Repeat("க", maxNameLength)
	s.Nil(validateTypeDefinition(typ))
}

func (s *ServiceTestSuite) TestIsValidProcessorConfig() {
	valid := []*providers.AuthorizationDetailProcessor{
		nil,
		{Endpoint: "https://bank.example.com/p"},
		{Endpoint: "http://localhost:9000/p"},
		{Endpoint: "http://127.0.0.1/p"},
	}
	for _, cfg := range valid {
		s.True(isValidProcessorConfig(cfg))
	}

	invalid := map[string]*providers.AuthorizationDetailProcessor{
		"relative":       {Endpoint: "/p"},
		"plain http":     {Endpoint: "http://bank.example.com/p"},
		"other scheme":   {Endpoint: "ftp://bank.example.com/p"},
		"unparsable url": {Endpoint: "://"},
		"credentials":    {Endpoint: "http://u:p@127.0.0.1:9099/process"},
		"port too large": {Endpoint: "https://bank.example.com:99999/p"},
		"port zero":      {Endpoint: "https://bank.example.com:0/p"},
	}
	for name, cfg := range invalid {
		s.False(isValidProcessorConfig(cfg), name)
	}
}

const testProcessorToken = "processor-token"

// withTestCrypto installs a hex-encoding crypto provider so stored credentials differ from plaintext.
func withTestCrypto(t *testing.T) {
	provider := cryptomock.NewConfigCryptoProviderMock(t)
	provider.EXPECT().Encrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, content []byte) ([]byte, error) {
			return []byte(hex.EncodeToString(content)), nil
		}).Maybe()
	provider.EXPECT().Decrypt(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, content []byte) ([]byte, error) {
			return hex.DecodeString(string(content))
		}).Maybe()
	cmodels.SetConfigCryptoProvider(provider)
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })
}

func bearer(token string) *outboundauth.Authentication {
	return &outboundauth.Authentication{Type: string(outboundauth.TypeBearer),
		Properties: map[string]string{outboundauth.FieldBearerToken: token}}
}

// storedBearer is a processor at the test endpoint whose stored authentication is the test bearer token.
func storedBearer(t *testing.T) *providers.AuthorizationDetailProcessor {
	endpoint, token := testProcessorURL, testProcessorToken
	properties, svcErr := processorAuthenticationProperties(bearer(token),
		&providers.AuthorizationDetailProcessor{Endpoint: endpoint}, nil)
	require.Nil(t, svcErr)
	return &providers.AuthorizationDetailProcessor{Endpoint: endpoint, AuthenticationProperties: properties}
}

func storedToken(t *testing.T, properties []cmodels.Property) string {
	config, err := outboundauth.FromProperties(properties)
	require.NoError(t, err)
	return config.Get(outboundauth.FieldBearerToken)
}

func (s *ServiceTestSuite) TestProcessorAuthentication_IsStoredEncryptedAndReturnedMasked() {
	withTestCrypto(s.T())
	processor := storedBearer(s.T())

	encoded, err := cmodels.SerializePropertiesToJSONArray(processor.AuthenticationProperties)
	s.Require().NoError(err)
	s.NotContains(encoded, testProcessorToken, "the credential is not stored in clear")
	s.Equal(testProcessorToken, storedToken(s.T(), processor.AuthenticationProperties))

	response := processorResponse(processor)
	s.Equal(testProcessorURL, response.Endpoint)
	s.Equal(string(outboundauth.TypeBearer), response.Authentication.Type)
	s.Equal("******", response.Authentication.Properties[outboundauth.FieldBearerToken])
}

func (s *ServiceTestSuite) TestProcessorAuthentication_DefaultsToNone() {
	withTestCrypto(s.T())
	properties, svcErr := processorAuthenticationProperties(nil,
		&providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}, nil)
	s.Require().Nil(svcErr)
	config, err := outboundauth.FromProperties(properties)
	s.Require().NoError(err)
	s.False(config.Enabled())
	s.Nil(processorResponse(nil))
}

func (s *ServiceTestSuite) TestProcessorAuthentication_UpdateKeepsMaskedCredentialAtTheSameEndpoint() {
	withTestCrypto(s.T())
	existing := storedBearer(s.T())

	for name, request := range map[string]*outboundauth.Authentication{
		"left out":         nil,
		"sent back masked": bearer("******"),
	} {
		properties, svcErr := processorAuthenticationProperties(request,
			&providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}, existing)
		s.Require().Nil(svcErr, name)
		s.Equal(testProcessorToken, storedToken(s.T(), properties), name)
	}
}

func (s *ServiceTestSuite) TestProcessorAuthentication_NewEndpointNeedsTheCredentialAgain() {
	withTestCrypto(s.T())
	existing := storedBearer(s.T())
	moved := &providers.AuthorizationDetailProcessor{Endpoint: "https://elsewhere.example.com/process"}

	_, svcErr := processorAuthenticationProperties(nil, moved, existing)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProcessorAuthenticationRequired.Code, svcErr.Code)

	_, svcErr = processorAuthenticationProperties(bearer("******"), moved, existing)
	s.Require().NotNil(svcErr, "a masked credential is not carried to another endpoint")

	properties, svcErr := processorAuthenticationProperties(bearer("new-token"), moved, existing)
	s.Require().Nil(svcErr)
	s.Equal("new-token", storedToken(s.T(), properties))
}

func (s *ServiceTestSuite) TestProcessorAuthentication_Refused() {
	withTestCrypto(s.T())
	processor := &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL}
	cases := map[string]struct {
		request   *outboundauth.Authentication
		processor *providers.AuthorizationDetailProcessor
	}{
		"unsupported method": {&outboundauth.Authentication{Type: "kerberos"}, processor},
		"missing credential": {
			&outboundauth.Authentication{Type: string(outboundauth.TypeBearer)}, processor},
	}
	for name, tc := range cases {
		_, svcErr := processorAuthenticationProperties(tc.request, tc.processor, nil)
		s.Require().NotNil(svcErr, name)
		s.Equal(ErrorInvalidProcessorAuthentication.Code, svcErr.Code, name)
	}
}

func (s *ServiceTestSuite) TestProcessorAuthentication_RoundTripsThroughTheStoredProperties() {
	withTestCrypto(s.T())
	original := flightType()
	original.Processor = storedBearer(s.T())

	encoded, err := buildTypeProperties(original)
	s.Require().NoError(err)
	s.NotContains(string(encoded), testProcessorToken)

	restored, err := buildTypeFromRow(map[string]interface{}{"id": "type-1", "type": original.Type,
		"name": original.Name, "version": int64(1), "schema_definition": string(original.Schema),
		"properties": string(encoded)})
	s.Require().NoError(err)
	s.Require().NotNil(restored.Processor)
	s.Equal(testProcessorURL, restored.Processor.Endpoint)
	s.Equal(testProcessorToken, storedToken(s.T(), restored.Processor.AuthenticationProperties))
}

func (s *ServiceTestSuite) TestSetProcessorAuthentication_KeepsTheStoredCredentialAtTheSameEndpoint() {
	withTestCrypto(s.T())
	existing := flightType()
	existing.Processor = storedBearer(s.T())
	masked := &AuthorizationDetailTypeRequest{Type: existing.Type, Name: existing.Name, Schema: existing.Schema,
		Processor: &ProcessorRequest{Endpoint: testProcessorURL, Authentication: bearer("******")}}
	omitted := &AuthorizationDetailTypeRequest{Type: existing.Type, Name: existing.Name, Schema: existing.Schema,
		Processor: &ProcessorRequest{Endpoint: testProcessorURL}}

	for name, req := range map[string]*AuthorizationDetailTypeRequest{"masked": masked, "omitted": omitted} {
		updated, svcErr := toType(req)
		s.Require().Nil(svcErr, name)
		s.Require().Nil(setProcessorAuthentication(&updated, existing.Processor), name)
		s.Equal(testProcessorToken, storedToken(s.T(), updated.Processor.AuthenticationProperties), name)
	}

	created, svcErr := toType(masked)
	s.Require().Nil(svcErr, "the credential is carried as sent")
	svcErr = setProcessorAuthentication(&created, nil)
	s.Require().NotNil(svcErr, "a new type has no stored credential for a masked one to keep")
	s.Equal(ErrorInvalidProcessorAuthentication.Code, svcErr.Code)

	moved, svcErr := toType(&AuthorizationDetailTypeRequest{Type: existing.Type, Name: existing.Name,
		Schema: existing.Schema, Processor: &ProcessorRequest{Endpoint: "https://other.example.com/rar"}})
	s.Require().Nil(svcErr)
	svcErr = setProcessorAuthentication(&moved, existing.Processor)
	s.Require().NotNil(svcErr, "a new endpoint needs the credential again")
	s.Equal(ErrorProcessorAuthenticationRequired.Code, svcErr.Code)

	_, svcErr = toType(&AuthorizationDetailTypeRequest{Type: existing.Type, Name: existing.Name,
		Schema: existing.Schema, Processor: &ProcessorRequest{Endpoint: testProcessorURL,
			Authentication: &outboundauth.Authentication{Type: "unknown"}}})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidProcessorAuthentication.Code, svcErr.Code)
}

// An update that sends authentication none clears the stored credential, even at the same endpoint.
func (s *ServiceTestSuite) TestSetProcessorAuthentication_SentNoneClearsTheStoredCredential() {
	withTestCrypto(s.T())
	existing := flightType()
	existing.Processor = storedBearer(s.T())
	updated, svcErr := toType(&AuthorizationDetailTypeRequest{Type: existing.Type, Name: existing.Name,
		Schema: existing.Schema, Processor: &ProcessorRequest{Endpoint: testProcessorURL,
			Authentication: &outboundauth.Authentication{Type: string(outboundauth.TypeNone)}}})
	s.Require().Nil(svcErr)

	s.Require().Nil(setProcessorAuthentication(&updated, existing.Processor))

	config, err := outboundauth.FromProperties(updated.Processor.AuthenticationProperties)
	s.Require().NoError(err)
	s.False(config.Enabled())
	s.Empty(storedToken(s.T(), updated.Processor.AuthenticationProperties))
	s.Equal(string(outboundauth.TypeNone), processorResponse(updated.Processor).Authentication.Type)
}
