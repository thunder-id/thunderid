// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	sharingAppID   = "app-1"
	sharingOwnerOU = "owner-ou"
)

// ownerStub answers the one read the declaration makes, and records whether it was made as an
// internal runtime caller.
type ownerStub struct {
	ApplicationServiceInterface
	ouID    string
	runtime bool
	svcErr  *tidcommon.ServiceError
}

func (s *ownerStub) GetApplication(
	ctx context.Context, _ string,
) (*providers.Application, *tidcommon.ServiceError) {
	s.runtime = security.IsRuntimeContext(ctx)
	if s.svcErr != nil {
		return nil, s.svcErr
	}
	return &providers.Application{ID: sharingAppID, OUID: s.ouID}, nil
}

// seedRecorder captures what the startup replay asks the framework for.
type seedRecorder struct {
	sharing.ServiceInterface
	calls  []sharing.PolicyRequest
	svcErr *tidcommon.ServiceError
}

func (r *seedRecorder) CreateDeclarativePolicy(
	_ context.Context, _ sharing.ResourceType, _, _ string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	r.calls = append(r.calls, req)
	return sharing.Policy{}, r.svcErr
}

type ApplicationSharingTestSuite struct {
	suite.Suite
}

func TestApplicationSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationSharingTestSuite))
}

// An application shares no field, which is what makes "a sharee may not edit anything" true by
// construction rather than by a rule somebody has to remember to write.
func (suite *ApplicationSharingTestSuite) TestNoFieldsAreDeclared() {
	decl := newApplicationSharing(nil)

	suite.Empty(decl.Fields())
	suite.Equal(ApplicationSharingType, decl.ResourceType())
}

func (suite *ApplicationSharingTestSuite) TestOwnerIsResolvedWithoutAnAccessCheck() {
	stub := &ownerStub{ouID: sharingOwnerOU}
	decl := newApplicationSharing(stub)

	ouID, svcErr := decl.OwningOUID(context.Background(), sharingAppID)

	suite.Require().Nil(svcErr)
	suite.Equal(sharingOwnerOU, ouID)
	suite.True(stub.runtime,
		"the framework asks this while deciding access, so the read must not itself be access-checked")
}

func (suite *ApplicationSharingTestSuite) TestOwnerResolutionCarriesTheServiceError() {
	decl := newApplicationSharing(&ownerStub{svcErr: &ErrorApplicationNotFound})

	_, svcErr := decl.OwningOUID(context.Background(), sharingAppID)

	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorApplicationNotFound.Code, svcErr.Code)
}

// The scopes an application may express. Naming units one hop at a time is the per-customer
// enumeration the feature exists to avoid, so it is refused rather than quietly accepted.
func (suite *ApplicationSharingTestSuite) TestOnlyBlanketScopesAreAccepted() {
	tests := []struct {
		name    string
		policy  providers.SharingPolicy
		wantErr string
	}{
		{
			name:   "every organization unit",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{AllOUs: true}},
		},
		{
			name:   "the owner's whole subtree",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{AllChildren: true}},
		},
		{
			name: "a blanket scope with a carve-out",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{
				AllOUs:        true,
				ExcludedOUIDs: []string{"child-b"},
			}},
		},
		{
			name: "every root",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{
				AllRoots: true,
			}},
			wantErr: "allRoots",
		},
		{
			name: "named roots",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{
				RootOUIDs: []string{"root-a"},
			}},
			wantErr: "rootOuIds",
		},
		{
			name: "root carve-outs",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{
				AllOUs:            true,
				ExcludedRootOUIDs: []string{"root-a"},
			}},
			wantErr: "excludedRootOuIds",
		},
		{
			name: "named children",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{
				OUIDs: []providers.SharingTargetOUEntry{{OUID: "child-a"}},
			}},
			wantErr: "ouIds",
		},
		{
			name:    "no scope at all",
			policy:  providers.SharingPolicy{},
			wantErr: "exactly one of allOus or allChildren",
		},
		{
			name: "both blanket scopes",
			policy: providers.SharingPolicy{TargetOuScope: providers.SharingTargetOUScope{
				AllOUs: true, AllChildren: true,
			}},
			wantErr: "exactly one of allOus or allChildren",
		},
		{
			name: "an overlay rule",
			policy: providers.SharingPolicy{
				TargetOuScope: providers.SharingTargetOUScope{AllOUs: true},
				OverlayRules:  map[string]providers.OverlayRule{"config": {Editable: true}},
			},
			wantErr: "overlayRules",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			err := validateApplicationPolicy(tt.policy)

			if tt.wantErr == "" {
				suite.NoError(err)
				return
			}
			suite.Require().Error(err)
			suite.Contains(err.Error(), tt.wantErr,
				"the refusal must name the field the author actually wrote")
		})
	}
}

