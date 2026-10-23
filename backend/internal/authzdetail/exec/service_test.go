// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailexec

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authzdetail/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authzdetailtypeprovidermock"
)

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
	testRSIdentifier = "https://api.example.com/bookings"
	testFlightType   = "https://scheme.example.com/flight/v1"
	testPaymentType  = "https://scheme.example.com/payment/v1"
	testProcessorURL = "https://bank.example.com/rar/v1/process"
	// testRecreatedTypeID is the id of a flight type deleted and created again under the same value.
	testRecreatedTypeID = "type-1-recreated"
	testClientID        = "client-1"
	testUserID          = "user-1"
	testAppID           = "app-1"
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
	types     *authzdetailtypeprovidermock.AuthorizationDetailTypeProviderMock
	processor *processorClientInterfaceMock
	service   *authorizationDetailExecService
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
	s.types = authzdetailtypeprovidermock.NewAuthorizationDetailTypeProviderMock(s.T())
	s.processor = newProcessorClientInterfaceMock(s.T())
	s.service = newAuthorizationDetailExecService(s.types, s.processor).(*authorizationDetailExecService)
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

func (s *ServiceTestSuite) expectTypes(types ...providers.AuthorizationDetailType) {
	for _, t := range types {
		s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, t.Type).Return(typeRef(t), nil).Maybe()
	}
}

func (s *ServiceTestSuite) validated(raw string) json.RawMessage {
	details, svcErr := s.service.ValidateAuthorizationDetails(s.ctx, raw)
	s.Require().Nil(svcErr)
	return details
}

const flightRequest = `[{"type":"` + testFlightType + `","actions":["create"],` +
	`"locations":["` + testRSIdentifier + `/v1"],"route":"CMB-SIN","maxFare":{"amount":450.00,"currency":"USD"}}]`

func (s *ServiceTestSuite) TestValidate_Success() {
	s.expectTypes(flightType())

	details, svcErr := s.service.ValidateAuthorizationDetails(s.ctx, flightRequest)
	s.Require().Nil(svcErr)

	s.NotContains(string(details), `"id"`, "detail identifiers never leave the server")
	objects, err := common.ParseRequestedDetails(string(details))
	s.Require().NoError(err)
	s.Require().Len(objects, 1)
	s.Equal(`450.00`, string(mustLookup(s.T(), objects[0].Fields, "maxFare.amount")))
}

func (s *ServiceTestSuite) TestValidate_Rejects() {
	s.expectTypes(flightType())
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, "unknown").
		Return(nil, nil).Maybe()

	cases := []struct {
		name string
		raw  string
		code string
	}{
		{"malformed", `{"type":"x"}`, ErrorMalformedAuthorizationDetails.Code},
		{"unknown type", `[{"type":"unknown"}]`, ErrorUnknownAuthorizationDetailType.Code},
		{"schema violation", `[{"type":"` + testFlightType + `","route":"CMB-SIN"}]`,
			ErrorAuthorizationDetailNotConformant.Code},
		{"unknown field", `[{"type":"` + testFlightType + `","route":"CMB-SIN","maxFare":{},"x":1}]`,
			ErrorAuthorizationDetailNotConformant.Code},
		{"undeclared common field", `[{"type":"` + testFlightType + `","route":"CMB-SIN","maxFare":{},` +
			`"privileges":["admin"]}]`, ErrorAuthorizationDetailNotConformant.Code},
	}
	for _, tc := range cases {
		_, svcErr := s.service.ValidateAuthorizationDetails(s.ctx, tc.raw)
		s.Require().NotNil(svcErr, tc.name)
		s.Equal(tidcommon.ClientErrorType, svcErr.Type, tc.name)
		s.Equal(tc.code, svcErr.Code, tc.name)
	}
}

// Types are not bound to a resource server, so details of any registered types share a request.
func (s *ServiceTestSuite) TestValidate_AcceptsDetailsOfSeveralTypes() {
	s.expectTypes(flightType(), paymentType())
	raw := `[` + strings.Trim(flightRequest, "[]") + `,{"type":"` + testPaymentType + `","amount":1}]`

	details, svcErr := s.service.ValidateAuthorizationDetails(s.ctx, raw)
	s.Require().Nil(svcErr)
	objects, err := common.ParseRequestedDetails(string(details))
	s.Require().NoError(err)
	s.Len(objects, 2)
}

