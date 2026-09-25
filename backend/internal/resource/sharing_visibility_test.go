// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// overlayStub answers the resolver the overlay endpoint calls, so a test can vary the one thing
// that decides the response: whether the organization unit was reached at all.
type overlayStub struct {
	sharing.ServiceInterface
	resolved sharing.ResolvedOverlay
}

func (s overlayStub) ResolveOverlayRules(
	_ context.Context, _ sharing.ResourceType, _, ouID string,
) (sharing.ResolvedOverlay, *tidcommon.ServiceError) {
	out := s.resolved
	out.OUID = ouID
	return out, nil
}

// fixedServerStore resolves one resource server and nothing else, which is all the permission
// filter reads.
type fixedServerStore struct {
	resourceStoreInterface
	server providers.ResourceServer
}

func (s fixedServerStore) GetResourceServer(
	_ context.Context, _ string,
) (providers.ResourceServer, error) {
	return s.server, nil
}

// Every permission these tests name exists on the server, so the only thing that can make one
// invalid is the organization unit it is being asked about.
func (s fixedServerStore) ValidatePermissions(
	_ context.Context, _ string, _ []string,
) ([]string, error) {
	return []string{}, nil
}

// ownerOnlyService is the resource service the overlay handler needs: it resolves the server so the
// handler can confirm it exists, and nothing else.
type ownerOnlyService struct {
	ResourceServiceInterface
	server providers.ResourceServer
}

func (s ownerOnlyService) GetResourceServer(
	_ context.Context, _ string,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	return &s.server, nil
}

type SharingVisibilityTestSuite struct {
	suite.Suite
	server providers.ResourceServer
}

func TestSharingVisibilityTestSuite(t *testing.T) {
	suite.Run(t, new(SharingVisibilityTestSuite))
}

func (suite *SharingVisibilityTestSuite) SetupTest() {
	suite.server = providers.ResourceServer{ID: viewServerID, OUID: viewOwnerOU, Delimiter: ":"}
}

func (suite *SharingVisibilityTestSuite) overlayRequest(shared sharing.ServiceInterface) *httptest.ResponseRecorder {
	handler := newSharingHandler(ownerOnlyService{server: suite.server}, shared)

	req := httptest.NewRequest(http.MethodGet,
		"/resource-servers/"+viewServerID+"/overlay-rules?ouId="+viewShareeOU, nil)
	req.SetPathValue("id", viewServerID)
	w := httptest.NewRecorder()
	handler.HandleSharingOverlayGetRequest(w, req)
	return w
}

// The case this exists for: a resource server withheld from an organization unit must not be
// described to it. An empty rule set is still an answer about a server it was never told about.
func (suite *SharingVisibilityTestSuite) TestUnreachedOUIsToldTheServerDoesNotExist() {
	w := suite.overlayRequest(overlayStub{
		resolved: sharing.ResolvedOverlay{Visible: false},
	})

	suite.Equal(http.StatusNotFound, w.Code)
	var body map[string]any
	require.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal(ErrorResourceServerNotFound.Code, body["code"])
}

func (suite *SharingVisibilityTestSuite) TestReachedOUGetsItsRules() {
	w := suite.overlayRequest(overlayStub{
		resolved: sharing.ResolvedOverlay{
			Visible: true,
			Rules:   map[string]sharing.OverlayRule{ResourcesFieldKey: {Editable: false}},
			Sources: map[string]string{ResourcesFieldKey: sharing.SourcePolicy},
		},
	})

	suite.Require().Equal(http.StatusOK, w.Code)
	var body SharingOverlayResponse
	require.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal(SharingOriginShared, body.Origin)
	suite.Contains(body.Rules, ResourcesFieldKey)
}

func (suite *SharingVisibilityTestSuite) TestOwnerGetsItsOwnTerms() {
	w := suite.overlayRequest(overlayStub{
		resolved: sharing.ResolvedOverlay{Visible: true, Owned: true},
	})

	suite.Require().Equal(http.StatusOK, w.Code)
	var body SharingOverlayResponse
	require.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &body))
	suite.Equal(SharingOriginOwned, body.Origin)
}

// resolveViewingOU settles which organization unit a read is answered as, and is the half that
// bounds the caller. It narrows and never widens, so naming a unit the caller has no standing over
// is refused rather than answered.
func (suite *SharingVisibilityTestSuite) TestResolvingTheViewingOU() {
	tests := []struct {
		name      string
		authz     sysauthz.SystemAuthorizationServiceInterface
		requested string
		want      string
		wantCode  string
	}{
		{
			name:      "a unit the caller stands over is answered as itself",
			authz:     stubAuthz{ids: []string{viewShareeOU}},
			requested: viewShareeOU,
			want:      viewShareeOU,
		},
		{
			name:      "a deployment-wide caller may ask about any unit",
			authz:     stubAuthz{allAllowed: true},
			requested: viewShareeOU,
			want:      viewShareeOU,
		},
		{
			name:      "a unit the caller has no standing over is refused",
			authz:     stubAuthz{ids: []string{viewOwnerOU}},
			requested: viewShareeOU,
			wantCode:  ErrorResourceServerNotFound.Code,
		},
		{
			// A token carrying one organization unit is answered as that one, so a tenant never has
			// to name itself.
			name:  "an omitted unit falls back to the caller's own",
			authz: stubAuthz{ids: []string{viewShareeOU}},
			want:  viewShareeOU,
		},
		{
			// A deployment-wide caller is bounded by no unit, so there is no single one its answer
			// could be about.
			name:     "a deployment-wide caller must name one",
			authz:    stubAuthz{allAllowed: true},
			wantCode: ErrorViewingOUIDRequired.Code,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			svc := &resourceService{logger: *log.GetLogger(), authzService: tt.authz}

			got, svcErr := svc.resolveViewingOU(context.Background(), tt.requested)

			if tt.wantCode != "" {
				suite.Require().NotNil(svcErr)
				suite.Equal(tt.wantCode, svcErr.Code)
				return
			}
			suite.Require().Nil(svcErr)
			suite.Equal(tt.want, got)
		})
	}
}

