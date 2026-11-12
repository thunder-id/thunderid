// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package thunderidengine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/auditsinkmock"
)

const auditWait = 5 * time.Second

type AuditSinkTestSuite struct {
	suite.Suite
}

func TestAuditSinkTestSuite(t *testing.T) {
	suite.Run(t, new(AuditSinkTestSuite))
}

// publishTokenIssue publishes one audit event on bus from a request that came from a public address.
func publishTokenIssue(bus eventbus.EventBusInterface) error {
	ctx := sysContext.WithClient(
		context.WithValue(context.Background(), sysContext.TraceIDKey, "trace-1"), "198.51.100.7", "audit-test/1.0")
	payload := audit.NewPayload(ctx, audit.Ref{Type: "token", ID: "token-1"}, audit.ActionIssue,
		audit.OutcomeSucceeded).WithDetails(map[string]string{audit.DetailGrantType: "client_credentials"})
	return bus.Publish(ctx, eventbus.NewEvent(ctx, audit.EventTokenIssue).With(payload))
}

// AC11.1: a sink added through the option receives the envelope and the audit payload.
func (s *AuditSinkTestSuite) TestSinkReceivesTheEnvelopeAndThePayload() {
	bus, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	received := make(chan providers.AuditEvent, 1)
	sink := auditsinkmock.NewAuditSinkMock(s.T())
	sink.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, event providers.AuditEvent) error {
			received <- event
			return nil
		}).Once()
	require.NoError(s.T(), subscribeAuditSinks(registry, []providers.AuditSink{sink}))

	require.NoError(s.T(), publishTokenIssue(bus))

	select {
	case event := <-received:
		s.NotEmpty(event.ID)
		s.Equal(audit.EventTokenIssue, event.Name)
		s.Equal("trace-1", event.TraceID)
		s.False(event.Time.IsZero())
		s.Equal(time.UTC, event.Time.Location())
		s.Equal(providers.AuditRef{Type: string(audit.ActorTypeAnonymous)}, event.Actor)
		s.Equal(providers.AuditRef{Type: "token", ID: "token-1"}, event.Target)
		s.Equal("issue", event.Action)
		s.Equal(providers.AuditOutcome{Result: "succeeded"}, event.Outcome)
		s.Equal(providers.AuditClient{Address: "198.51.100.7", UserAgent: "audit-test/1.0"}, event.Client)
		s.Equal(map[string]string{"grant_type": "client_credentials"}, event.Details)
	case <-time.After(auditWait):
		s.Fail("the sink received no event")
	}
}

// AC11.2: a slow sink delays neither the publisher nor another sink.
func (s *AuditSinkTestSuite) TestSlowSinkDelaysNeitherThePublisherNorAnotherSink() {
	bus, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	release := make(chan struct{})
	slow := auditsinkmock.NewAuditSinkMock(s.T())
	slow.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, _ providers.AuditEvent) error {
			select {
			case <-release:
			case <-ctx.Done():
			}
			return nil
		}).Maybe()

	fast := make(chan providers.AuditEvent, 1)
	other := auditsinkmock.NewAuditSinkMock(s.T())
	other.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, event providers.AuditEvent) error {
			fast <- event
			return nil
		}).Once()
	require.NoError(s.T(), subscribeAuditSinks(registry, []providers.AuditSink{slow, other}))
	s.T().Cleanup(func() { close(release) })

	started := time.Now()
	require.NoError(s.T(), publishTokenIssue(bus))
	s.Less(time.Since(started), time.Second)

	select {
	case <-fast:
	case <-time.After(auditWait):
		s.Fail("the second sink waited for the slow one")
	}
}

// A sink's failure and its edits to an event reach no other sink.
func (s *AuditSinkTestSuite) TestSinksAreIsolatedFromEachOther() {
	bus, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	edited := make(chan struct{})
	failing := auditsinkmock.NewAuditSinkMock(s.T())
	failing.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, event providers.AuditEvent) error {
			event.Details["grant_type"] = "changed"
			close(edited)
			return errors.New("sink is down")
		}).Once()

	received := make(chan providers.AuditEvent, 1)
	other := auditsinkmock.NewAuditSinkMock(s.T())
	other.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, event providers.AuditEvent) error {
			received <- event
			return nil
		}).Once()
	require.NoError(s.T(), subscribeAuditSinks(registry, []providers.AuditSink{failing, other}))

	require.NoError(s.T(), publishTokenIssue(bus))

	select {
	case event := <-received:
		s.Equal("client_credentials", event.Details["grant_type"])
	case <-time.After(auditWait):
		s.Fail("the second sink received no event")
	}
	select {
	case <-edited:
	case <-time.After(auditWait):
		s.Fail("the failing sink received no event")
	}
}

// AC11.3: with no sink added the engine subscribes nothing, so an event is dropped and nothing breaks.
func (s *AuditSinkTestSuite) TestNoSinkSubscribesNothingAndPublishingStillWorks() {
	bus, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	require.NoError(s.T(), subscribeAuditSinks(registry, nil))
	s.NoError(publishTokenIssue(bus))
}

// Each sink is subscribed under its own stable ID, which follows its position in the options.
func (s *AuditSinkTestSuite) TestSinksGetDistinctStableIDs() {
	_, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	first := auditsinkmock.NewAuditSinkMock(s.T())
	second := auditsinkmock.NewAuditSinkMock(s.T())
	s.NoError(subscribeAuditSinks(registry, []providers.AuditSink{first, second}))

	s.Error(subscribeAuditSinks(registry, []providers.AuditSink{first}), "an ID is taken once")
}

// The option adds a sink each time it is passed, in order.
func (s *AuditSinkTestSuite) TestWithAuditSinkAddsEachSinkInOrder() {
	first := auditsinkmock.NewAuditSinkMock(s.T())
	second := auditsinkmock.NewAuditSinkMock(s.T())

	var ctx engineContext
	WithAuditSink(first)(&ctx)
	WithAuditSink(second)(&ctx)

	s.Equal([]providers.AuditSink{first, second}, ctx.auditSinks)
}

// A nil sink is refused at start rather than silently leaving the embedder without audit.
func (s *AuditSinkTestSuite) TestNilSinkIsRefused() {
	ctx := validEngineContext(s.T())
	WithAuditSink(nil)(ctx)

	s.ErrorContains(validateEngineContext(ctx), "audit sink is nil")
}

// AC11.3: an engine context with no sink is valid.
func (s *AuditSinkTestSuite) TestNoSinkIsValid() {
	ctx := validEngineContext(s.T())

	s.Empty(ctx.auditSinks)
	s.NoError(validateEngineContext(ctx))
}
