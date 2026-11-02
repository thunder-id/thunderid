// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DetailTestSuite struct {
	suite.Suite
}

func TestDetailTestSuite(t *testing.T) {
	suite.Run(t, new(DetailTestSuite))
}

func (s *DetailTestSuite) TestParseRequestedDetails_Rejects() {
	cases := map[string]string{
		"not an array":         `{"type":"a"}`,
		"blank":                `   `,
		"invalid JSON":         `[{"type":"a"`,
		"empty array":          `[]`,
		"non-object entry":     `["a"]`,
		"missing type":         `[{"actions":["read"]}]`,
		"non-string type":      `[{"type":7}]`,
		"empty type":           `[{"type":""}]`,
		"entry is not an JSON": `[{"type":"a"},1]`,
		"too many entries":     "[" + strings.Repeat(`{"type":"a"},`, maxDetailsPerRequest) + `{"type":"a"}]`,
	}
	for name, raw := range cases {
		_, err := ParseRequestedDetails(raw)
		s.Error(err, name)
	}
}

func (s *DetailTestSuite) TestParseRequestedDetails_AtMostTheLimit() {
	raw := "[" + strings.Repeat(`{"type":"a"},`, maxDetailsPerRequest-1) + `{"type":"a"}]`
	objects, err := ParseRequestedDetails(raw)
	s.Require().NoError(err)
	s.Len(objects, maxDetailsPerRequest)
}

func (s *DetailTestSuite) TestParseRequestedDetails_Valid() {
	objects, err := ParseRequestedDetails(`[{"type":"a","amount":450.00},{"type":"b"}]`)
	s.Require().NoError(err)
	s.Len(objects, 2)
	s.Equal("a", objects[0].Type)
	s.Equal("b", objects[1].Type)
	s.Equal(`450.00`, string(objects[0].Fields["amount"]))
}

func (s *DetailTestSuite) TestEncodeValues_PreservesValueBytes() {
	objects, err := ParseRequestedDetails(`[{"type":"a","big":12345678901234567890.123,"s":"xé"}]`)
	s.Require().NoError(err)
	objects[0].ID = "id-1"

	encoded, err := EncodeValues(objects)
	s.Require().NoError(err)
	s.JSONEq(`[{"type":"a","big":12345678901234567890.123,"s":"xé"}]`, string(encoded),
		"values carry no identifier")
	s.Contains(string(encoded), "12345678901234567890.123")
}

func (s *DetailTestSuite) TestEncodeDetailValues() {
	details := []providers.AuthorizationDetail{
		{ID: "d-1", Type: "pay", TypeID: "t-1", TypeVersion: 2, Requested: json.RawMessage(`{"type":"pay"}`),
			Value: json.RawMessage(`{"type":"pay", "amount":35.00}`)},
		{ID: "d-2", Type: "pay", TypeID: "t-1", TypeVersion: 2, Value: json.RawMessage(`{"type":"pay","amount":9}`)},
	}
	encoded, err := EncodeDetailValues(details)
	s.Require().NoError(err)
	s.Equal(`[{"type":"pay","amount":35.00},{"type":"pay","amount":9}]`, string(encoded),
		"only the values are carried, in order and as written")

	encoded, err = EncodeDetailValues([]providers.AuthorizationDetail{})
	s.Require().NoError(err)
	s.Nil(encoded, "no details carry no values")

	_, err = EncodeDetailValues([]providers.AuthorizationDetail{{Value: json.RawMessage(`bad`)}})
	s.Error(err)
}

