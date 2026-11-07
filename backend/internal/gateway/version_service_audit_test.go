// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const testAuditAdminID = "admin-1"

// publishedEvent is one audit event the version service published.
type publishedEvent struct {
	name    string
	payload audit.Payload
}

// recordingSink keeps every audit event it is handed, in order.
type recordingSink struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (r *recordingSink) Handle(_ context.Context, env eventbus.Envelope, payload audit.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, publishedEvent{name: env.Name, payload: payload})
	return nil
}

// newRecordingBus returns an event bus with a sink subscribed the way the binaries subscribe theirs, so
// an event audit refuses never reaches it, and a function that stops the bus and returns the audit
// events written in order.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "gateway-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus is an event bus that drops every event, for a test the audit event does not matter to.
// A test of what the service publishes uses newRecordingBus.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

type VersionServiceAuditTestSuite struct {
	suite.Suite
	f         *versionFixture
	svc       *versionService
	published func() []publishedEvent
}

func TestVersionServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(VersionServiceAuditTestSuite))
}

// SetupTest administers gw-1 and gw-2, publishing on a recording bus, with one version captured.
func (s *VersionServiceAuditTestSuite) SetupTest() {
	s.f = newVersionFixture(s.T())
	second := s.f.gateways.gateways[0]
	second.ID, second.Name = scopeTarget, testProdName
	s.f.gateways.gateways = append(s.f.gateways.gateways, second)
	s.svc = s.f.svc.(*versionService)
	var bus eventbus.EventBusInterface
	bus, s.published = newRecordingBus(s.T())
	s.svc.bus = bus
	s.f.capture(s.T(), "ou-a")
}

func (s *VersionServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "", "", nil, nil))
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the gateway with gatewayID. An attempt that succeeded names the gateway as subject too; a failed
// one has no subject. No value of the version or of the gateway is recorded.
func (s *VersionServiceAuditTestSuite) requireOnlyEvent(name string, action audit.Action, gatewayID string,
	outcome audit.Outcome) {
	s.T().Helper()
	events := s.published()
	s.Require().Len(events, 1)
	event := events[0]
	gateway := audit.Ref{Type: audit.TargetTypeGateway, ID: gatewayID}
	s.Equal(name, event.name)
	s.Equal(gateway, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details(), "no part of the version or the gateway is recorded")
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no gateway")
	} else {
		s.Equal(gateway, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	s.NotContains(recorded, "the-key", "the gateway's key is never recorded")
	s.NotContains(recorded, "ou-a", "the version's content is never recorded")
	s.NotContains(recorded, "https://dp.test", "the gateway's address is not recorded")
}

func (s *VersionServiceAuditTestSuite) TestAnApplyRecordsItsOutcome() {
	cases := []struct {
		name      string
		gatewayID string
		req       ApplyRequest
		outcome   audit.Outcome
	}{
		{"an apply of the latest version", "gw-1", ApplyRequest{}, audit.OutcomeSucceeded},
		{"an apply with a selection", "gw-1", ApplyRequest{Selection: &[]string{}}, audit.OutcomeSucceeded},
		{"an apply to a gateway that does not exist", "gw-missing", ApplyRequest{},
			audit.OutcomeFailed("gtw_1002")},
		{"an apply of a version that does not exist", "gw-1", ApplyRequest{Version: "0000000"},
			audit.OutcomeFailed("gtw_1013")},
		{"an apply whose origin is too long", "gw-1",
			ApplyRequest{Origin: strings.Repeat("o", maxOriginLength+1)}, audit.OutcomeFailed("gtw_1035")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, svcErr := s.svc.Apply(adminContext(), tc.gatewayID, tc.req)

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventGatewayApply, audit.ActionApply, tc.gatewayID, tc.outcome)
		})
	}
}

func (s *VersionServiceAuditTestSuite) TestARevertRecordsItsOutcome() {
	cases := []struct {
		name      string
		gatewayID string
		req       RevertRequest
		applied   bool
		outcome   audit.Outcome
	}{
		{"a revert to the version held before", "gw-1", RevertRequest{}, true, audit.OutcomeSucceeded},
		{"a revert with nothing to return to", "gw-1", RevertRequest{}, false, audit.OutcomeFailed("gtw_1016")},
		{"a revert of a gateway that does not exist", "gw-missing", RevertRequest{}, false,
			audit.OutcomeFailed("gtw_1002")},
		{"a revert of a project the deployment does not have", "gw-1", RevertRequest{Project: "shop"}, true,
			audit.OutcomeFailed("gtw_1038")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.applied {
				s.f.capture(s.T(), "ou-b")
				_, svcErr := s.svc.apply(adminContext(), "gw-1", ApplyRequest{Version: s.f.hash(1)})
				s.Require().Nil(svcErr)
				_, svcErr = s.svc.apply(adminContext(), "gw-1", ApplyRequest{Version: s.f.hash(2)})
				s.Require().Nil(svcErr)
			}

			_, svcErr := s.svc.Revert(adminContext(), tc.gatewayID, tc.req)

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventGatewayRevert, audit.ActionRevert, tc.gatewayID, tc.outcome)
		})
	}
}