func (s *ServiceTestSuite) TestValidate_ServerErrors() {
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr := s.service.ValidateAuthorizationDetails(s.ctx, flightRequest)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	corrupt := flightType()
	corrupt.Schema = json.RawMessage(`not json`)
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).Return(typeRef(corrupt),
		nil).Once()
	_, svcErr = s.service.ValidateAuthorizationDetails(s.ctx, flightRequest)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

const paymentRequest = `[{"type":"` + testPaymentType + `","amount":45000.00}]`

// clientSubject is the subject of a client_credentials request: the client application itself.
var clientSubject = providers.EntityReference{EntityID: testAppID, EntityCategory: string(providers.EntityCategoryApp)}

// On client_credentials the client is the subject, and the processor decides on its own.
func (s *ServiceTestSuite) TestProcess_ClientAsSubject() {
	details := s.prepared(paymentRequest, paymentType())

	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.MatchedBy(func(req processorRequest) bool {
		return req.Subject == processorEntity{Type: oauth2const.SubTypeApp, ID: testAppID} &&
			req.Client == processorEntity{Type: oauth2const.SubTypeApp, ID: testClientID} &&
			req.Resource == testResourceServer
	})).RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
		req processorRequest) (*processorResponse, error) {
		return &processorResponse{Results: []processorResult{{DetailID: req.Details[0].DetailID,
			Decision: decision(true),
			Patch:    map[string]json.RawMessage{"/debtorAccount/iban": json.RawMessage(`"DE12"`)}}}}, nil
	}).Once()
	admitted, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, clientSubject,
		testResourceServer)
	s.Require().Nil(svcErr)
	s.Require().Len(admitted, 1)
	s.Contains(string(admitted[0].Value), "DE12")

	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
			req processorRequest) (*processorResponse, error) {
			return &processorResponse{Results: []processorResult{refusal(req.Details[0].DetailID, "over limit")}}, nil
		}).Once()
	_, svcErr = s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, clientSubject, testResourceServer)
	s.Equal(ErrorAuthorizationDetailRefused.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.String(), "over limit")
}

// An agent requesting for itself is reported to the processor as an agent.
func (s *ServiceTestSuite) TestProcess_AgentAsSubject() {
	details := s.prepared(paymentRequest, paymentType())
	agent := &providers.Application{ID: "agent-1", EntityCategory: providers.EntityCategoryAgent,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "agent-client"}}}}
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.MatchedBy(func(req processorRequest) bool {
		return req.Subject == processorEntity{Type: oauth2const.SubTypeAgent, ID: "agent-1"} &&
			req.Client == processorEntity{Type: oauth2const.SubTypeAgent, ID: "agent-client"}
	})).RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
		req processorRequest) (*processorResponse, error) {
		return &processorResponse{Results: []processorResult{{DetailID: req.Details[0].DetailID,
			Decision: decision(true)}}}, nil
	}).Once()

	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, agent,
		providers.EntityReference{EntityID: "agent-1", EntityCategory: string(providers.EntityCategoryAgent)},
		testResourceServer)
	s.Nil(svcErr)
}

// With the client as the subject no one can consent, so a type without a processor is refused.
func (s *ServiceTestSuite) TestProcess_ClientAsSubjectRequiresAProcessor() {
	details := s.prepared(flightRequest, flightType())
	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, clientSubject, testResourceServer)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAuthorizationDetailNeedsUser.Code, svcErr.Code)
	s.Equal("authorization details of type "+testFlightType+" cannot be requested without a user",
		svcErr.ErrorDescription.String())
}

func (s *ServiceTestSuite) TestRefreshableAuthorizationDetails() {
	oneOff := paymentType()
	oneOff.Reuse = false
	granted := s.grantedOn(`[{"type":"`+testFlightType+`","route":"CMB-LHR"},`+
		`{"type":"`+testPaymentType+`","amount":10.00}]`, flightType(), paymentType())

	s.expectTypes(flightType(), oneOff)
	refreshable, svcErr := s.service.RefreshableAuthorizationDetails(s.ctx, granted)
	s.Require().Nil(svcErr)
	s.Equal(`[{"route":"CMB-LHR","type":"`+testFlightType+`"}]`, s.values(refreshable))
	s.Require().Len(refreshable, 1)
	s.Equal(flightType().ID, refreshable[0].TypeID, "a renewed detail keeps the type it was granted on")
	s.Equal(flightType().Version, refreshable[0].TypeVersion)

	onlyOneOff := s.grantedOn(`[{"type":"`+testPaymentType+`","amount":10.00}]`, paymentType())
	refreshable, svcErr = s.service.RefreshableAuthorizationDetails(s.ctx, onlyOneOff)
	s.Nil(svcErr)
	s.Nil(refreshable)

	refreshable, svcErr = s.service.RefreshableAuthorizationDetails(s.ctx, nil)
	s.Nil(svcErr)
	s.Nil(refreshable)

	_, svcErr = s.service.RefreshableAuthorizationDetails(s.ctx,
		[]providers.AuthorizationDetail{{Type: testFlightType, Value: json.RawMessage(`bad`)}})
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	_, svcErr = s.service.RefreshableAuthorizationDetails(s.ctx,
		[]providers.AuthorizationDetail{{Type: testFlightType, Value: json.RawMessage(`{"route":"CMB-LHR"}`)}})
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code, "a granted value without a type")
}

