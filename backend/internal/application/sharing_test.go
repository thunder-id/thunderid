// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
)

const (
	testAppID    = "m2m-app"
	testAppName  = "Billing Sync"
	testAppOwner = "m2m-root"
	testPolicyID = "m2m-app-share"
)

// ApplicationSharingTestSuite covers how an application participates in sharing: what it declares to
// the framework, and how the policies its file declares are validated and replayed.
type ApplicationSharingTestSuite struct {
	suite.Suite
}

func TestApplicationSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationSharingTestSuite))
}

// An application declares no field, which is what makes "a sharee may not edit anything" true by
// construction rather than by a rule somebody has to remember to write.
func (s *ApplicationSharingTestSuite) TestNoFieldsAreDeclared() {
	decl := newApplicationSharingDeclaration(applicationmock.NewApplicationServiceInterfaceMock(s.T()))

	s.Equal(ApplicationSharingType, decl.ResourceType())
	s.Empty(decl.Fields(), "an application shares no per-organization-unit state")
}

// Ownership is read through the runtime context. The framework asks in order to decide access, so an
// access-checked read here would recurse back through the framework.
func (s *ApplicationSharingTestSuite) TestOwnerIsResolvedWithoutAnAccessCheck() {
	appService := applicationmock.NewApplicationServiceInterfaceMock(s.T())
	var sawRuntime bool
	appService.EXPECT().GetApplication(mock.Anything, testAppID).
		RunAndReturn(func(ctx context.Context, _ string) (*providers.Application, *tidcommon.ServiceError) {
			sawRuntime = security.IsRuntimeContext(ctx)
			return &providers.Application{ID: testAppID, OUID: testAppOwner}, nil
		}).Once()

	owner, svcErr := newApplicationSharingDeclaration(appService).OwningOUID(context.Background(), testAppID)

	s.Require().Nil(svcErr)
	s.Equal(testAppOwner, owner)
	s.True(sawRuntime, "the ownership read must not be access-checked")
}

// A failure to read the application is reported rather than answered as "no owner", which the
// framework would read as a resource it does not recognize.
func (s *ApplicationSharingTestSuite) TestOwnerResolutionCarriesTheServiceError() {
	appService := applicationmock.NewApplicationServiceInterfaceMock(s.T())
	appService.EXPECT().GetApplication(mock.Anything, testAppID).
		Return(nil, &tidcommon.InternalServerError).Once()

	owner, svcErr := newApplicationSharingDeclaration(appService).OwningOUID(context.Background(), testAppID)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Empty(owner)
}

// The policies have to survive the parse, because the declarative path is the only way in: there is
// no API that could set them afterwards.
func (s *ApplicationSharingTestSuite) TestDeclaredPoliciesSurviveYAMLParsing() {
	doc := []byte(`
id: ` + testAppID + `
name: ` + testAppName + `
ouId: ` + testAppOwner + `
sharingPolicies:
  - id: ` + testPolicyID + `
    targets:
      - scope: allOus
        excludedOuIds:
          - m2m-child-b
`)

	var request model.ApplicationRequestWithID
	s.Require().NoError(yaml.Unmarshal(doc, &request))

	s.Require().Len(request.SharingPolicies, 1)
	policy := request.SharingPolicies[0]
	s.Equal(testPolicyID, policy.ID)
	s.Require().Len(policy.Targets, 1)
	s.Equal(sharing.ScopeAllOUs, policy.Targets[0].Scope)
	s.Equal([]string{"m2m-child-b"}, policy.Targets[0].ExcludedOUIDs)
}
