// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const consentFlightType = "https://scheme.example.com/flight/v1"

type ConsentTestSuite struct {
	suite.Suite
}

func TestConsentTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentTestSuite))
}

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

func flightApproval(amount string, version int) Approval {
	return Approval{TypeID: "type-1", TypeVersion: version,
		Granted: json.RawMessage(`{"type":"` + consentFlightType + `","actions":["create"],"route":"CMB-SIN",` +
			`"maxFare":{"amount":` + amount + `,"currency":"USD"}}`)}
}

func (s *ConsentTestSuite) TestDetailElement() {
	p := DetailElement(consentFlightDetail(s.T(), "450.00"), consentFlightTypeDefinition())

	s.Equal("d-1", p.Name)
	s.Equal(consentFlightType, p.DetailType)
	s.Equal("Book a CMB-SIN flight up to 450.00 USD", p.Summary)

	rows := map[string]providers.ConsentDetailRow{}
	paths := make([]string, 0, len(p.Rows))
	for _, r := range p.Rows {
		rows[r.Path] = r
		paths = append(paths, r.Path)
	}
	s.Equal([]string{"actions", "route", "maxFare.amount", "maxFare.currency"}, paths,
		"rows follow the order the schema declares its properties")
	s.NotContains(rows, "type")
	s.Equal("Spend cap", rows["maxFare.amount"].Label)
	s.Equal("decrease", rows["maxFare.amount"].Modification)
	s.Equal("number", rows["maxFare.amount"].As)
	s.Equal("maxFare.currency", rows["maxFare.currency"].Label, "an undeclared field is labeled by its path")
	s.JSONEq(`["create"]`, string(rows["actions"].Value))
}

func (s *ConsentTestSuite) TestDetailElement_RowsFollowTheDeclaredPropertyOrder() {
	typ := providers.AuthorizationDetailType{ID: "type-5", Type: "transfer", Name: "Transfer", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"payee":{"type":"object","properties":{"name":{"type":"string"},"iban":{"type":"string"}}},` +
			`"amount":{"type":"number"},"currency":{"type":"string"}}}`)}
	detail := Detail{ID: "d-5", Type: "transfer", Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"transfer"`), "currency": json.RawMessage(`"EUR"`),
		"amount": json.RawMessage(`10`), "payee": json.RawMessage(`{"iban":"DE12","name":"Ann"}`),
		"zNote": json.RawMessage(`"x"`), "aNote": json.RawMessage(`"y"`)}}

	rows := DetailElement(detail, typ).Rows
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		paths = append(paths, row.Path)
	}
	s.Equal([]string{"payee.name", "payee.iban", "amount", "currency", "aNote", "zNote"}, paths,
		"declared fields follow the schema at every level, and fields it does not declare follow by name")
}

