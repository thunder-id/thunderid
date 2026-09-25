// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	viewOwnerOU  = "owner-ou"
	viewShareeOU = "sharee-ou"
	viewServerID = "rs-1"
)

// stubAuthz reports one fixed answer, which is all these tests vary. The embedded interface
// supplies the methods they never reach.
type stubAuthz struct {
	sysauthz.SystemAuthorizationServiceInterface
	allAllowed bool
	ids        []string
}

func (s stubAuthz) GetAccessibleResources(
	_ context.Context, _ security.Action, _ security.ResourceType,
) (*sysauthz.AccessibleResources, *tidcommon.ServiceError) {
	return &sysauthz.AccessibleResources{AllAllowed: s.allAllowed, IDs: s.ids}, nil
}

// stubSharing answers the two questions the view resolver asks.
type stubSharing struct {
	sharing.ServiceInterface
	visible bool
	rule    sharing.OverlayRule
}

func (s stubSharing) IsVisible(
	_ context.Context, _ sharing.ResourceType, _, _ string,
) (bool, *tidcommon.ServiceError) {
	return s.visible, nil
}

func (s stubSharing) ResolveOverlayRules(
	_ context.Context, _ sharing.ResourceType, _, ouID string,
) (sharing.ResolvedOverlay, *tidcommon.ServiceError) {
	return sharing.ResolvedOverlay{
		OUID:    ouID,
		Rules:   map[string]sharing.OverlayRule{ResourcesFieldKey: s.rule},
		Sources: map[string]string{ResourcesFieldKey: sharing.SourcePolicy},
	}, nil
}

type SharingViewTestSuite struct {
	suite.Suite
	server providers.ResourceServer
}

func TestSharingViewTestSuite(t *testing.T) {
	suite.Run(t, new(SharingViewTestSuite))
}

func (suite *SharingViewTestSuite) SetupTest() {
	suite.server = providers.ResourceServer{ID: viewServerID, OUID: viewOwnerOU, Delimiter: ":"}
}

// serviceFor builds a service with the given standing and sharing terms.
func (suite *SharingViewTestSuite) serviceFor(
	authz sysauthz.SystemAuthorizationServiceInterface, shared sharing.ServiceInterface,
) *resourceService {
	return &resourceService{
		logger:         *log.GetLogger(),
		authzService:   authz,
		sharingService: shared,
	}
}

// A deployment-wide token sees the server's own tree, untouched.
func (suite *SharingViewTestSuite) TestSystemScopeSeesTheOriginalTree() {
	svc := suite.serviceFor(stubAuthz{allAllowed: true}, nil)

	filter, svcErr := svc.resourceServerView(context.Background(), &suite.server)

	suite.Require().Nil(svcErr)
	suite.True(filter.unrestricted, "a deployment-wide caller sees the server as its owner defined it")
}

// The owning organization unit also sees its own tree untouched, without consulting sharing.
func (suite *SharingViewTestSuite) TestOwningOUSeesTheOriginalTree() {
	svc := suite.serviceFor(stubAuthz{ids: []string{viewOwnerOU}}, nil)

	filter, svcErr := svc.resourceServerView(context.Background(), &suite.server)

	suite.Require().Nil(svcErr)
	suite.True(filter.unrestricted)
}

// A sharee sees the tree through the rule its policy left it.
func (suite *SharingViewTestSuite) TestShareeSeesTheSharedSubsetOnly() {
	excluded := []string{"bookings:refund"}
	svc := suite.serviceFor(
		stubAuthz{ids: []string{viewShareeOU}},
		stubSharing{visible: true, rule: sharing.OverlayRule{ExcludedValues: &excluded}},
	)

	filter, svcErr := svc.resourceServerView(context.Background(), &suite.server)

	suite.Require().Nil(svcErr)
	suite.Require().False(filter.unrestricted)
	suite.True(filter.permits("bookings"), "the branch itself is still shared")
	suite.True(filter.permits("bookings:view"), "a sibling of the withheld action stays")
	suite.False(filter.permits("bookings:refund"), "the withheld action is hidden")
	suite.False(filter.permits("bookings:refund:approve"), "and everything beneath it")
}

