// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestInvalidationRemovesTheEntryOfAChangedValue() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"workEmail":"User@EXAMPLE.COM","mobile":"+94771234567"}`),
		SystemAttributes: json.RawMessage(`{"keep":true,"attributeVerification":` +
			`{"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`),
	}
	tests := []struct {
		name, attributes, nextType string
		verified, changed          bool
	}{
		{"canonical equality", `{"workEmail":" User@example.com ","mobile":"+94771234567"}`, "person", true, false},
		{"local part changed", `{"workEmail":"user@example.com","mobile":"+94771234567"}`, "person", false, true},
		{"email removed", `{"mobile":"+94771234567"}`, "person", false, true},
		{"user type changed", `{"workEmail":"User@EXAMPLE.COM","mobile":"+94771234567"}`, "employee", false, true},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			result, cleared, err := invalidateVerification(current, test.nextType, declaredVerifiableAttributes,
				json.RawMessage(test.attributes), current.SystemAttributes)
			s.Require().NoError(err)
			s.Require().Equal(test.changed, len(cleared) > 0)
			entries, err := identitygovernance.ReadVerifiedAttributes(result)
			s.Require().NoError(err)
			_, verified := entries["workEmail"]
			s.Require().Equal(test.verified, verified)
			s.Require().Contains(string(result), `"keep":true`)
		})
	}
	// A -> B -> A does not bring the verification back.
	for _, value := range []string{`"other@example.com"`, `"User@EXAMPLE.COM"`} {
		next := json.RawMessage(`{"workEmail":` + value + `,"mobile":"+94771234567"}`)
		result, _, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes, next,
			current.SystemAttributes)
		s.Require().NoError(err)
		current.Attributes, current.SystemAttributes = next, result
	}
	entries, err := identitygovernance.ReadVerifiedAttributes(current.SystemAttributes)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *UtilsTestSuite) TestInvalidationComparesAnIneligibleAttributeExactly() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"formerEmail":"a@example.com"}`),
		SystemAttributes: json.RawMessage(
			`{"attributeVerification":{"formerEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`),
	}
	updated, cleared, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"formerEmail":"A@example.com"}`), current.SystemAttributes)
	s.Require().NoError(err)
	s.Require().Equal([]clearedVerification{
		{attribute: "formerEmail", reason: identitygovernance.VerificationValueChanged}}, cleared)
	entries, err := identitygovernance.ReadVerifiedAttributes(updated)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *UtilsTestSuite) TestInvalidationWritesNothingWithoutAVerification() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"workEmail":"a@example.com"}`)}
	updated, cleared, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":"new@example.com"}`), json.RawMessage(`{"keep":true}`))
	s.Require().NoError(err)
	s.Require().Empty(cleared)
	s.Require().JSONEq(`{"keep":true}`, string(updated))
}

func (s *UtilsTestSuite) TestInvalidationCanReplaceAnOldNonStringContact() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes:       json.RawMessage(`{"workEmail":42}`),
		SystemAttributes: json.RawMessage(verifiedWorkEmail),
	}
	updated, cleared, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":"new@example.com"}`), current.SystemAttributes)
	s.Require().NoError(err)
	s.Require().NotEmpty(cleared)
	entries, err := identitygovernance.ReadVerifiedAttributes(updated)
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *UtilsTestSuite) TestInvalidationReportsAnEmptyValueAsRemoved() {
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)

	_, cleared, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":" "}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Equal([]clearedVerification{
		{attribute: "workEmail", reason: identitygovernance.VerificationValueRemoved}}, cleared)
}

func (s *UtilsTestSuite) TestInvalidationReportsANullValueAsRemoved() {
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)

	_, cleared, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":null}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Equal([]clearedVerification{
		{attribute: "workEmail", reason: identitygovernance.VerificationValueRemoved}}, cleared)
}