func flightDetail(amount, extraFare string) string {
	return `{"type":"` + testFlightType + `","route":"CMB-SIN","maxFare":{"amount":` + amount +
		`,"currency":"USD"` + extraFare + `}}`
}

func details(entries ...string) string {
	return "[" + strings.Join(entries, ",") + "]"
}

func (s *ServiceTestSuite) TestModifyAuthorizationDetails() {
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).Return(typeRef(flightType()),
		nil).Maybe()
	grantedRaw := details(flightDetail("450.00", ""), flightDetail("300.00", ""))
	granted := s.grantedOn(grantedRaw, flightType())

	modified, svcErr := s.service.ModifyAuthorizationDetails(context.Background(), granted,
		details(flightDetail("200.00", "")))
	s.Require().Nil(svcErr)
	s.JSONEq(details(flightDetail("200.00", "")), string(modified), "details no entry modifies are left out")

	for name, raw := range map[string]string{
		"each entry finds its own grant":    details(flightDetail("400.00", ""), flightDetail("250.00", "")),
		"pairing moves an earlier entry":    details(flightDetail("250.00", ""), flightDetail("400.00", "")),
		"the same values as both grants":    details(flightDetail("300.00", ""), flightDetail("450.00", "")),
		"a request equal to the full grant": grantedRaw,
	} {
		_, svcErr := s.service.ModifyAuthorizationDetails(context.Background(), granted, raw)
		s.Nil(svcErr, name)
	}

	for name, raw := range map[string]string{
		"widens":               details(flightDetail("500.00", "")),
		"claims a grant twice": details(flightDetail("400.00", ""), flightDetail("350.00", "")),
		"more entries":         details(flightDetail("1.00", ""), flightDetail("1.00", ""), flightDetail("1.00", "")),
		"other type":           `[{"type":"` + testPaymentType + `"}]`,
		"breaks schema":        details(flightDetail("1.00", `,"tip":"1"`)),
	} {
		_, svcErr := s.service.ModifyAuthorizationDetails(context.Background(), granted, raw)
		s.Require().NotNil(svcErr, name)
		s.Equal(tidcommon.ClientErrorType, svcErr.Type, name)
	}

	_, svcErr = s.service.ModifyAuthorizationDetails(context.Background(), nil, `[]`)
	s.Require().NotNil(svcErr, "nothing was granted")
	s.Equal(ErrorNoAuthorizationDetailsGranted.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestModifyAuthorizationDetails_RefusedWhenTheTypeHasChanged() {
	granted := s.grantedOn(details(flightDetail("450.00", "")), flightType())
	raw := details(flightDetail("200.00", ""))

	updated := flightType()
	updated.Version++
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).Return(typeRef(updated),
		nil).Once()
	_, svcErr := s.service.ModifyAuthorizationDetails(context.Background(), granted, raw)
	s.Require().NotNil(svcErr, "a newer version")
	s.Equal(ErrorAuthorizationDetailTypeChangedSinceGrant.Code, svcErr.Code)

	recreated := flightType()
	recreated.ID = testRecreatedTypeID
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).Return(typeRef(recreated),
		nil).Once()
	_, svcErr = s.service.ModifyAuthorizationDetails(context.Background(), granted, raw)
	s.Require().NotNil(svcErr, "a type deleted and created again on the same version number")
	s.Equal(ErrorAuthorizationDetailTypeChangedSinceGrant.Code, svcErr.Code)

	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(nil, nil).Once()
	_, svcErr = s.service.ModifyAuthorizationDetails(context.Background(), granted, raw)
	s.Require().NotNil(svcErr, "a deleted type")
	s.Equal(ErrorAuthorizationDetailTypeChangedSinceGrant.Code, svcErr.Code)

	_, svcErr = s.service.ModifyAuthorizationDetails(context.Background(), granted,
		`[{"type":"`+testPaymentType+`","amount":1.00}]`)
	s.Require().NotNil(svcErr, "a type the grant holds no detail of")
	s.Equal(ErrorAuthorizationDetailNotGranted.Code, svcErr.Code)

	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr = s.service.ModifyAuthorizationDetails(context.Background(), granted, raw)
	s.Require().NotNil(svcErr, "a failed lookup")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestMatchDetails() {
	s.Equal([]int{1, 0}, matchDetails([][]bool{{true, true}, {true, false}}))
	s.Equal([]int{0, -1}, matchDetails([][]bool{{true}, {true}}))
	s.Equal([]int{}, matchDetails([][]bool{}))
}