func (s *ConsentTestSuite) TestDetailElement_ModificationOnlyWhenTheUserMayChangeTheField() {
	typ := providers.AuthorizationDetailType{ID: "type-3", Type: "transfer", Name: "Transfer", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"cap":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user"]}},` +
			`"limit":{"type":"number","policy":{"modification":"decrease","modifiableBy":["client"]}}}}`)}
	detail := Detail{ID: "d-3", Type: "transfer", Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"transfer"`), "cap": json.RawMessage(`10`), "limit": json.RawMessage(`10`)}}

	rows := map[string]providers.ConsentDetailRow{}
	for _, row := range DetailElement(detail, typ).Rows {
		rows[row.Path] = row
	}
	s.Equal("decrease", rows["cap"].Modification)
	s.Empty(rows["limit"].Modification, "a field only the client may change is shown read-only")
}

func (s *ConsentTestSuite) TestDetailElement_OffersTheValuesTheUserMayAdd() {
	typ := providers.AuthorizationDetailType{ID: "type-4", Type: "card", Name: "Card", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"excluded":{"type":"array","items":{"type":"string","enum":["gambling","crypto","travel"]},` +
			`"policy":{"modification":"addItems","modifiableBy":["user"]}},` +
			`"blocked":{"type":"array","items":{"type":"string"},` +
			`"policy":{"modification":"addItems","modifiableBy":["user"]}},` +
			`"actions":{"type":"array","items":{"type":"string","enum":["pay","refund"]},` +
			`"policy":{"modification":"removeItems","modifiableBy":["user"]}}}}`)}
	detail := Detail{ID: "d-4", Type: "card", Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"card"`), "excluded": json.RawMessage(`["gambling"]`),
		"blocked": json.RawMessage(`["x"]`), "actions": json.RawMessage(`["pay","refund"]`)}}

	rows := map[string]providers.ConsentDetailRow{}
	for _, row := range DetailElement(detail, typ).Rows {
		rows[row.Path] = row
	}
	s.Equal("addItems", rows["excluded"].Modification)
	s.JSONEq(`["gambling","crypto","travel"]`, string(rows["excluded"].Options))
	s.Empty(rows["blocked"].Options, "a list that allows any value offers no options")
	s.Empty(rows["actions"].Options, "options are offered only where the user may add items")
}

func (s *ConsentTestSuite) TestDetailElement_MarksProcessorSuppliedFields() {
	typ := providers.AuthorizationDetailType{ID: "type-2", Type: "payment", Name: "Payment", Version: 1,
		Schema: json.RawMessage(`{"type":"object","properties":{"amount":{"type":"number"},` +
			`"iban":{"type":"string","policy":{"suppliedBy":"processor"}}}}`)}
	detail := Detail{ID: "d-2", Type: "payment", Fields: map[string]json.RawMessage{
		"type": json.RawMessage(`"payment"`), "amount": json.RawMessage(`10`), "iban": json.RawMessage(`"DE12"`)}}

	p := DetailElement(detail, typ)
	for _, row := range p.Rows {
		if row.Path == "iban" {
			s.Equal(providers.ConsentDetailRowOriginServer, row.Origin)
			return
		}
	}
	s.FailNow("the processor-supplied field is not listed")
}

func (s *ConsentTestSuite) TestRenderConsentTemplate() {
	fields := map[string]json.RawMessage{
		"route":   json.RawMessage(`"CMB-SIN"`),
		"actions": json.RawMessage(`["create","cancel",1]`),
		"count":   json.RawMessage(`3`),
		"nested":  json.RawMessage(`{"a":"b"}`),
		"none":    json.RawMessage(`null`),
	}
	s.Equal("CMB-SIN: create, cancel, 1 x3 {\"a\":\"b\"}",
		renderConsentTemplate("{{ route }}: {{actions}} x{{count}} {{nested}}", fields))
	s.Empty(renderConsentTemplate("{{route}} [{{none}}]", fields), "a null value leaves the summary out")
	s.Empty(renderConsentTemplate("{{route}} [{{missing}}]", fields),
		"a missing value leaves the summary out")
	s.Equal("", renderConsentTemplate("", fields))
	s.Equal("", formatValue(json.RawMessage(`not json`)))
	s.Equal("9007199254740993, 45.00, a", formatValue(json.RawMessage(`[9007199254740993, 45.00, "a"]`)),
		"list numbers are shown exactly as granted")
}

func (s *ConsentTestSuite) TestIsCovered() {
	typ := consentFlightTypeDefinition()
	s.True(IsCovered(consentFlightDetail(s.T(), "450.00"), typ, []Approval{flightApproval("500.00", 1)}),
		"a request within an earlier approval is covered")

	recreated := consentFlightTypeDefinition()
	recreated.ID = "type-1-recreated"
	oneOff := consentFlightTypeDefinition()
	oneOff.Reuse = false
	cases := map[string]struct {
		typ       providers.AuthorizationDetailType
		approvals []Approval
	}{
		"request exceeds the approval":   {typ, []Approval{flightApproval("400.00", 1)}},
		"type version changed":           {typ, []Approval{flightApproval("500.00", 0)}},
		"type deleted and created again": {recreated, []Approval{flightApproval("500.00", 1)}},
		"no earlier approval":            {typ, nil},
		"type never reuses":              {oneOff, []Approval{flightApproval("500.00", 1)}},
		"approval unreadable": {typ, []Approval{
			{TypeID: "type-1", TypeVersion: 1}}},
	}
	for name, tc := range cases {
		s.False(IsCovered(consentFlightDetail(s.T(), "450.00"), tc.typ, tc.approvals), name)
	}
}

// A request that leaves out a list the approval has is not covered, since a missing list can read as unrestricted.
func (s *ConsentTestSuite) TestIsCovered_LeftOutListIsNotCovered() {
	typ := providers.AuthorizationDetailType{ID: "type-a", Type: "account_access", Version: 1, Reuse: true,
		Schema: json.RawMessage(`{"type":"object","properties":{"accounts":{"type":"array",` +
			`"items":{"type":"string"},"policy":{"modification":"removeItems","modifiableBy":["user","client"]}}}}`)}
	approvals := []Approval{{TypeID: "type-a", TypeVersion: 1,
		Granted: json.RawMessage(`{"type":"account_access","accounts":["acc-1","acc-2"]}`)}}
	detail := func(value string) Detail {
		obj, err := DetailFromValue("d-1", "account_access", json.RawMessage(value))
		s.Require().NoError(err)
		return obj
	}

	s.True(IsCovered(detail(`{"type":"account_access","accounts":["acc-1"]}`), typ, approvals))
	s.True(IsCovered(detail(`{"type":"account_access","accounts":[]}`), typ, approvals),
		"an empty list is the narrowest value")
	s.False(IsCovered(detail(`{"type":"account_access"}`), typ, approvals))
}
