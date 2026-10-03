// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package inboundclient

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/sharing"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/certmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

const (
	admittedClientID = "m2m-client"
	admittedAppID    = "m2m-app"
	ownerOUID        = "m2m-root"
	otherOUID        = "m2m-child-a"
	appSharingType   = sharing.ResourceType("application")
)

// AccessingOUTestSuite covers the rule that decides which organization units a client may be used
// on behalf of. It lives in resolution, so every path that resolves a client inherits it.
type AccessingOUTestSuite struct {
	suite.Suite
	sharingSvc *sharingmock.SharingServiceInterfaceMock
}

func TestAccessingOUTestSuite(t *testing.T) {
	suite.Run(t, new(AccessingOUTestSuite))
}

func (s *AccessingOUTestSuite) SetupTest() {
	s.sharingSvc = sharingmock.NewSharingServiceInterfaceMock(s.T())
}

// check runs the rule as resolution would, for a client owned by ownerOUID.
func (s *AccessingOUTestSuite) check(
	ctx context.Context, sharingSvc sharing.SharingServiceInterface, category providers.EntityCategory,
) error {
	svc := &inboundClientService{
		sharingService: sharingSvc,
		sharedTypes:    map[providers.EntityCategory]sharing.ResourceType{providers.EntityCategoryApp: appSharingType},
		logger:         log.GetLogger(),
	}
	return svc.requireAccessibleFromAccessingOU(ctx, &providers.OAuthClient{
		ID: admittedAppID, ClientID: admittedClientID, OUID: ownerOUID, EntityCategory: category,
	})
}