// An organization unit that was never shared the server cannot see it at all.
func (suite *SharingViewTestSuite) TestUnsharedOUIsNotFound() {
	svc := suite.serviceFor(
		stubAuthz{ids: []string{viewShareeOU}},
		stubSharing{visible: false},
	)

	_, svcErr := svc.resourceServerView(context.Background(), &suite.server)

	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorResourceServerNotFound.Code, svcErr.Code)
}

// The three listing endpoints all pass their results through the same filter, so a withheld branch
// is absent from each of them and the counts stay consistent with what is returned.
func (suite *SharingViewTestSuite) TestListingsDropWhatWasWithheld() {
	excluded := []string{"bookings:refund"}
	svc := suite.serviceFor(
		stubAuthz{ids: []string{viewShareeOU}},
		stubSharing{visible: true, rule: sharing.OverlayRule{ExcludedValues: &excluded}},
	)
	filter, svcErr := svc.treeFilterFor(context.Background(), &suite.server)
	suite.Require().Nil(svcErr)

	suite.Run("resources", func() {
		list := &ResourceList{
			TotalResults: 2, Count: 2,
			Resources: []providers.Resource{
				{Permission: "bookings"},
				{Permission: "bookings:refund"},
			},
		}
		filterResourceList(list, filter)

		require.Len(suite.T(), list.Resources, 1)
		assert.Equal(suite.T(), "bookings", list.Resources[0].Permission)
		assert.Equal(suite.T(), 1, list.TotalResults, "the count matches what is returned")
		assert.Equal(suite.T(), 1, list.Count)
	})

	suite.Run("actions at either level", func() {
		list := &ActionList{
			TotalResults: 3, Count: 3,
			Actions: []providers.Action{
				{Permission: "bookings:view"},
				{Permission: "bookings:refund"},
				{Permission: "billing:view"},
			},
		}
		filterActionList(list, filter)

		require.Len(suite.T(), list.Actions, 2)
		assert.Equal(suite.T(), 2, list.TotalResults)
		for _, a := range list.Actions {
			assert.NotEqual(suite.T(), "bookings:refund", a.Permission)
		}
	})
}

// A deployment-wide caller's listings are returned whole.
func (suite *SharingViewTestSuite) TestSystemScopeListingsAreUntouched() {
	svc := suite.serviceFor(stubAuthz{allAllowed: true}, nil)
	filter, svcErr := svc.treeFilterFor(context.Background(), &suite.server)
	suite.Require().Nil(svcErr)

	list := &ActionList{
		TotalResults: 2, Count: 2,
		Actions: []providers.Action{{Permission: "bookings:view"}, {Permission: "bookings:refund"}},
	}
	filterActionList(list, filter)

	assert.Len(suite.T(), list.Actions, 2, "nothing is withheld from a deployment-wide caller")
	assert.Equal(suite.T(), 2, list.TotalResults)
}

// The sharing framework has its own error vocabulary. It describes a generic policy engine and
// means nothing to a caller of the resource server API, so no SHR code may reach one.
func TestNoSharingErrorCodeEscapesTheResourceAPI(t *testing.T) {
	framework := []tidcommon.ServiceError{
		sharing.ErrorInvalidRequestFormat,
		sharing.ErrorResourceTypeNotRegistered,
		sharing.ErrorPolicyNotFound,
		sharing.ErrorInvalidTargetOU,
		sharing.ErrorNotShared,
		sharing.ErrorCoreConfigOwnerOnly,
		sharing.ErrorCrossTreeShareRestricted,
		sharing.ErrorPolicyDeclared,
		sharing.ErrorRuleWidens,
		sharing.ErrorValueOutsideAllowed,
		sharing.ErrorUnknownFieldKey,
		sharing.ErrorPinnedWithAllowed,
		sharing.ErrorMemberNotVisible,
		sharing.ErrorPolicyExists,
		sharing.ErrorBlanketNarrowOnly,
		sharing.ErrorVersionMismatch,
		sharing.ErrorResourceNotFound,
	}

	for _, in := range framework {
		t.Run(in.Code, func(t *testing.T) {
			out := mapSharingError(&in)

			require.NotNil(t, out)
			assert.NotEqual(t, in.Code, out.Code, "the framework's own code was passed through")
			assert.False(t, strings.HasPrefix(out.Code, "SHR-"),
				"a sharing code reached the resource server API: %s", out.Code)
			assert.True(t, strings.HasPrefix(out.Code, "RES-") || out.Code == tidcommon.InternalServerError.Code,
				"unexpected code %s", out.Code)
		})
	}
}

