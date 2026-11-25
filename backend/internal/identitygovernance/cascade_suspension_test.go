// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"errors"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// dependentsRegistry names the identities suspended together with a user. The other registry methods are not used.
type dependentsRegistry struct {
	resourcedependency.Registry
	dependents []string
	err        error
	asked      *[]string
}

func (r dependentsRegistry) SuspensionDependents(_ context.Context, resourceType, id string) ([]string, error) {
	*r.asked = append(*r.asked, resourceType+"/"+id)
	return r.dependents, r.err
}

// withDependents gives the service a registry naming the dependents, and returns what it was asked for.
func (s *ServiceTestSuite) withDependents(err error, dependents ...string) *[]string {
	asked := []string{}
	s.svc.SetDependencyRegistry(dependentsRegistry{dependents: dependents, err: err, asked: &asked})
	return &asked
}

// expectDependent serves a dependent identity's governed read, which must ask to seed a missing runtime row. held
// says whether it already carries the cascaded hold.
func (s *ServiceTestSuite) expectDependent(id string, held bool) {
	dependent := GovernedEntity{ID: id, Category: providers.EntityCategoryAgent, State: providers.EntityStateActive}
	if held {
		dependent.AccessState.CascadeSuspend = &CascadeSuspendState{SuspendedAt: s.at(-time.Hour)}
	}
	s.port.On("GetGovernedEntity", mock.Anything, id, true).Return(&dependent, nil).Once()
}

// Suspending a user suspends their dependents first, each with the cascaded hold, then the user.
func (s *ServiceTestSuite) TestSuspendingAUserSuspendsTheirDependentsFirst() {
	asked := s.withDependents(nil, "agent-1", "agent-2")
	s.expectRead(s.activeEntity())
	var order []string
	for _, id := range []string{"agent-1", "agent-2"} {
		s.expectDependent(id, false)
		s.port.On("SetCascadeSuspension", mock.Anything, id, s.now).
			Run(func(mock.Arguments) { order = append(order, id) }).Return(true, nil).Once()
	}
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").
		Run(func(mock.Arguments) { order = append(order, "e1") }).Return(true, nil).Once()

	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", ""))

	s.Equal([]string{"user/e1"}, *asked)
	s.Equal([]string{"agent-1", "agent-2", "e1"}, order)
}