// requireVisibleToOU is the other half: whether that organization unit holds the server at all. The
// same not-found answer covers both ways of not holding it, so a probe cannot tell them apart.
func (suite *SharingVisibilityTestSuite) TestReadingOnBehalfOfAnOU() {
	tests := []struct {
		name    string
		authz   sysauthz.SystemAuthorizationServiceInterface
		shared  sharing.ServiceInterface
		ouID    string
		wantErr bool
	}{
		{
			name:   "the organization unit owns it",
			authz:  stubAuthz{allAllowed: true},
			shared: stubSharing{visible: false},
			ouID:   viewOwnerOU,
		},
		{
			name:   "a policy reached it",
			authz:  stubAuthz{allAllowed: true},
			shared: stubSharing{visible: true},
			ouID:   viewShareeOU,
		},
		{
			name:    "nothing reached it",
			authz:   stubAuthz{allAllowed: true},
			shared:  stubSharing{visible: false},
			ouID:    viewShareeOU,
			wantErr: true,
		},
		{
			name:    "sharing is not wired and the unit is not the owner",
			authz:   stubAuthz{allAllowed: true},
			shared:  nil,
			ouID:    viewShareeOU,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			svc := &resourceService{
				logger:         *log.GetLogger(),
				authzService:   tt.authz,
				sharingService: tt.shared,
			}

			svcErr := svc.requireVisibleToOU(context.Background(), &suite.server, tt.ouID)

			if tt.wantErr {
				suite.Require().NotNil(svcErr)
				suite.Equal(ErrorResourceServerNotFound.Code, svcErr.Code)
				return
			}
			suite.Nil(svcErr)
		})
	}
}

// ValidatePermissions answers for the organization unit a token is for, which has no standing of
// its own to resolve. A permission the server defines but that unit cannot see is invalid for it,
// exactly as one the server never defined is: the caller has one list to act on, not two.
func (suite *SharingVisibilityTestSuite) TestPermissionsHiddenFromAnOUAreInvalid() {
	withheld := []string{"bookings:refund"}

	tests := []struct {
		name        string
		shared      sharing.ServiceInterface
		ouID        string
		wantInvalid []string
	}{
		{
			name:        "the owner sees everything its server defines",
			shared:      stubSharing{visible: false},
			ouID:        viewOwnerOU,
			wantInvalid: []string{},
		},
		{
			name:        "an organization unit nothing reached sees none of it",
			shared:      stubSharing{visible: false},
			ouID:        viewShareeOU,
			wantInvalid: []string{"bookings", "bookings:view", "bookings:refund"},
		},
		{
			name:        "a sharee sees what its policy left",
			shared:      stubSharing{visible: true, rule: sharing.OverlayRule{ExcludedValues: &withheld}},
			ouID:        viewShareeOU,
			wantInvalid: []string{"bookings:refund"},
		},
		{
			name:        "without the framework a non-owner sees nothing",
			shared:      nil,
			ouID:        viewShareeOU,
			wantInvalid: []string{"bookings", "bookings:view", "bookings:refund"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			svc := &resourceService{
				logger:         *log.GetLogger(),
				sharingService: tt.shared,
				resourceStore:  fixedServerStore{server: suite.server},
			}

			invalid, svcErr := svc.ValidatePermissions(context.Background(), viewServerID,
				[]string{"bookings", "bookings:view", "bookings:refund"}, tt.ouID)

			suite.Require().Nil(svcErr)
			suite.Equal(tt.wantInvalid, invalid)
		})
	}
}

// Naming no organization unit asks only whether the server defines these paths, which is what every
// caller but the token path wants and what the bare token endpoint keeps doing.
func (suite *SharingVisibilityTestSuite) TestNoOUAsksOnlyWhetherTheServerDefinesThem() {
	svc := &resourceService{
		logger:         *log.GetLogger(),
		sharingService: stubSharing{visible: false},
		resourceStore:  fixedServerStore{server: suite.server},
	}

	invalid, svcErr := svc.ValidatePermissions(context.Background(), viewServerID,
		[]string{"bookings", "bookings:view"}, "")

	suite.Require().Nil(svcErr)
	suite.Empty(invalid, "sharing must not be consulted when no organization unit is named")
}

// The deployment root permission belongs to no organization unit and no resource server defines it,
// so the tree filter has no opinion on it and must not drop it.
func (suite *SharingVisibilityTestSuite) TestRootPermissionSurvivesTheFilter() {
	withheld := []string{"bookings"}
	svc := &resourceService{
		logger:         *log.GetLogger(),
		sharingService: stubSharing{visible: true, rule: sharing.OverlayRule{ExcludedValues: &withheld}},
		resourceStore:  fixedServerStore{server: suite.server},
	}
	security.InitSystemPermissions("system")
	root := security.GetSystemRootPermission()

	invalid, svcErr := svc.ValidatePermissions(context.Background(), viewServerID,
		[]string{root, "bookings:view"}, viewShareeOU)

	suite.Require().Nil(svcErr)
	suite.Equal([]string{"bookings:view"}, invalid, "only the withheld branch is invalid")
}