// Wrapping must not throw away which field, organization unit or policy was at fault.
func TestMappedSharingErrorKeepsTheFrameworksDetail(t *testing.T) {
	in := sharing.ErrorPolicyExists
	in.ErrorDescription.DefaultValue += ": existing policy p-123"

	out := mapSharingError(&in)

	require.NotNil(t, out)
	assert.Equal(t, ErrorSharingPolicyExists.Code, out.Code)
	assert.Contains(t, out.ErrorDescription.DefaultValue, "p-123",
		"the caller still needs to know which policy to edit")
}

// A framework failure that is not the caller's fault is reported as a server error, not as a
// malformed policy.
func TestServerSideSharingFailureIsNotReportedAsAClientError(t *testing.T) {
	out := mapSharingError(&tidcommon.InternalServerError)

	require.NotNil(t, out)
	assert.Equal(t, tidcommon.InternalServerError.Code, out.Code)
	assert.Equal(t, tidcommon.ServerErrorType, out.Type)
}

// Each mapped error has to land on the status code its meaning implies, or a client cannot act on
// it without reading the body.
func TestMappedSharingErrorsCarryTheRightStatus(t *testing.T) {
	tests := []struct {
		in   tidcommon.ServiceError
		want int
	}{
		{sharing.ErrorPolicyNotFound, http.StatusNotFound},
		{sharing.ErrorResourceNotFound, http.StatusNotFound},
		{sharing.ErrorPolicyExists, http.StatusConflict},
		{sharing.ErrorVersionMismatch, http.StatusPreconditionFailed},
		{sharing.ErrorPolicyDeclared, http.StatusForbidden},
		{sharing.ErrorNotShared, http.StatusForbidden},
		{sharing.ErrorCrossTreeShareRestricted, http.StatusForbidden},
		{sharing.ErrorRuleWidens, http.StatusBadRequest},
		{sharing.ErrorUnknownFieldKey, http.StatusBadRequest},
		{sharing.ErrorMemberNotVisible, http.StatusBadRequest},
		{sharing.ErrorBlanketNarrowOnly, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.in.Code, func(t *testing.T) {
			in := tt.in
			w := httptest.NewRecorder()
			handleError(context.Background(), w, mapSharingError(&in))
			assert.Equal(t, tt.want, w.Code)
		})
	}
}

// A bounded listing says how each server is held, because "you own this" and "this was shared with
// you" are different answers a client acts on differently.
func TestOriginsForSaysHowEachServerIsHeld(t *testing.T) {
	servers := []providers.ResourceServer{
		{ID: "rs-own", OUID: "ou-1"},
		{ID: "rs-shared", OUID: "ou-other"},
	}

	origins := originsFor(servers, []string{"ou-1"})

	assert.Equal(t, SharingOriginOwned, origins["rs-own"])
	assert.Equal(t, SharingOriginShared, origins["rs-shared"],
		"a bounded listing returns nothing a policy did not reach, so the rest are shared")
}

// Several organization units may be in scope at once; owning through any of them counts.
func TestOriginsForAcceptsAnyOfTheBoundingOUs(t *testing.T) {
	origins := originsFor(
		[]providers.ResourceServer{{ID: "rs-1", OUID: "ou-2"}},
		[]string{"ou-1", "ou-2"})

	assert.Equal(t, SharingOriginOwned, origins["rs-1"])
}