// A promotion changes the target gateway as an apply does, so it is recorded as an apply to the target.
func (s *VersionServiceAuditTestSuite) TestAPromotionIsRecordedAsAnApplyToItsTarget() {
	cases := []struct {
		name    string
		req     PromoteRequest
		applied bool
		target  string
		outcome audit.Outcome
	}{
		{"a promotion of what the source runs", PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: scopeTarget},
			true, scopeTarget, audit.OutcomeSucceeded},
		{"a promotion from a source that runs nothing",
			PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: scopeTarget}, false, scopeTarget,
			audit.OutcomeFailed("gtw_1037")},
		{"a promotion to the source itself", PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: "gw-1"},
			false, "gw-1", audit.OutcomeFailed("gtw_1036")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.applied {
				_, svcErr := s.svc.apply(adminContext(), "gw-1", ApplyRequest{})
				s.Require().Nil(svcErr)
			}

			_, svcErr := s.svc.Promote(adminContext(), tc.req)

			s.Equal(tc.outcome.Result == audit.ResultSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventGatewayApply, audit.ActionApply, tc.target, tc.outcome)
		})
	}
}

// A dry run changes nothing, so it publishes nothing, whether it would succeed or fail.
func (s *VersionServiceAuditTestSuite) TestADryRunPublishesNothing() {
	_, svcErr := s.svc.Apply(adminContext(), "gw-1", ApplyRequest{DryRun: true})
	s.Require().Nil(svcErr)
	_, svcErr = s.svc.Apply(adminContext(), "gw-missing", ApplyRequest{DryRun: true})
	s.Require().NotNil(svcErr)
	_, svcErr = s.svc.Revert(adminContext(), "gw-1", RevertRequest{DryRun: true})
	s.Require().NotNil(svcErr)
	_, svcErr = s.svc.Promote(adminContext(), PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: scopeTarget,
		DryRun: true})
	s.Require().NotNil(svcErr)

	s.Empty(s.published())
}

// An apply queued behind a running one is recorded when it is asked for, by who asked.
func (s *VersionServiceAuditTestSuite) TestAQueuedApplyIsRecordedWhenItIsAskedFor() {
	s.f.versions.leases = map[string]*applyLease{"gw-1": {Token: "running", Since: time.Now()}}

	result, svcErr := s.svc.Apply(adminContext(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	s.Require().True(result.Queued)
	s.Empty(s.f.client.sent)
	s.requireOnlyEvent(audit.EventGatewayApply, audit.ActionApply, "gw-1", audit.OutcomeSucceeded)
}

// The run that carries out a queued apply once the running one finishes publishes nothing: its context
// is the running apply's, whose caller is not who asked for it.
func (s *VersionServiceAuditTestSuite) TestTheRunThatCarriesOutAQueuedApplyPublishesNothing() {
	s.svc.applyLatestAgain(adminContext(), "gw-1")

	s.Require().Len(s.f.client.sent, 1, "the queued apply was not carried out")
	s.Empty(s.published())
}

// An apply auto apply makes after a capture is recorded as any other apply, once.
func (s *VersionServiceAuditTestSuite) TestAnAutoApplyIsRecorded() {
	s.svc.autoApply = fixedAutoApply(true)
	s.svc.gateways.(*fakeStore).gateways[0].IsDefault = true

	s.svc.applyToFirstGateway(adminContext(), nil)

	s.Require().Len(s.f.client.sent, 1)
	s.requireOnlyEvent(audit.EventGatewayApply, audit.ActionApply, "gw-1", audit.OutcomeSucceeded)
}

// A bus that cannot take the event leaves the apply as it was.
func (s *VersionServiceAuditTestSuite) TestAFailingBusDoesNotFailTheApply() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.svc.bus = failing

	result, svcErr := s.svc.Apply(adminContext(), "gw-1", ApplyRequest{})

	s.Require().Nil(svcErr)
	s.True(result.Recorded)
}

func (s *VersionServiceAuditTestSuite) TestInitializeRefusesANilEventBus() {
	service, err := Initialize(http.NewServeMux(), nil, nil, nil, nil, nil, nil, nil, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Nil(service)
}