func (s *DetailTestSuite) TestSetPath() {
	fields := map[string]json.RawMessage{"debtor": json.RawMessage(`{"name":"Acme"}`)}

	s.Require().NoError(setPath(fields, "debtor.iban", json.RawMessage(`"DE12"`)))
	s.JSONEq(`{"name":"Acme","iban":"DE12"}`, string(fields["debtor"]))

	s.Require().NoError(setPath(fields, "new.nested.value", json.RawMessage(`1`)))
	s.JSONEq(`{"nested":{"value":1}}`, string(fields["new"]))

	s.Require().NoError(setPath(fields, "flat", json.RawMessage(`true`)))
	s.Equal("true", string(fields["flat"]))

	fields["nullable"] = json.RawMessage(`null`)
	s.Require().NoError(setPath(fields, "nullable.x", json.RawMessage(`"y"`)))
	s.JSONEq(`{"x":"y"}`, string(fields["nullable"]))

	fields["scalar"] = json.RawMessage(`"text"`)
	s.Error(setPath(fields, "scalar.x", json.RawMessage(`1`)))
	s.Error(setPath(fields, ".x", json.RawMessage(`1`)))
}

func (s *DetailTestSuite) TestFlattenFieldsAndLookupPath() {
	fields := map[string]json.RawMessage{
		"type":    json.RawMessage(`"a"`),
		"maxFare": json.RawMessage(`{"amount":450,"currency":"USD"}`),
		"actions": json.RawMessage(`["create"]`),
		"empty":   json.RawMessage(`{}`),
	}
	paths := func(order map[string][]string) []string {
		entries := flattenFields(fields, "", order)
		paths := make([]string, 0, len(entries))
		for _, e := range entries {
			paths = append(paths, e.Path)
		}
		return paths
	}
	s.Equal([]string{"actions", "empty", "maxFare.amount", "maxFare.currency", "type"}, paths(nil))
	s.Equal([]string{"maxFare.currency", "maxFare.amount", "type", "actions", "empty"},
		paths(map[string][]string{"": {"maxFare", "missing", "type"}, "maxFare": {"currency", "amount"}}))

	v, ok := lookupPath(fields, "maxFare.amount")
	s.True(ok)
	s.Equal(`450`, string(v))

	_, ok = lookupPath(fields, "maxFare.missing")
	s.False(ok)
	_, ok = lookupPath(fields, "actions.first")
	s.False(ok)
	_, ok = lookupPath(fields, "absent")
	s.False(ok)
}

func (s *DetailTestSuite) TestPointerPath() {
	for pointer, want := range map[string]string{
		"/amount":             "amount",
		"/debtorAccount/iban": "debtorAccount.iban",
		"/a~1b":               "a/b",
		"/c~0d":               "c~d",
	} {
		got, err := PointerPath(pointer)
		s.NoError(err, pointer)
		s.Equal(want, got, pointer)
	}
	for _, pointer := range []string{"", "amount", "/", "/a//b", "/a.b"} {
		_, err := PointerPath(pointer)
		s.Error(err, pointer)
	}
}

func (s *DetailTestSuite) TestParseRequestedDetails_RefusesAmbiguousText() {
	cases := map[string]string{
		"repeated member":            `[{"type":"pay","amount":1,"amount":2}]`,
		"repeated nested member":     `[{"type":"pay","to":{"name":"a","x":[1,{"k":1}],"name":"b"}}]`,
		"bidirectional override":     `[{"type":"pay","to":"Merchant \u202eA"}]`,
		"bidirectional isolate":      `[{"type":"pay","to":"\u2067A"}]`,
		"control character":          `[{"type":"pay","to":"A\u0007"}]`,
		"control character in a key": `[{"type":"pay","\u0001":"A"}]`,
		"invalid UTF-8":              "[{\"type\":\"pay\",\"to\":\"M\xff\"}]",
		"lone surrogate":             `[{"type":"pay","to":"M\ud800"}]`,
	}
	for name, raw := range cases {
		_, err := ParseRequestedDetails(raw)
		s.Error(err, name)
	}
}

func (s *DetailTestSuite) TestParseRequestedDetails_AcceptsOrdinaryText() {
	objects, err := ParseRequestedDetails(`[{"type":"pay","to":{"name":"Café\tA"},"items":[{"a":1},{"a":2}],` +
		`"note":"line one\nline two"},{"type":"pay","to":{"name":"B"}}]`)
	s.Require().NoError(err)
	s.Len(objects, 2)
}
