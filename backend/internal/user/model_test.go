// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestEntityToUserProjectsVerificationWithoutInternalMetadata() {
	entity := &providers.Entity{ID: "user-a", Category: providers.EntityCategoryUser,
		Attributes: json.RawMessage(`{"email":"user@example.com"}`),
		SystemAttributes: json.RawMessage(
			`{"attributeVerification":{"email":{"verifiedAt":"2026-10-16T12:00:00Z"}},"internal":"private-marker"}`)}

	projected := entityToUser(entity)

	s.Nil(projected.VerifiedAttributes, "stored metadata is not projected as verification")
	projected.VerifiedAttributes = map[string]providers.VerifiedAttribute{"email": {VerifiedAt: "2026-10-16T12:00:00Z"}}
	encoded, err := json.Marshal(projected)
	s.Require().NoError(err)
	var response map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(encoded, &response))
	s.JSONEq(`{"email":{"verifiedAt":"2026-10-16T12:00:00Z"}}`, string(response["verifiedAttributes"]))
	s.NotContains(response, "systemAttributes")
	s.NotContains(string(encoded), "attributeVerification")
	s.NotContains(string(encoded), "private-marker")
	s.Empty(userToEntity(&projected).SystemAttributes, "projection cannot be written back as proof")
}

// A user whose verification was not read omits the field; one read with none verified reports {}.
func (s *ModelTestSuite) TestUserOmitsVerificationOnlyWhenNotRead() {
	notRead, err := json.Marshal(entityToUser(&providers.Entity{ID: "user-a"}))
	s.Require().NoError(err)
	s.NotContains(string(notRead), "verifiedAttributes")

	read, err := json.Marshal(providers.User{ID: "user-a",
		VerifiedAttributes: map[string]providers.VerifiedAttribute{}})
	s.Require().NoError(err)
	s.Contains(string(read), `"verifiedAttributes":{}`)
}
