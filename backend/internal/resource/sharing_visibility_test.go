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

// requireVisibleToOU backs reading one server on another organization unit's behalf. The same
// not-found answer covers both ways of not holding it, so a probe cannot tell them apart.
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
			name:    "the caller has no standing over the named unit",
			authz:   stubAuthz{ids: []string{viewOwnerOU}},
			shared:  stubSharing{visible: true},
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
