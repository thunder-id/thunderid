// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	providerRSID       = "rs-ca"
	providerRSAudience = "https://api.dmv.ca.gov"
	providerResourceID = "resource-1"
	providerActionID   = "action-1"
)

type AdminProviderTestSuite struct {
	suite.Suite
	resources *ResourceServiceInterfaceMock
	provider  AdminProviderInterface
}

func TestAdminProviderTestSuite(t *testing.T) {
	suite.Run(t, new(AdminProviderTestSuite))
}

func (s *AdminProviderTestSuite) SetupTest() {
	s.resources = NewResourceServiceInterfaceMock(s.T())
	s.provider = newAdminProvider(s.resources)
}

func (s *AdminProviderTestSuite) mutableResourceServer() {
	s.resources.On("IsResourceServerDeclarative", providerRSID).Return(false)
	s.resources.On("GetResourceServer", mock.Anything, providerRSID).
		Return(&providers.ResourceServer{ID: providerRSID, Identifier: providerRSAudience}, nil)
}

// The scope a retired action defines is identified by the audience and the permission together, since
// a permission string is unique only within its resource server.
func (s *AdminProviderTestSuite) TestValidateDeleteAction_PairsThePermissionWithTheAudience() {
	s.mutableResourceServer()
	s.resources.On("GetAction", mock.Anything, providerRSID, mock.Anything, providerActionID).
		Return(&providers.Action{ID: providerActionID, Permission: "license:issue"}, nil)

	target, svcErr := s.provider.ValidateDeleteAction(
		context.Background(), providerRSID, providerResourceID, providerActionID)

	s.Require().Nil(svcErr)
	s.Equal([]revocation.AudienceScope{
		{Audience: providerRSAudience, Scope: "license:issue"},
	}, target.Scopes)
	s.Empty(target.EntityIDs,
		"a retired scope should be held by nobody, so the revocation names no principals")
}

// An action defined on the resource server rather than under one of its resources is located with no
// resource. The service rejects a pointer to an empty string, so the absence must become nil.
func (s *AdminProviderTestSuite) TestValidateDeleteAction_AcceptsAnActionWithNoResource() {
	s.mutableResourceServer()
	s.resources.On("GetAction", mock.Anything, providerRSID, (*string)(nil), providerActionID).
		Return(&providers.Action{ID: providerActionID, Permission: "license:issue"}, nil)

	target, svcErr := s.provider.ValidateDeleteAction(
		context.Background(), providerRSID, "", providerActionID)

	s.Require().Nil(svcErr)
	s.Len(target.Scopes, 1)
}

// A named resource is passed through as the pointer the service expects.
func (s *AdminProviderTestSuite) TestValidateDeleteAction_ScopesTheLookupToTheResource() {
	s.mutableResourceServer()
	s.resources.On("GetAction", mock.Anything, providerRSID,
		mock.MatchedBy(func(id *string) bool { return id != nil && *id == providerResourceID }),
		providerActionID).
		Return(&providers.Action{ID: providerActionID, Permission: "license:issue"}, nil)

	_, svcErr := s.provider.ValidateDeleteAction(
		context.Background(), providerRSID, providerResourceID, providerActionID)

	s.Nil(svcErr)
}

// A declarative catalog owns its actions from a file, so the deletion would be refused. Refusing here
// keeps the flow from retiring a scope that is about to still exist.
func (s *AdminProviderTestSuite) TestValidateDeleteAction_RefusesDeclarativeCatalogue() {
	s.resources.On("IsResourceServerDeclarative", providerRSID).Return(true)

	_, svcErr := s.provider.ValidateDeleteAction(
		context.Background(), providerRSID, "", providerActionID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorImmutableAction.Code, svcErr.Code)
}

// A resource server with no identifier issues no audience, so no token can carry this scope in a form a
// criterion could match. The caller reports nothing to revoke and still performs the deletion.
func (s *AdminProviderTestSuite) TestValidateDeleteAction_ResourceServerWithoutIdentifier() {
	s.resources.On("IsResourceServerDeclarative", providerRSID).Return(false)
	s.resources.On("GetResourceServer", mock.Anything, providerRSID).
		Return(&providers.ResourceServer{ID: providerRSID}, nil)
	s.resources.On("GetAction", mock.Anything, providerRSID, mock.Anything, providerActionID).
		Return(&providers.Action{ID: providerActionID, Permission: "license:issue"}, nil)

	target, svcErr := s.provider.ValidateDeleteAction(
		context.Background(), providerRSID, "", providerActionID)

	s.Require().Nil(svcErr)
	s.Empty(target.Scopes)
}

func (s *AdminProviderTestSuite) TestValidateDeleteAction_RequiresBothIdentifiers() {
	_, svcErr := s.provider.ValidateDeleteAction(context.Background(), "", "", providerActionID)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingID.Code, svcErr.Code)

	_, svcErr = s.provider.ValidateDeleteAction(context.Background(), providerRSID, "", "")
	s.Require().NotNil(svcErr)
	s.Equal(ErrorMissingID.Code, svcErr.Code)
}

// The service's own refusal must reach the caller, since the flow surfaces its code to the operator.
func (s *AdminProviderTestSuite) TestValidateDeleteAction_CarriesTheServiceRefusal() {
	s.resources.On("IsResourceServerDeclarative", providerRSID).Return(false)
	s.resources.On("GetResourceServer", mock.Anything, providerRSID).
		Return(nil, &ErrorResourceServerNotFound)

	_, svcErr := s.provider.ValidateDeleteAction(
		context.Background(), providerRSID, "", providerActionID)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorResourceServerNotFound.Code, svcErr.Code)
}

func (s *AdminProviderTestSuite) TestDeleteAction_CarriesTheServiceRefusal() {
	refusal := tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "RES-1030"}
	s.resources.On("DeleteAction", mock.Anything, providerRSID, (*string)(nil), providerActionID).
		Return(&refusal)

	svcErr := s.provider.DeleteAction(context.Background(), providerRSID, "", providerActionID)

	s.Require().NotNil(svcErr)
	s.Equal("RES-1030", svcErr.Code)
}