// A request naming no organization unit is the ordinary one, and the rule stays out of its way
// entirely: nothing is asked of the sharing framework.
func (s *AccessingOUTestSuite) TestNoAccessingOULeavesResolutionUntouched() {
	s.Require().NoError(s.check(context.Background(), s.sharingSvc, providers.EntityCategoryApp))
	s.sharingSvc.AssertNotCalled(s.T(), "IsVisible", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A client's own organization unit needs no policy. The framework would answer the same, but only
// after resolving ownership, and this is the common case on the token path.
func (s *AccessingOUTestSuite) TestOwnOrganizationUnitNeedsNoPolicy() {
	ctx := syscontext.WithAccessingOUID(context.Background(), ownerOUID)

	s.Require().NoError(s.check(ctx, s.sharingSvc, providers.EntityCategoryApp))
	s.sharingSvc.AssertNotCalled(s.T(), "IsVisible", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A runtime context must NOT skip the check. The token endpoint is a public path, and the security
// layer marks every public request as an internal runtime caller so the authorization layer lets it
// through; treating that as "trusted, skip" would disable admission on exactly the requests it
// exists for, admitting every organization unit that merely exists.
func (s *AccessingOUTestSuite) TestARuntimeContextStillGetsAccessChecked() {
	ctx := security.WithRuntimeContext(syscontext.WithAccessingOUID(context.Background(), otherOUID))
	s.sharingSvc.EXPECT().IsVisible(mock.Anything, appSharingType, admittedAppID, otherOUID).
		Return(false, nil).Once()

	s.ErrorIs(s.check(ctx, s.sharingSvc, providers.EntityCategoryApp), ErrInboundClientNotAccessibleFromOU)
}

// Another organization unit resolves exactly when a sharing policy reached it.
func (s *AccessingOUTestSuite) TestReachedOrganizationUnitResolves() {
	ctx := syscontext.WithAccessingOUID(context.Background(), otherOUID)
	s.sharingSvc.EXPECT().IsVisible(mock.Anything, appSharingType, admittedAppID, otherOUID).
		Return(true, nil).Once()

	s.Require().NoError(s.check(ctx, s.sharingSvc, providers.EntityCategoryApp))
}

// An organization unit no policy reached does not resolve the client at all. Refusing in resolution
// rather than at a grant handler is what keeps one registration usable everywhere it is allowed and
// nowhere else.
func (s *AccessingOUTestSuite) TestUnreachedOrganizationUnitIsRefused() {
	ctx := syscontext.WithAccessingOUID(context.Background(), otherOUID)
	s.sharingSvc.EXPECT().IsVisible(mock.Anything, appSharingType, admittedAppID, otherOUID).
		Return(false, nil).Once()

	s.ErrorIs(s.check(ctx, s.sharingSvc, providers.EntityCategoryApp), ErrInboundClientNotAccessibleFromOU)
}

// Outside the server there is no sharing framework, so nothing can say a client was shared. That
// refuses every organization unit but the client's own, rather than admitting on the strength of a
// check that cannot run.
func (s *AccessingOUTestSuite) TestMissingFrameworkRefusesEveryOtherOrganizationUnit() {
	ctx := syscontext.WithAccessingOUID(context.Background(), otherOUID)

	s.ErrorIs(s.check(ctx, nil, providers.EntityCategoryApp), ErrInboundClientNotAccessibleFromOU)
}

// A kind of client no resource type is registered for cannot be shared, so it is usable in its own
// organization unit alone. Agents become shareable by gaining an entry, not by changing this.
func (s *AccessingOUTestSuite) TestAnUnshareableKindIsRefusedElsewhere() {
	ctx := syscontext.WithAccessingOUID(context.Background(), otherOUID)

	s.ErrorIs(s.check(ctx, s.sharingSvc, providers.EntityCategoryAgent), ErrInboundClientNotAccessibleFromOU)
	s.sharingSvc.AssertNotCalled(s.T(), "IsVisible", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failed lookup is not an answer. Reporting it as "not shared" would turn a broken dependency
// into a refusal the caller would spend time trying to fix in its own configuration.
func (s *AccessingOUTestSuite) TestLookupFailureIsNotARefusal() {
	ctx := syscontext.WithAccessingOUID(context.Background(), otherOUID)
	s.sharingSvc.EXPECT().IsVisible(mock.Anything, appSharingType, admittedAppID, otherOUID).
		Return(false, &tidcommon.InternalServerError).Once()

	err := s.check(ctx, s.sharingSvc, providers.EntityCategoryApp)

	s.Require().Error(err)
	s.False(errors.Is(err, ErrInboundClientNotAccessibleFromOU),
		"a broken dependency must not read as a policy decision")
}

// resolve runs a full client resolution, so what is asserted is that the rule is reached from
// GetOAuthClientByClientID rather than merely that it would answer correctly if called.
func (s *AccessingOUTestSuite) resolve(ctx context.Context) (*providers.OAuthClient, error) {
	entityID := admittedAppID
	ep := entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	ep.EXPECT().IdentifyEntity(mock.Anything).Return(&entityID, nil)
	ep.EXPECT().GetEntity(entityID).Return(&providers.Entity{
		ID: entityID, OUID: ownerOUID, Category: providers.EntityCategoryApp,
	}, nil)

	store := newInboundClientStoreInterfaceMock(s.T())
	store.EXPECT().GetOAuthProfileByEntityID(mock.Anything, entityID).
		Return(&providers.OAuthProfile{}, nil)

	// The client carries no certificate; the resolution path asks for one regardless.
	certSvc := certmock.NewCertificateServiceInterfaceMock(s.T())
	certSvc.EXPECT().GetCertificateByReference(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &cert.ErrorCertificateNotFound).Maybe()

	svc := &inboundClientService{
		entityProvider: ep,
		store:          store,
		certService:    certSvc,
		sharingService: s.sharingSvc,
		sharedTypes: map[providers.EntityCategory]sharing.ResourceType{
			providers.EntityCategoryApp: appSharingType,
		},
		logger: log.GetLogger(),
	}
	return svc.GetOAuthClientByClientID(ctx, admittedClientID)
}

// Resolution itself enforces the rule, which is what makes every caller inherit it: a client the
// request may not act as does not resolve at all.
func (s *AccessingOUTestSuite) TestResolutionRefusesAnUnreachedOrganizationUnit() {
	// A runtime context, because the token endpoint is public and the security layer marks every
	// public request as one. This is the shape the production call actually has.
	ctx := security.WithRuntimeContext(syscontext.WithAccessingOUID(context.Background(), otherOUID))
	s.sharingSvc.EXPECT().IsVisible(mock.Anything, appSharingType, admittedAppID, otherOUID).
		Return(false, nil).Once()

	client, err := s.resolve(ctx)

	s.ErrorIs(err, ErrInboundClientNotAccessibleFromOU)
	s.Nil(client, "no client is handed back for a unit that may not act as it")
}

// And a reached one resolves as it always did.
func (s *AccessingOUTestSuite) TestResolutionAdmitsAReachedOrganizationUnit() {
	ctx := security.WithRuntimeContext(syscontext.WithAccessingOUID(context.Background(), otherOUID))
	s.sharingSvc.EXPECT().IsVisible(mock.Anything, appSharingType, admittedAppID, otherOUID).
		Return(true, nil).Once()

	client, err := s.resolve(ctx)

	s.Require().NoError(err)
	s.Require().NotNil(client)
	s.Equal(admittedClientID, client.ClientID)
}