func (s *ServiceTestSuite) TestValidate_RejectsFieldsTheResourceServerSupplies() {
	s.expectTypes(paymentType())
	_, svcErr := s.service.ValidateAuthorizationDetails(s.ctx,
		`[{"type":"`+testPaymentType+`","amount":1.00,"debtorAccount":{"iban":"DE12"}}]`)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAuthorizationDetailNotAllowed.Code, svcErr.Code)
}

func decision(approved bool) *bool {
	return &approved
}

func refusal(detailID, reason string) processorResult {
	return processorResult{DetailID: detailID, Decision: decision(false),
		Context: processorResultContext{ReasonUser: reason}}
}

// grantedOn returns the details of a raw array as granted on the given types; a detail whose type is not
// given is granted on no type.
func (s *ServiceTestSuite) grantedOn(raw string,
	types ...providers.AuthorizationDetailType) []providers.AuthorizationDetail {
	var values []json.RawMessage
	s.Require().NoError(json.Unmarshal([]byte(raw), &values))
	granted := make([]providers.AuthorizationDetail, 0, len(values))
	for _, value := range values {
		var typed struct {
			Type string `json:"type"`
		}
		s.Require().NoError(json.Unmarshal(value, &typed))
		detail := providers.AuthorizationDetail{Type: typed.Type, Value: value, Admitted: true}
		for _, t := range types {
			if t.Type == typed.Type {
				detail.TypeID, detail.TypeVersion = t.ID, t.Version
			}
		}
		granted = append(granted, detail)
	}
	return granted
}

// values returns the granted values of details as tokens carry them.
func (s *ServiceTestSuite) values(details []providers.AuthorizationDetail) string {
	encoded, err := common.EncodeDetailValues(details)
	s.Require().NoError(err)
	return string(encoded)
}

const testSecondProcessorURL = "https://airline.example.com/rar/v1/process"

// twoEndpointDetails returns a payment and a flight detail whose types use different processors.
func (s *ServiceTestSuite) twoEndpointDetails() ([]common.Detail, map[string]providers.AuthorizationDetailType) {
	flight := flightType()
	flight.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testSecondProcessorURL}
	types := map[string]providers.AuthorizationDetailType{testPaymentType: paymentType(), testFlightType: flight}
	return []common.Detail{
		{ID: "d-pay", Type: testPaymentType, Fields: map[string]json.RawMessage{"type": json.RawMessage(`"` +
			testPaymentType + `"`)}},
		{ID: "d-flight", Type: testFlightType, Fields: map[string]json.RawMessage{
			"type":    json.RawMessage(`"` + testFlightType + `"`),
			"route":   json.RawMessage(`"CMB-LHR"`),
			"maxFare": json.RawMessage(`{"amount":500,"currency":"USD"}`),
		}},
	}, types
}

