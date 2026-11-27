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
	// The engine does not populate the client, so the test sets it as the embedder's HTTP layer would.
	ctx := sysContext.WithClient(
		context.WithValue(context.Background(), sysContext.TraceIDKey, "trace-1"), "198.51.100.7", "audit-test/1.0")
	payload := audit.NewPayload(ctx, audit.Ref{Type: "token", ID: "token-1"}, audit.ActionIssue,
		audit.OutcomeSucceeded).WithDetails(map[string]string{audit.DetailGrantType: "client_credentials"})
	return bus.Publish(ctx, eventbus.NewEvent(ctx, audit.EventTokenIssue).With(payload))
}

// AC9.1: a sink added through the option receives the envelope and the audit payload.
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
		s.Equal(providers.AuditActor{Type: providers.AuditActorTypeAnonymous}, event.Actor)
		s.Equal(providers.AuditRef{Type: "token", ID: "token-1"}, event.Target)
		s.Equal(providers.AuditActionIssue, event.Action)
		s.Equal(providers.AuditOutcome{Result: providers.AuditResultSucceeded}, event.Outcome)
		s.Equal(providers.AuditClient{Address: "198.51.100.7", UserAgent: "audit-test/1.0"}, event.Client)
		s.Equal(map[string]string{"grant_type": "client_credentials"}, event.Details)
	case <-time.After(auditWait):
		s.Fail("the sink received no event")
	}
}

// AC9.2: a slow sink delays neither the publisher nor another sink.
func (s *AuditSinkTestSuite) TestSlowSinkDelaysNeitherThePublisherNorAnotherSink() {
	bus, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	release := make(chan struct{})
	entered := make(chan struct{})
	slow := auditsinkmock.NewAuditSinkMock(s.T())
	slow.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, _ providers.AuditEvent) error {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
			}
			return nil
		}).Once()

	fast := make(chan providers.AuditEvent, 1)
	other := auditsinkmock.NewAuditSinkMock(s.T())
	other.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, event providers.AuditEvent) error {
			fast <- event
			return nil
		}).Once()
	require.NoError(s.T(), subscribeAuditSinks(registry, []providers.AuditSink{slow, other}))
	s.T().Cleanup(func() { close(release) })

	require.NoError(s.T(), publishTokenIssue(bus))

	select {
	case <-fast:
	case <-time.After(auditWait):
		s.Fail("the second sink waited for the slow one")
	}
	select {
	case <-entered:
	case <-time.After(auditWait):
		s.Fail("the slow sink never ran")
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
			// Read only after the failing sink has edited its copy, so a shared map shows the edit.
			select {
			case <-edited:
			case <-time.After(auditWait):
			}
			received <- event
			return nil
		}).Once()
	require.NoError(s.T(), subscribeAuditSinks(registry, []providers.AuditSink{failing, other}))

	require.NoError(s.T(), publishTokenIssue(bus))

	select {
	case <-edited:
	case <-time.After(auditWait):
		s.Fail("the failing sink received no event")
	}
	select {
	case event := <-received:
		s.Equal("client_credentials", event.Details["grant_type"])
	case <-time.After(auditWait):
		s.Fail("the second sink received no event")
	}
}

// AC9.3: with no sink added the engine subscribes nothing, so an event is dropped and nothing breaks.
func (s *AuditSinkTestSuite) TestNoSinkSubscribesNothingAndPublishingStillWorks() {
	bus, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	require.NoError(s.T(), subscribeAuditSinks(registry, nil))
	s.NoError(publishTokenIssue(bus))
}

// AC9.5: each sink is subscribed under its own stable ID, which follows its position in the options.
func (s *AuditSinkTestSuite) TestSinksGetDistinctStableIDs() {
	_, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)

	first := auditsinkmock.NewAuditSinkMock(s.T())
	second := auditsinkmock.NewAuditSinkMock(s.T())
	s.NoError(subscribeAuditSinks(registry, []providers.AuditSink{first, second}))

	for _, id := range []string{"audit-embedder-1", "audit-embedder-2"} {
		s.Error(eventbus.Subscribe[audit.Payload](registry, id, auditSinkAdapter{sink: first}),
			"the ID %s is taken once", id)
	}
	s.NoError(eventbus.Subscribe[audit.Payload](registry, "audit-embedder-3", auditSinkAdapter{sink: first}))
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

// A nil sink, an interface or a typed nil, is ignored as no sink, and the sinks added stay in order.
func (s *AuditSinkTestSuite) TestNilSinkIsIgnored() {
	var typedNil *auditsinkmock.AuditSinkMock
	first := auditsinkmock.NewAuditSinkMock(s.T())
	second := auditsinkmock.NewAuditSinkMock(s.T())

	ctx := validEngineContext(s.T())
	WithAuditSink(nil)(ctx)
	WithAuditSink(typedNil)(ctx)
	s.Empty(ctx.auditSinks)
	s.NoError(validateEngineContext(ctx))

	WithAuditSink(first)(ctx)
	WithAuditSink(nil)(ctx)
	WithAuditSink(typedNil)(ctx)
	WithAuditSink(second)(ctx)
	s.Equal([]providers.AuditSink{first, second}, ctx.auditSinks)
	s.NoError(validateEngineContext(ctx))

	_, registry, stopper := eventbus.Initialize()
	s.T().Cleanup(stopper.Stop)
	s.NoError(subscribeAuditSinks(registry, ctx.auditSinks))
	for _, id := range []string{"audit-embedder-1", "audit-embedder-2"} {
		s.Error(eventbus.Subscribe[audit.Payload](registry, id, auditSinkAdapter{sink: first}), id)
	}
	s.NoError(eventbus.Subscribe[audit.Payload](registry, "audit-embedder-3", auditSinkAdapter{sink: first}))
}

// AC9.3: an engine context with no sink is valid, so the engine starts without one.
func (s *AuditSinkTestSuite) TestNoSinkIsValid() {
	ctx := validEngineContext(s.T())

	s.Empty(ctx.auditSinks)
	s.NoError(validateEngineContext(ctx))
}
