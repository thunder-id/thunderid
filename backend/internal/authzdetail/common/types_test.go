// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authzdetailtypeprovidermock"
)

type TypesTestSuite struct {
	suite.Suite
}

func TestTypesTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}

func (s *TypesTestSuite) TestDetailFromValue() {
	obj, err := DetailFromValue("d-1", "flight", json.RawMessage(`{"type":"flight","route":"CMB-SIN"}`))
	s.Require().NoError(err)
	s.Equal("d-1", obj.ID)
	s.Equal("flight", obj.Type)
	s.JSONEq(`"CMB-SIN"`, string(obj.Fields["route"]))

	_, err = DetailFromValue("d-1", "flight", json.RawMessage(`[]`))
	s.Error(err, "a value that is not an object")
}

func (s *TypesTestSuite) TestStartedType() {
	started := consentFlightTypeDefinition()
	detail := providers.AuthorizationDetail{ID: "d-1", Type: started.Type, TypeID: started.ID,
		TypeVersion: started.Version}
	edited := started
	edited.Version++
	recreated := started
	recreated.ID = "type-1-recreated"

	provider := authzdetailtypeprovidermock.NewAuthorizationDetailTypeProviderMock(s.T())
	provider.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, started.Type).Return(&started, nil).Once()
	got, svcErr := StartedType(context.Background(), provider, log.GetLogger(), detail)
	s.Require().Nil(svcErr)
	s.Equal(started, got)

	for name, current := range map[string]*providers.AuthorizationDetailType{
		"edited": &edited, "deleted": nil, "deleted and created again": &recreated,
	} {
		provider.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, started.Type).Return(current, nil).Once()
		_, svcErr := StartedType(context.Background(), provider, log.GetLogger(), detail)
		s.Require().NotNil(svcErr, name)
		s.Equal(ErrorAuthorizationDetailTypeChanged.Code, svcErr.Code, name)
	}

	provider.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, started.Type).
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr = StartedType(context.Background(), provider, log.GetLogger(), detail)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	_, svcErr = StartedType(context.Background(), nil, log.GetLogger(), detail)
	s.Require().NotNil(svcErr, "without a type provider no type is registered")
	s.Equal(ErrorAuthorizationDetailTypeChanged.Code, svcErr.Code)
}