func (s *ServiceTestSuite) TestProcessDetails_ApprovalWithoutPatchMustFillRequiredFields() {
	payment := paymentType()
	payment.Schema = json.RawMessage(`{"type":"object","required":["amount","debtorAccount"],"properties":{` +
		`"amount":{"type":"number"},` +
		`"debtorAccount":{"type":"string","policy":{"suppliedBy":"processor"}}}}`)
	types := map[string]providers.AuthorizationDetailType{testPaymentType: payment}
	objects := []common.Detail{{ID: "d-pay", Type: testPaymentType, Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"` + testPaymentType + `"`), "amount": json.RawMessage(`10`)}}}
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.Anything).
		Return(&processorResponse{Results: []processorResult{{DetailID: "d-pay", Decision: decision(true)}}},
			nil).Once()

	_, svcErr := s.service.processDetails(s.ctx, objects, types, processorEntity{}, processorEntity{}, "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProcessorUnavailable.Code, svcErr.Code)
}

// A result for a detail sent to another endpoint makes the answer unusable.
func (s *ServiceTestSuite) TestProcessDetails_ResultForADetailNotSent() {
	types := map[string]providers.AuthorizationDetailType{testPaymentType: paymentType()}
	objects := []common.Detail{{ID: "d-pay", Type: testPaymentType, Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"` + testPaymentType + `"`), "amount": json.RawMessage(`10`)}}}
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.Anything).
		Return(&processorResponse{Results: []processorResult{{DetailID: "d-pay", Decision: decision(true)},
			{DetailID: "d-other", Decision: decision(true)}}}, nil).Once()

	_, svcErr := s.service.processDetails(s.ctx, objects, types, processorEntity{}, processorEntity{}, "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProcessorUnavailable.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestProcessDetails_FailedCallStopsTheRequest() {
	objects, types := s.twoEndpointDetails()
	s.processor.EXPECT().Process(mock.Anything, providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL},
		mock.Anything).
		Return(nil, errProcessorUnavailable).Once()

	_, svcErr := s.service.processDetails(s.ctx, objects, types, processorEntity{}, processorEntity{}, "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProcessorUnavailable.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestProcessDetails_RefusalOnOneEndpointKeepsTheOther() {
	objects, types := s.twoEndpointDetails()
	s.processor.EXPECT().Process(mock.Anything, providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL},
		mock.Anything).
		Return(&processorResponse{Results: []processorResult{refusal("d-pay", "over limit")}}, nil).Once()
	s.processor.EXPECT().Process(mock.Anything,
		providers.AuthorizationDetailProcessor{Endpoint: testSecondProcessorURL}, mock.Anything).
		Return(&processorResponse{Results: []processorResult{{DetailID: "d-flight", Decision: decision(true)}}},
			nil).Once()

	kept, svcErr := s.service.processDetails(s.ctx, objects, types, processorEntity{}, processorEntity{}, "")
	s.Require().Nil(svcErr)
	s.Require().Len(kept, 1)
	s.Equal("d-flight", kept[0].ID)
}

func (s *ServiceTestSuite) TestRefreshableAuthorizationDetails_DeletedTypeIsNotRenewed() {
	granted := s.grantedOn(`[{"type":"`+testFlightType+`","route":"CMB-LHR"},`+
		`{"type":"`+testPaymentType+`","amount":10.00}]`, flightType(), paymentType())
	s.expectTypes(flightType())
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testPaymentType).
		Return(nil, nil)

	refreshable, svcErr := s.service.RefreshableAuthorizationDetails(s.ctx, granted)
	s.Require().Nil(svcErr)
	s.Contains(s.values(refreshable), "CMB-LHR")
	s.NotContains(s.values(refreshable), testPaymentType)
}

// A recreated type, or one the grant has no version for, is not renewed even when it allows reuse.
func (s *ServiceTestSuite) TestRefreshableAuthorizationDetails_RecreatedTypeIsNotRenewed() {
	raw := `[{"type":"` + testFlightType + `","route":"CMB-LHR"}]`
	recreated := flightType()
	recreated.ID = "type-1-recreated"
	s.expectTypes(recreated)

	refreshable, svcErr := s.service.RefreshableAuthorizationDetails(s.ctx, s.grantedOn(raw, flightType()))
	s.Require().Nil(svcErr)
	s.Nil(refreshable)

	refreshable, svcErr = s.service.RefreshableAuthorizationDetails(s.ctx, s.grantedOn(raw))
	s.Require().Nil(svcErr)
	s.Nil(refreshable, "a detail the grant names no type for is not renewed")
}

// A type whose version moved since the grant is not renewed, so a tightened limit or a changed consent text
// takes effect at the next refresh.
func (s *ServiceTestSuite) TestRefreshableAuthorizationDetails_ChangedTypeIsNotRenewed() {
	granted := s.grantedOn(`[{"type":"`+testFlightType+`","route":"CMB-LHR"},`+
		`{"type":"`+testPaymentType+`","amount":10.00}]`, flightType(), paymentType())
	changed := paymentType()
	changed.Version++
	s.expectTypes(flightType(), changed)

	refreshable, svcErr := s.service.RefreshableAuthorizationDetails(s.ctx, granted)
	s.Require().Nil(svcErr)
	s.Contains(s.values(refreshable), "CMB-LHR")
	s.NotContains(s.values(refreshable), testPaymentType)
}

func typeRef(t providers.AuthorizationDetailType) *providers.AuthorizationDetailType {
	return &t
}

const testResourceServer = "https://bank.example.com"

var testSubject = providers.EntityReference{EntityID: testUserID, EntityCategory: "user"}

var testApp = &providers.Application{ID: testAppID, EntityCategory: providers.EntityCategoryApp,
	InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{Type: providers.OAuthInboundAuthType,
		OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: testClientID}}}}

