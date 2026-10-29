// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MetadataTestSuite struct {
	suite.Suite
}

func TestMetadataTestSuite(t *testing.T) {
	suite.Run(t, new(MetadataTestSuite))
}

func (s *MetadataTestSuite) TestRequestSchema_KeepsOrderAndDropsProcessorFieldsAndPolicies() {
	raw := json.RawMessage(`{"type":"object","required":["route","fare","payee"],"properties":{` +
		`"route":{"type":"string","policy":{"suppliedBy":"client"}},` +
		`"payee":{"type":"string","policy":{"suppliedBy":"processor"}},` +
		`"fare":{"type":"object","required":["amount","rate"],"properties":{` +
		`"rate":{"type":"number","policy":{"suppliedBy":"processor"}},` +
		`"amount":{"type":"number","maximum":500,"policy":{"modification":"decrease","modifiableBy":["user"]}}}}}}`)

	view, err := RequestSchema(raw)

	s.Require().NoError(err)
	s.Equal(`{"type":"object","required":["route","fare"],"properties":{`+
		`"route":{"type":"string"},`+
		`"fare":{"type":"object","required":["amount"],"properties":{"amount":{"type":"number","maximum":500}}}}}`,
		string(view), "declared order is kept, and processor fields and policies are left out")
}

func (s *MetadataTestSuite) TestRequestSchema_LeavesSchemaWithoutPoliciesUnchanged() {
	raw := `{"type":"object","properties":{"b":{"type":"string"},"a":{"type":"number"}},"required":["b"]}`

	view, err := RequestSchema(json.RawMessage(raw))

	s.Require().NoError(err)
	s.Equal(raw, string(view))
}

func (s *MetadataTestSuite) TestRequestSchema_RefusesUnreadablePolicies() {
	_, err := RequestSchema(json.RawMessage(`{"type":"object","properties":{"a":{"type":"string",` +
		`"policy":{"suppliedBy":"processor","modification":"bogus"}}}}`))

	s.Error(err)
}

func (s *MetadataTestSuite) TestClientModifications_ListsOnlyChangesTheClientMayMake() {
	raw := json.RawMessage(`{"type":"object","properties":{` +
		`"limits":{"type":"object","properties":{` +
		`"max":{"type":"number","policy":{"modification":"decrease","modifiableBy":["client"]}}}},` +
		`"actions":{"type":"array","items":{"type":"string"},` +
		`"policy":{"modification":"removeItems","modifiableBy":["user","client"]}},` +
		`"note":{"type":"string","policy":{"modification":"none"}},` +
		`"until":{"type":"string","format":"date","policy":{"modification":"decrease","modifiableBy":["user"]}}}}`)

	s.Equal([]ClientModification{
		{Field: "actions", Modification: modificationRemoveItems},
		{Field: "limits.max", Modification: modificationDecrease},
	}, ClientModifications(raw))
}

func (s *MetadataTestSuite) TestClientModifications_EmptyWhenNothingIsModifiable() {
	s.Empty(ClientModifications(json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`)))
}