func (suite *ApplicationSharingTestSuite) TestSeedingReplaysEveryDeclaredPolicy() {
	recorder := &seedRecorder{}
	declared := []declaredAppPolicies{{
		appID: sharingAppID, appName: "Reconciler", ownerOU: sharingOwnerOU,
		policies: []providers.SharingPolicy{
			{TargetOuScope: providers.SharingTargetOUScope{AllOUs: true, ExcludedOUIDs: []string{"child-b"}}},
			{TargetOuScope: providers.SharingTargetOUScope{AllChildren: true}},
		},
	}}

	suite.Require().NoError(seedDeclaredApplicationPolicies(declared, recorder))

	suite.Require().Len(recorder.calls, 2)
	suite.True(recorder.calls[0].TargetOUScope.AllOUs)
	suite.Equal([]string{"child-b"}, recorder.calls[0].TargetOUScope.ExcludedOUIDs)
	suite.True(recorder.calls[1].TargetOUScope.AllChildren)
}

// A startup failure has to say which document is wrong. An organization unit that does not exist is
// the most common way to get this wrong, and the id alone sends the operator through every file.
func (suite *ApplicationSharingTestSuite) TestSeedingFailureNamesTheApplicationAndPolicy() {
	refusal := tidcommon.ServiceError{
		Code:             "SHR-1019",
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "the organization unit does not exist"},
	}
	recorder := &seedRecorder{svcErr: &refusal}
	declared := []declaredAppPolicies{{
		appID: sharingAppID, appName: "Reconciler", ownerOU: sharingOwnerOU,
		policies: []providers.SharingPolicy{
			{TargetOuScope: providers.SharingTargetOUScope{AllOUs: true}},
		},
	}}

	err := seedDeclaredApplicationPolicies(declared, recorder)

	suite.Require().Error(err)
	for _, want := range []string{"Reconciler", sharingOwnerOU, "SHR-1019", "does not exist"} {
		suite.Contains(err.Error(), want)
	}
}

// Declaring policies with no framework to record them in is a misconfiguration, not a silent no-op.
func (suite *ApplicationSharingTestSuite) TestSeedingRefusesWhenSharingIsNotEnabled() {
	declared := []declaredAppPolicies{{
		appName:  "Reconciler",
		policies: []providers.SharingPolicy{{TargetOuScope: providers.SharingTargetOUScope{AllOUs: true}}},
	}}

	err := seedDeclaredApplicationPolicies(declared, nil)

	suite.Require().Error(err)
	suite.True(strings.Contains(err.Error(), "Reconciler"))
}

func (suite *ApplicationSharingTestSuite) TestSeedingNothingIsNotAFailure() {
	suite.NoError(seedDeclaredApplicationPolicies(nil, nil))
}

// The declarative parser copies fields one by one and silently drops anything it does not name, so
// the shape is checked against the real fixture rather than a hand-built struct.
func TestDeclaredPoliciesSurviveYAMLParsing(t *testing.T) {
	const doc = `
resource_type: application
id: decl-m2m-carved-out
name: M2M Service (Carved Out)
ouId: decl-m2m-root
type: fullstack
template: web
url: https://example.com
sharingPolicies:
  - targetOuScope:
      allChildren: true
      excludedOuIds:
        - decl-m2m-child-b
inboundAuthConfig:
  - type: "oauth2"
    config:
      clientId: "decl-m2m-carved-out-client"
      clientSecret: "decl-m2m-carved-out-secret"
      grantTypes:
        - "client_credentials"
`

	dto, err := parseToApplicationDTO([]byte(doc))

	require.NoError(t, err)
	require.Len(t, dto.SharingPolicies, 1)
	policy := dto.SharingPolicies[0]
	assert.True(t, policy.TargetOuScope.AllChildren)
	assert.Equal(t, []string{"decl-m2m-child-b"}, policy.TargetOuScope.ExcludedOUIDs)
	assert.False(t, policy.TargetOuScope.AllOUs)
	assert.NoError(t, validateApplicationPolicy(policy), "the fixture must be a policy we accept")
}

// An application with no sharing block must stay exactly as it was.
func TestAnApplicationWithoutPoliciesParsesUnchanged(t *testing.T) {
	dto, err := parseToApplicationDTO([]byte("resource_type: application\nid: decl-app\nname: Plain\n"))

	require.NoError(t, err)
	assert.Empty(t, dto.SharingPolicies)
}