func (s *ServiceTestSuite) prepared(raw string, t providers.AuthorizationDetailType) []providers.AuthorizationDetail {
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, t.Type).Return(typeRef(t), nil).Maybe()
	details, svcErr := s.service.PrepareAuthorizationDetails(s.ctx, s.validated(raw))
	s.Require().Nil(svcErr)
	return details
}

func approveAll(_ context.Context, _ providers.AuthorizationDetailProcessor,
	req processorRequest) (*processorResponse, error) {
	results := make([]processorResult, 0, len(req.Details))
	for _, d := range req.Details {
		results = append(results, processorResult{DetailID: d.DetailID, Decision: decision(true)})
	}
	return &processorResponse{Results: results}, nil
}

func (s *ServiceTestSuite) TestPrepare_CarriesIdentifiersAndTheTypesTheRequestStartsWith() {
	details := s.prepared(paymentRequest, paymentType())
	s.Require().Len(details, 1)
	d := details[0]
	s.NotEmpty(d.ID)
	s.Equal(testPaymentType, d.Type)
	s.Equal(paymentType().ID, d.TypeID)
	s.Equal(paymentType().Version, d.TypeVersion)
	s.JSONEq(string(d.Requested), string(d.Value))
	s.False(d.Admitted, "a type with a processor is admitted only once the processor ran")

	flight := s.prepared(flightRequest, flightType())
	s.True(flight[0].Admitted, "a type without a processor needs no processing")
}

func (s *ServiceTestSuite) TestPrepare_Failures() {
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(typeRef(flightType()), nil).Once()
	validated := s.validated(flightRequest)

	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr := s.service.PrepareAuthorizationDetails(s.ctx, validated)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).Return(nil, nil).Once()
	_, svcErr = s.service.PrepareAuthorizationDetails(s.ctx, validated)
	s.Require().NotNil(svcErr, "a type deleted since validation")
	s.Equal(ErrorUnknownAuthorizationDetailType.Code, svcErr.Code)

	_, svcErr = s.service.PrepareAuthorizationDetails(s.ctx, json.RawMessage(`not json`))
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A pushed request's types may change before it is used, so preparing validates again.
func (s *ServiceTestSuite) TestPrepare_ValidatesAgainstTheCurrentType() {
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(typeRef(flightType()), nil).Once()
	validated := s.validated(flightRequest)

	tightened := flightType()
	tightened.Version = 2
	tightened.Schema = json.RawMessage(`{"type":"object","required":["route","maxFare"],"properties":{` +
		`"route":{"type":"string","maxLength":3},"maxFare":{"type":"object"}}}`)
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(typeRef(tightened), nil).Once()
	_, svcErr := s.service.PrepareAuthorizationDetails(s.ctx, validated)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
}

func (s *ServiceTestSuite) TestProcess_AppliesPatch() {
	details := s.prepared(paymentRequest, paymentType())
	s.processor.EXPECT().Process(mock.Anything, *paymentType().Processor,
		mock.MatchedBy(func(req processorRequest) bool {
			return req.Subject == processorEntity{Type: string(providers.EntityCategoryUser), ID: testUserID} &&
				req.Client == processorEntity{Type: oauth2const.SubTypeApp, ID: testClientID} &&
				req.Resource == testResourceServer && len(req.Details) == 1 &&
				req.Details[0].TypeVersion == 3 && req.Details[0].DetailID == details[0].ID &&
				!strings.Contains(string(req.Details[0].Detail), req.Details[0].DetailID)
		})).RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
		req processorRequest) (*processorResponse, error) {
		return &processorResponse{Results: []processorResult{{
			DetailID: req.Details[0].DetailID, Decision: decision(true),
			Patch: map[string]json.RawMessage{"/debtorAccount/iban": json.RawMessage(`"DE12500105170648489890"`)},
		}}}, nil
	}).Once()

	admitted, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
	s.Require().Nil(svcErr)
	s.Require().Len(admitted, 1)
	s.True(admitted[0].Admitted)
	s.Equal(details[0].ID, admitted[0].ID)
	s.Contains(string(admitted[0].Value), "DE12500105170648489890")
	s.NotContains(string(admitted[0].Requested), "DE12", "the request is kept as the client sent it")
}

