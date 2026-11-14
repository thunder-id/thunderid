// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// The service is built over the given entity type service, so a collected value is pinned by its schema.
func (s *InitTestSuite) TestInitializeBuildsTheService() {
	service, err := Initialize(entitymock.NewEntityServiceInterfaceMock(s.T()), contactSchema(s.T()), nil, nil,
		discardingBus{})
	s.Require().NoError(err)
	var provider providers.AttributeVerificationProvider = service

	pin, svcErr := provider.PrepareAttributeVerification(context.Background(),
		pendingRequest("person", "workEmail", " User@Example.com "))

	s.Require().Nil(svcErr)
	s.Equal("User@example.com", pin.Value)
}