func (s *UtilsTestSuite) TestInvalidationLeavesANonUserAlone() {
	current := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)
	current.Category = providers.EntityCategoryAgent

	updated, cleared, err := invalidateVerification(current, "other", nil,
		json.RawMessage(`{"workEmail":"b@example.com"}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Empty(cleared)
	s.JSONEq(verifiedWorkEmail, string(updated))
}

func (s *UtilsTestSuite) TestUserTypeChangeClearsEveryVerification() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"workEmail":"a@example.com","mobile":"+94771234567"}`),
		SystemAttributes: json.RawMessage(`{"attributeVerification":{` +
			`"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"},"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)}

	updated, cleared, err := invalidateVerification(current, "employee", declaredVerifiableAttributes,
		current.Attributes, current.SystemAttributes)

	s.Require().NoError(err)
	s.Equal([]clearedVerification{{attribute: "mobile", reason: identitygovernance.VerificationUserTypeChanged},
		{attribute: "workEmail", reason: identitygovernance.VerificationUserTypeChanged}}, cleared,
		"cleared in attribute order")
	entries, err := identitygovernance.ReadVerifiedAttributes(updated)
	s.Require().NoError(err)
	s.Empty(entries)
}

// Changed values are cleared in attribute order, so their events are published in a fixed order.
func (s *UtilsTestSuite) TestChangedValuesAreClearedInAttributeOrder() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(`{"e":"1","b":"1","d":"1","a":"1","c":"1"}`),
		SystemAttributes: json.RawMessage(`{"attributeVerification":{"e":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
			`"b":{"verifiedAt":"2026-10-15T00:00:00Z"},"d":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
			`"a":{"verifiedAt":"2026-10-15T00:00:00Z"},"c":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)}

	_, cleared, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"e":"2","b":"2","d":"2","a":"2","c":"2"}`), current.SystemAttributes)

	s.Require().NoError(err)
	names := make([]string, 0, len(cleared))
	for _, c := range cleared {
		names = append(names, c.attribute)
	}
	s.Equal([]string{"a", "b", "c", "d", "e"}, names)
}

func (s *UtilsTestSuite) TestAnUnrelatedUpdateLeavesVerificationAlone() {
	current := providers.Entity{Category: providers.EntityCategoryUser, Type: "person",
		Attributes:       json.RawMessage(`{"workEmail":"a@example.com","nickname":"a"}`),
		SystemAttributes: json.RawMessage(verifiedWorkEmail)}

	updated, cleared, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
		json.RawMessage(`{"workEmail":"a@example.com","nickname":"b"}`), current.SystemAttributes)

	s.Require().NoError(err)
	s.Empty(cleared)
	s.JSONEq(string(current.SystemAttributes), string(updated))
}

func (s *UtilsTestSuite) TestRemovedAndRestoredMobileIsUnverified() {
	current := verificationUser(`{"mobile":"+94771234567"}`,
		`{"attributeVerification":{"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)

	for _, next := range []string{`{}`, `{"mobile":"+94771234567"}`} {
		result, _, err := invalidateVerification(current, current.Type, declaredVerifiableAttributes,
			json.RawMessage(next), current.SystemAttributes)
		s.Require().NoError(err)
		current.Attributes, current.SystemAttributes = json.RawMessage(next), result
	}

	entries, err := identitygovernance.ReadVerifiedAttributes(current.SystemAttributes)
	s.Require().NoError(err)
	s.Empty(entries)
}

func (s *UtilsTestSuite) TestInvalidationRefusesMalformedJSON() {
	stored := verificationUser(`{"workEmail":"a@example.com"}`, verifiedWorkEmail)
	malformedMetadata := stored
	malformedMetadata.SystemAttributes = json.RawMessage(`{`)
	malformedAttributes := stored
	malformedAttributes.Attributes = json.RawMessage(`[`)
	for name, test := range map[string]struct {
		current providers.Entity
		next    string
	}{
		"stored metadata":   {malformedMetadata, `{"workEmail":"b@example.com"}`},
		"stored attributes": {malformedAttributes, `{"workEmail":"b@example.com"}`},
		"next attributes":   {stored, `[`},
	} {
		s.Run(name, func() {
			_, _, err := invalidateVerification(test.current, test.current.Type, declaredVerifiableAttributes,
				json.RawMessage(test.next), test.current.SystemAttributes)

			s.Error(err)
		})
	}
}