func (s *ServiceTestSuite) TestProcess_NoProcessorAdmitsUnchanged() {
	details := s.prepared(flightRequest, flightType())

	admitted, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
	s.Require().Nil(svcErr)
	s.Require().Len(admitted, 1)
	s.JSONEq(string(details[0].Value), string(admitted[0].Value))
	s.processor.AssertNotCalled(s.T(), "Process", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestProcess_Failures() {
	patchResult := func(detailID string, patch map[string]json.RawMessage) *processorResponse {
		return &processorResponse{Results: []processorResult{{
			DetailID: detailID, Decision: decision(true), Patch: patch}}}
	}
	cases := []struct {
		name   string
		answer func(req processorRequest) (*processorResponse, error)
		code   string
	}{
		{"refused", func(req processorRequest) (*processorResponse, error) {
			return &processorResponse{Results: []processorResult{refusal(req.Details[0].DetailID, "limit")}}, nil
		}, ErrorAuthorizationDetailRefused.Code},
		{"unreachable", func(processorRequest) (*processorResponse, error) {
			return nil, errProcessorUnavailable
		}, ErrorProcessorUnavailable.Code},
		{"missing result", func(processorRequest) (*processorResponse, error) {
			return &processorResponse{}, nil
		}, ErrorProcessorUnavailable.Code},
		{"patch outside server fields", func(req processorRequest) (*processorResponse, error) {
			patch := map[string]json.RawMessage{"/amount": json.RawMessage(`1`)}
			return patchResult(req.Details[0].DetailID, patch), nil
		}, ErrorProcessorUnavailable.Code},
		{"answered twice", func(req processorRequest) (*processorResponse, error) {
			result := processorResult{DetailID: req.Details[0].DetailID, Decision: decision(true)}
			return &processorResponse{Results: []processorResult{result, result}}, nil
		}, ErrorProcessorUnavailable.Code},
		{"no decision", func(req processorRequest) (*processorResponse, error) {
			undecided := processorResult{DetailID: req.Details[0].DetailID}
			return &processorResponse{Results: []processorResult{undecided}}, nil
		}, ErrorProcessorUnavailable.Code},
		{"patch key that is no pointer", func(req processorRequest) (*processorResponse, error) {
			return patchResult(req.Details[0].DetailID,
				map[string]json.RawMessage{"debtorAccount.iban": json.RawMessage(`"DE12"`)}), nil
		}, ErrorProcessorUnavailable.Code},
		{"patch breaks the schema", func(req processorRequest) (*processorResponse, error) {
			return patchResult(req.Details[0].DetailID,
				map[string]json.RawMessage{"/debtorAccount/iban": json.RawMessage(`42`)}), nil
		}, ErrorProcessorUnavailable.Code},
	}
	for _, tc := range cases {
		details := s.prepared(paymentRequest, paymentType())
		answer := tc.answer
		s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
				req processorRequest) (*processorResponse, error) {
				return answer(req)
			}).Once()
		_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
		s.Require().NotNil(svcErr, tc.name)
		s.Equal(tc.code, svcErr.Code, tc.name)
	}

	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, []providers.AuthorizationDetail{
		{ID: "d-1", Type: testPaymentType, Value: json.RawMessage(`[]`)}}, testApp, testSubject, testResourceServer)
	s.Require().NotNil(svcErr, "a detail that is not an object")
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestProcess_DropsRefusedDetails() {
	twoPayments := `[{"type":"` + testPaymentType + `","amount":45000.00},` +
		`{"type":"` + testPaymentType + `","amount":10.00}]`
	details := s.prepared(twoPayments, paymentType())
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
			req processorRequest) (*processorResponse, error) {
			return &processorResponse{Results: []processorResult{
				refusal(req.Details[0].DetailID, "over limit"),
				{DetailID: req.Details[1].DetailID, Decision: decision(true)},
			}}, nil
		}).Once()

	admitted, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
	s.Require().Nil(svcErr)
	s.Require().Len(admitted, 1)
	s.Equal(details[1].ID, admitted[0].ID)
}

func (s *ServiceTestSuite) TestProcess_RefusalCarriesTheReason() {
	details := s.prepared(paymentRequest, paymentType())
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
			req processorRequest) (*processorResponse, error) {
			long := strings.Repeat("x", common.MaxReasonLength+50)
			return &processorResponse{Results: []processorResult{refusal(req.Details[0].DetailID, long)}}, nil
		}).Once()

	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAuthorizationDetailRefused.Code, svcErr.Code)
	s.Len(svcErr.ErrorDescription.Params["reason"], common.MaxReasonLength)
}