// A user already suspended still carries the suspension to their dependents, so a retry completes a cascade.
func (s *ServiceTestSuite) TestSuspendingASuspendedUserStillCascades() {
	s.withDependents(nil, "agent-1")
	s.expectRead(s.suspendedEntity())
	s.expectDependent("agent-1", false)
	s.port.On("SetCascadeSuspension", mock.Anything, "agent-1", s.now).Return(true, nil).Once()

	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", ""))
	s.port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A failure to list or to suspend a dependent fails the operation before the user is suspended.
func (s *ServiceTestSuite) TestAFailedCascadeLeavesTheUserActive() {
	s.withDependents(errors.New("listing failed"))
	s.expectRead(s.activeEntity())
	s.requireServiceError(s.svc.Suspend(s.ctx, "e1", ""), tidcommon.InternalServerError)

	s.SetupTest()
	s.withDependents(nil, "agent-1")
	s.expectRead(s.activeEntity())
	s.expectDependent("agent-1", false)
	s.port.On("SetCascadeSuspension", mock.Anything, "agent-1", s.now).Return(false, errors.New("store down")).Once()
	s.requireServiceError(s.svc.Suspend(s.ctx, "e1", ""), tidcommon.InternalServerError)

	s.SetupTest()
	s.withDependents(nil, "agent-1")
	s.expectRead(s.activeEntity())
	s.port.On("GetGovernedEntity", mock.Anything, "agent-1", true).Return(nil, errors.New("read failed")).Once()
	s.requireServiceError(s.svc.Suspend(s.ctx, "e1", ""), tidcommon.InternalServerError)

	s.port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Unsuspending a user lifts their dependents' cascaded holds first, then the user's own.
func (s *ServiceTestSuite) TestUnsuspendingAUserReleasesTheirDependentsFirst() {
	s.withDependents(nil, "agent-1")
	s.expectRead(s.suspendedEntity())
	s.expectDependent("agent-1", true)
	var order []string
	s.port.On("ClearCascadeSuspension", mock.Anything, "agent-1").
		Run(func(mock.Arguments) { order = append(order, "agent-1") }).Return(true, nil).Once()
	s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, s.at(-time.Hour)).
		Run(func(mock.Arguments) { order = append(order, "e1") }).Return(true, nil).Once()

	s.Require().Nil(s.svc.Unsuspend(s.ctx, "e1"))
	s.Equal([]string{"agent-1", "e1"}, order)
}

// A failed release of a dependent leaves the user suspended, so the operation can be retried.
func (s *ServiceTestSuite) TestAFailedReleaseLeavesTheUserSuspended() {
	s.withDependents(nil, "agent-1")
	s.expectRead(s.suspendedEntity())
	s.expectDependent("agent-1", true)
	s.port.On("ClearCascadeSuspension", mock.Anything, "agent-1").Return(false, errors.New("store down")).Once()

	s.requireServiceError(s.svc.Unsuspend(s.ctx, "e1"), tidcommon.InternalServerError)
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Only a user's suspension by the organization cascades: not an agent's, and not an owner's own suspension.
func (s *ServiceTestSuite) TestOnlyAUsersSuspensionCascades() {
	s.withDependents(nil, "agent-1")
	agent := s.activeEntity()
	agent.Category = providers.EntityCategoryAgent
	s.expectRead(agent)
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").Return(true, nil).Once()
	s.Require().Nil(s.svc.Suspend(s.ctx, "e1", ""))

	s.SetupTest()
	asked := s.withDependents(nil, "agent-1")
	s.expectRead(agent)
	s.port.On("SetOwnerSuspension", mock.Anything, "e1", s.now).Return(true, nil).Once()
	s.Require().Nil(s.svc.Suspend(WithOwnerApplier(s.ctx), "e1", ""))

	s.Empty(*asked)
	s.port.AssertNotCalled(s.T(), "SetCascadeSuspension", mock.Anything, mock.Anything, mock.Anything)
}

// The cascaded hold is lifted only with the user: the organization cannot lift it from the agent on its own.
func (s *ServiceTestSuite) TestTheCascadedHoldIsNotTheOrganizations() {
	agent := s.activeEntity()
	agent.Category = providers.EntityCategoryAgent
	agent.AccessState.CascadeSuspend = &CascadeSuspendState{SuspendedAt: s.at(-time.Hour)}
	s.expectRead(agent)

	s.requireServiceError(s.svc.Unsuspend(s.ctx, "e1"), ErrorNotApplicable)
	s.port.AssertNotCalled(s.T(), "ClearSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The cascaded hold refuses access at every plane, as the other suspensions do.
func (s *ServiceTestSuite) TestTheCascadedHoldHoldsAtEveryPlane() {
	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication, providers.AccessPlaneSession, providers.AccessPlaneIssuance,
		providers.AccessPlaneApplication,
	} {
		s.Run(string(plane), func() {
			s.SetupTest()
			entity := s.activeEntity()
			entity.AccessState.CascadeSuspend = &CascadeSuspendState{SuspendedAt: s.at(-time.Hour)}
			s.expectRead(entity)
			s.expectConfig(methodGranularityConfig())

			decision, err := s.svc.EvaluateAccess(s.ctx, "e1", plane, testMethod)
			s.Require().Nil(err)
			s.False(decision.Admitted)
			s.Equal(providers.AccessRefusalReasonSuspended, decision.Reason)
		})
	}
}

// A dependent that already carries the hold as asked, or no longer exists, is left as it is and not audited.
func (s *ServiceTestSuite) TestTheCascadeLeavesSettledDependentsAlone() {
	svc, published := s.lockAuditedService(methodGranularityConfig())
	svc.SetDependencyRegistry(dependentsRegistry{dependents: []string{"agent-1", "gone"}, asked: &[]string{}})
	s.expectRead(s.activeEntity())
	s.expectDependent("agent-1", true)
	s.port.On("GetGovernedEntity", mock.Anything, "gone", true).Return(nil, nil).Once()
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").Return(true, nil).Once()

	s.Require().Nil(svc.Suspend(s.ctx, "e1", ""))

	s.port.AssertNotCalled(s.T(), "SetCascadeSuspension", mock.Anything, mock.Anything, mock.Anything)
	s.Require().Len(*published, 1, "only the user's own suspension is published")
	s.Equal("e1", (*published)[0].Data[event.DataKey.Subject])
}

// Each dependent's cascaded hold is published as that identity's own suspension or unsuspension, applied by the
// cascade, with the operator who acted on the user as the actor.
func (s *ServiceTestSuite) TestEachCascadedHoldIsAudited() {
	svc, published := s.lockAuditedService(methodGranularityConfig())
	svc.SetDependencyRegistry(dependentsRegistry{dependents: []string{"agent-1"}, asked: &[]string{}})
	ctx := security.WithSecurityContextTest(s.ctx,
		security.NewSecurityContextForTest("admin-1", "", "token", []string{"system"}, nil))
	s.expectRead(s.activeEntity())
	s.expectDependent("agent-1", false)
	s.port.On("SetCascadeSuspension", mock.Anything, "agent-1", s.now).Return(true, nil).Once()
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").Return(true, nil).Once()

	s.Require().Nil(svc.Suspend(ctx, "e1", ""))

	s.Require().Len(*published, 2)
	cascaded := (*published)[0]
	s.Equal(string(event.EventTypeIdentitySuspended), cascaded.Type)
	s.Equal(providers.StatusSuccess, cascaded.Status)
	s.Equal("agent-1", cascaded.Data[event.DataKey.Subject])
	s.Equal(event.PrincipalTypeAgent, cascaded.Data[event.DataKey.SubjectType])
	s.Equal(string(providers.SuspensionAppliedByCascade), cascaded.Data[event.DataKey.AppliedBy])
	s.Equal("admin-1", cascaded.Data[event.DataKey.ActorSub])
	s.Equal("e1", (*published)[1].Data[event.DataKey.Subject])

	*published = (*published)[:0]
	s.SetupTest()
	svc.entityState = s.port
	s.expectRead(s.suspendedEntity())
	s.expectDependent("agent-1", true)
	s.port.On("ClearCascadeSuspension", mock.Anything, "agent-1").Return(false, errors.New("store down")).Once()

	s.requireServiceError(svc.Unsuspend(ctx, "e1"), tidcommon.InternalServerError)

	s.Require().Len(*published, 2, "the failed release and the user's failed unsuspension are both published")
	s.Equal(string(event.EventTypeIdentityUnsuspensionFailed), (*published)[0].Type)
	s.Equal("agent-1", (*published)[0].Data[event.DataKey.Subject])
	s.Equal(string(providers.SuspensionAppliedByCascade), (*published)[0].Data[event.DataKey.AppliedBy])
}