func (s *TypesTestSuite) TestApplyChanges() {
	typ := consentFlightTypeDefinition()
	base := consentFlightDetail(s.T(), "450")

	changed, err := ApplyChanges(base, typ, map[string]json.RawMessage{"maxFare.amount": json.RawMessage(`300`)},
		PartyUser)
	s.Require().NoError(err)
	s.Equal(base.ID, changed.ID)
	s.JSONEq(`{"amount":300,"currency":"USD"}`, string(changed.Fields["maxFare"]))
	s.JSONEq(`{"amount":450,"currency":"USD"}`, string(base.Fields["maxFare"]), "the base is left as it is")

	cases := map[string]struct {
		changes map[string]json.RawMessage
		party   string
	}{
		"path through a list": {map[string]json.RawMessage{"actions.first": json.RawMessage(`1`)}, PartyUser},
		"raises the cap":      {map[string]json.RawMessage{"maxFare.amount": json.RawMessage(`500`)}, PartyUser},
		"party may not change it": {map[string]json.RawMessage{"maxFare.amount": json.RawMessage(`300`)},
			PartyProcessor},
	}
	for name, c := range cases {
		_, err := ApplyChanges(base, typ, c.changes, c.party)
		s.ErrorIs(err, ErrChangeRefused, name)
	}

	_, err = ApplyChanges(base, typ, map[string]json.RawMessage{
		"maxFare":        json.RawMessage(`{"amount":300,"currency":"USD"}`),
		"maxFare.amount": json.RawMessage(`200`)}, PartyUser)
	s.ErrorIs(err, ErrChangeRefused)
	s.ErrorContains(err, "overlap", "the order overlapping changes are applied in would decide the result")

	unknown := map[string]json.RawMessage{"maxFare.amount": json.RawMessage(`300`)}
	for i := range 20000 {
		unknown[fmt.Sprintf("extra.f%d", i)] = json.RawMessage(`1`)
	}
	_, err = ApplyChanges(base, typ, unknown, PartyUser)
	s.ErrorIs(err, ErrChangeRefused)
	s.ErrorContains(err, "is not part of the type", "paths outside the schema are refused before they are compared")

	conformance := providers.AuthorizationDetailType{ID: "type-4", Type: "cap", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{"cap":{"type":"number","minimum":0,` +
			`"policy":{"modification":"decrease","modifiableBy":["user"]}}}}`)}
	capped := Detail{ID: "d-4", Type: "cap", Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"cap"`), "cap": json.RawMessage(`10`)}}
	_, err = ApplyChanges(capped, conformance, map[string]json.RawMessage{"cap": json.RawMessage(`-1`)},
		PartyUser)
	s.ErrorIs(err, ErrChangeRefused, "does not conform to its type")

	limited := providers.AuthorizationDetailType{ID: "type-5", Type: "until", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{"until":{"type":"string","format":"date",` +
			`"policy":{"modification":"decrease","modifiableBy":["user"],"minimum":"2026-01-01"}}}}`)}
	dated := Detail{ID: "d-5", Type: "until", Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"until"`), "until": json.RawMessage(`"2026-06-01"`)}}
	_, err = ApplyChanges(dated, limited, map[string]json.RawMessage{"until": json.RawMessage(`"2025-01-01"`)},
		PartyUser)
	s.ErrorIs(err, ErrChangeRefused, "outside its limits")

	named := providers.AuthorizationDetailType{ID: "type-7", Type: "named", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string",` +
			`"policy":{"suppliedBy":"processor"}}}}`)}
	unnamed := Detail{ID: "d-7", Type: "named", Fields: map[string]json.RawMessage{"type": json.RawMessage(`"named"`)}}
	_, err = ApplyChanges(unnamed, named, map[string]json.RawMessage{"name": json.RawMessage(`"ACME"`)},
		PartyProcessor)
	s.Require().NoError(err)
	_, err = ApplyChanges(unnamed, named, map[string]json.RawMessage{"name": json.RawMessage(`"\u202eACME"`)},
		PartyProcessor)
	s.ErrorIs(err, ErrChangeRefused, "a value with a bidirectional formatting character")

	account := providers.AuthorizationDetailType{ID: "type-8", Type: "account", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{"account":{"type":"object","properties":{` +
			`"iban":{"type":"string","policy":{"suppliedBy":"processor"}}}}}}`)}
	emptyAccount := Detail{ID: "d-8", Type: "account", Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"account"`), "account": json.RawMessage(`{}`)}}
	filled, err := ApplyChanges(emptyAccount, account,
		map[string]json.RawMessage{"account.iban": json.RawMessage(`"DE1"`)}, PartyProcessor)
	s.Require().NoError(err, "the processor fills a field into an empty object the client sent")
	s.JSONEq(`{"iban":"DE1"}`, string(filled.Fields["account"]))
	_, err = ApplyChanges(unnamed, named, map[string]json.RawMessage{"name": json.RawMessage("\"AC\xffME\"")},
		PartyProcessor)
	s.ErrorIs(err, ErrChangeRefused, "a value that is not valid UTF-8")

	broken := providers.AuthorizationDetailType{ID: "type-6", Type: "broken", Version: 1,
		Schema: json.RawMessage(`{"type":"string"}`)}
	_, err = ApplyChanges(Detail{Type: "broken", Fields: map[string]json.RawMessage{}}, broken, nil,
		PartyUser)
	s.Require().Error(err)
	s.NotErrorIs(err, ErrChangeRefused, "a schema that cannot be compiled is not a refusal")
}

const consentFlightType = "https://scheme.example.com/flight/v1"

func consentFlightTypeDefinition() providers.AuthorizationDetailType {
	return providers.AuthorizationDetailType{
		ID: "type-1", Type: consentFlightType, Name: "Flight booking", Version: 1,
		Schema:          json.RawMessage(flightSchema),
		ConsentTemplate: "Book a {{route}} flight up to {{maxFare.amount}} {{maxFare.currency}}",
		Reuse:           true,
	}
}

func consentFlightDetail(t *testing.T, amount string) Detail {
	t.Helper()
	fields := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal([]byte(`{"type":"`+consentFlightType+`","actions":["create"],`+
		`"route":"CMB-SIN","maxFare":{"amount":`+amount+`,"currency":"USD"}}`), &fields))
	return Detail{ID: "d-1", Type: consentFlightType, Fields: fields}
}