// A reason carrying control or bidirectional formatting characters is not shown to the user.
func (s *ServiceTestSuite) TestProcess_RefusalDropsAnUnsafeReason() {
	details := s.prepared(paymentRequest, paymentType())
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ providers.AuthorizationDetailProcessor,
			req processorRequest) (*processorResponse, error) {
			return &processorResponse{Results: []processorResult{
				refusal(req.Details[0].DetailID, "limit \u202eexceeded")}}, nil
		}).Once()

	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorAuthorizationDetailRefused.Code, svcErr.Code)
	s.Empty(svcErr.ErrorDescription.Params["reason"])
}

func (s *ServiceTestSuite) TestProcess_BatchesDetailsPerEndpoint() {
	second := paymentType()
	second.Type = testFlightType
	second.Schema = json.RawMessage(paymentSchema)
	s.expectTypes(paymentType(), second)
	raw := `[{"type":"` + testPaymentType + `","amount":1},{"type":"` + testFlightType + `","amount":2}]`
	details, svcErr := s.service.PrepareAuthorizationDetails(s.ctx, s.validated(raw))
	s.Require().Nil(svcErr)
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.MatchedBy(func(req processorRequest) bool {
		return len(req.Details) == 2
	})).RunAndReturn(approveAll).Once()

	_, svcErr = s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestProcess_AgentIsTheClient() {
	details := s.prepared(paymentRequest, paymentType())
	agent := &providers.Application{ID: "agent-1", EntityCategory: providers.EntityCategoryAgent,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "agent-client"}}}}
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.MatchedBy(func(req processorRequest) bool {
		return req.Subject == processorEntity{Type: string(providers.EntityCategoryUser), ID: testUserID} &&
			req.Client == processorEntity{Type: oauth2const.SubTypeAgent, ID: "agent-client"}
	})).RunAndReturn(approveAll).Once()

	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, agent, testSubject, testResourceServer)
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestProcess_NamesAnAgentSubjectAsAnAgent() {
	details := s.prepared(paymentRequest, paymentType())
	s.processor.EXPECT().Process(mock.Anything, mock.Anything, mock.MatchedBy(func(req processorRequest) bool {
		return req.Subject == processorEntity{Type: oauth2const.SubTypeAgent, ID: "agent-subject"}
	})).RunAndReturn(approveAll).Once()

	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, providers.EntityReference{
		EntityID: "agent-subject", EntityCategory: string(providers.EntityCategoryAgent)}, testResourceServer)
	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestProcess_RefusedWhenTheTypeChanged() {
	details := s.prepared(paymentRequest, paymentType())
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testPaymentType).Unset()

	edited := paymentType()
	edited.Version++
	recreated := paymentType()
	recreated.ID = "type-2-recreated"
	for name, current := range map[string]*providers.AuthorizationDetailType{
		"edited": &edited, "deleted": nil, "deleted and created again": &recreated,
	} {
		s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testPaymentType).Return(current, nil).Once()
		_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
		s.Require().NotNil(svcErr, name)
		s.Equal(common.ErrorAuthorizationDetailTypeChanged.Code, svcErr.Code, name)
	}
	s.types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testPaymentType).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr := s.service.ProcessAuthorizationDetails(s.ctx, details, testApp, testSubject, testResourceServer)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.processor.AssertNotCalled(s.T(), "Process", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestGrantedFromClaim() {
	claim := json.RawMessage(`[{"id":"d-1","type":"pay","typeId":"t-1","typeVersion":2,` +
		`"requested":{"amount":9007199254740995},"value":{"amount":45.00},"admitted":true}]`)
	granted, err := GrantedFromClaim(claim)
	s.Require().NoError(err)
	s.Require().Len(granted, 1)
	s.Equal("d-1", granted[0].ID)
	s.Equal(2, granted[0].TypeVersion)
	s.Equal(`{"amount":9007199254740995}`, string(granted[0].Requested), "numbers are kept exactly")
	s.Equal(`{"amount":45.00}`, string(granted[0].Value), "numbers are kept as written")

	_, err = GrantedFromClaim(json.RawMessage(`"not a list"`))
	s.Error(err)

	for _, empty := range []string{`[]`, `null`} {
		granted, err = GrantedFromClaim(json.RawMessage(empty))
		s.Require().NoError(err)
		s.NotNil(granted, "a claim is a decision even when it grants none")
		s.Empty(granted)
	}
}

func mustLookup(t *testing.T, fields map[string]json.RawMessage, path string) json.RawMessage {
	t.Helper()
	value, ok := fields[path]
	for !ok {
		head, rest, nested := strings.Cut(path, ".")
		require.True(t, nested, path)
		var child map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(fields[head], &child), path)
		fields, path = child, rest
		value, ok = fields[path]
	}
	return value
}
