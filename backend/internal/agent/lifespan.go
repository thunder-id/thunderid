// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/agent/model"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// lifespanRestoreKey marks a context whose agent writes restore an exported lifespan.
type lifespanRestoreKey struct{}

// WithLifespanRestore marks ctx as restoring agents as they were exported, as an import does. A
// validUntil already in the past is then accepted, as it is from a declarative file, so an expired
// agent is restored expired rather than refused. Every other lifespan check still applies.
func WithLifespanRestore(ctx context.Context) context.Context {
	return context.WithValue(ctx, lifespanRestoreKey{}, true)
}

// isLifespanRestore reports whether ctx restores agents as they were exported.
func isLifespanRestore(ctx context.Context) bool {
	restore, _ := ctx.Value(lifespanRestoreKey{}).(bool)
	return restore
}

// unboundedLifespanAt is the instant the unbounded sentinel names.
var unboundedLifespanAt, _ = time.Parse(time.RFC3339, authnprovidercm.UnboundedValidUntil)

// lifespanEnd is the instant an agent's lifespan ends. The zero value is a lifespan that never ends.
type lifespanEnd struct {
	at      time.Time
	bounded bool
}

// boundedLifespanEnd returns a lifespan that ends at t, at second precision in UTC, the form the
// identity governance authority reads.
func boundedLifespanEnd(t time.Time) lifespanEnd {
	return lifespanEnd{at: t.UTC().Truncate(time.Second), bounded: true}
}

// String returns the stored form: a UTC RFC 3339 instant, or the unbounded sentinel.
func (e lifespanEnd) String() string {
	if !e.bounded {
		return authnprovidercm.UnboundedValidUntil
	}
	return e.at.Format(time.RFC3339)
}

// equal reports whether e and other end at the same instant, or both never end.
func (e lifespanEnd) equal(other lifespanEnd) bool {
	return e.bounded == other.bounded && e.at.Equal(other.at)
}

// endsAfter reports whether e ends later than other. A lifespan that never ends ends after any other.
func (e lifespanEnd) endsAfter(other lifespanEnd) bool {
	if !other.bounded {
		return false
	}
	return !e.bounded || e.at.After(other.at)
}

// expired reports whether the lifespan has ended by now. It ends at its validUntil instant.
func (e lifespanEnd) expired(now time.Time) bool {
	return e.bounded && !now.Before(e.at)
}

// lifespanChange is a requested change of an agent's lifespan. previous is nil when the stored value
// cannot be read.
type lifespanChange struct {
	previous *lifespanEnd
	next     lifespanEnd
}

// lifespanUpdate is the lifespan part of an agent update. changing is false when the request leaves
// the lifespan alone: validUntil is absent, or equal to the stored value.
type lifespanUpdate struct {
	change   lifespanChange
	changing bool
	// parseErr is set when the requested value is not a valid lifespan end.
	parseErr *tidcommon.ServiceError
}

// newLifespanUpdate reads the lifespan part of an update request against the agent's stored system
// attributes.
func newLifespanUpdate(storedAttributes, requested json.RawMessage) lifespanUpdate {
	next, isRequested, parseErr := parseUpdateValidUntil(requested)
	change := lifespanChange{previous: storedLifespanEnd(storedAttributes), next: next}
	return lifespanUpdate{
		change: change,
		changing: isRequested &&
			(parseErr != nil || change.previous == nil || !change.next.equal(*change.previous)),
		parseErr: parseErr,
	}
}

// checkLifespanUpdate refuses a lifespan change to a value that is not a valid lifespan end in the
// future. A refusal is published with its failure class.
func (s *agentService) checkLifespanUpdate(ctx context.Context, agentID string, update lifespanUpdate,
	now time.Time) *tidcommon.ServiceError {
	if !update.changing {
		return nil
	}
	svcErr := update.parseErr
	if svcErr == nil && !isLifespanRestore(ctx) {
		svcErr = validateFutureLifespanEnd(update.change.next, now)
	}
	if svcErr != nil {
		s.refuseLifespanUpdate(ctx, agentID, update, lifespanFailureInvalidLifespan)
	}
	return svcErr
}

// writeAgentUpdate writes an agent update and its lifespan change in one entity-database transaction,
// and returns the agent as written. The generic update carries the stored validUntil forward, so the
// change is written by the entity service's lifespan setter, and the agent is read again after it so
// the status reported for the update reflects the new lifespan end.
func (s *agentService) writeAgentUpdate(ctx context.Context, agentID string, entity *providers.Entity,
	update lifespanUpdate) (*providers.Entity, error) {
	var updated *providers.Entity
	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		var err error
		if updated, err = s.entityService.UpdateEntity(txCtx, agentID, entity); err != nil {
			return err
		}
		if !update.changing {
			return nil
		}
		if err := s.entityService.SetValidUntil(txCtx, agentID, update.change.next.String()); err != nil {
			return err
		}
		updated, err = s.entityService.GetEntity(txCtx, agentID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// completeLifespanUpdate publishes a committed lifespan change and returns the agent's lifespan end,
// nil when it cannot be read.
func (s *agentService) completeLifespanUpdate(ctx context.Context, agentID string, update lifespanUpdate,
	now time.Time) *lifespanEnd {
	if !update.changing {
		return update.change.previous
	}
	s.publishLifespanChanged(ctx, agentID, update.change, now)
	return &update.change.next
}

// refuseLifespanUpdate publishes AGENT_LIFESPAN_UPDATE_FAILED when the refused or failed update
// carried a lifespan change.
func (s *agentService) refuseLifespanUpdate(ctx context.Context, agentID string, update lifespanUpdate,
	failureClass lifespanFailureClass) {
	if update.changing {
		s.publishLifespanUpdateFailed(ctx, agentID, update.change, failureClass)
	}
}

// failLifespanUpdate publishes the lifespan change of an update that failed with svcErr, as
// not_authorized when the caller was refused and as operation_failed otherwise.
func (s *agentService) failLifespanUpdate(ctx context.Context, agentID string, update lifespanUpdate,
	svcErr *tidcommon.ServiceError) {
	failureClass := lifespanFailureOperationFailed
	if svcErr.Code == tidcommon.ErrorUnauthorized.Code {
		failureClass = lifespanFailureNotAuthorized
	}
	s.refuseLifespanUpdate(ctx, agentID, update, failureClass)
}

// publishLifespanSet publishes AGENT_LIFESPAN_SET for a registered agent.
func (s *agentService) publishLifespanSet(ctx context.Context, agentID string, end lifespanEnd) {
	s.publishLifespanEvent(ctx, event.EventTypeAgentLifespanSet, agentID, "", end.String(),
		providers.StatusSuccess, "")
}

// publishLifespanChanged publishes the events for a committed lifespan change: an extension or a
// reduction and, when an expired agent's lifespan now ends in the future, its reactivation. A change
// from a stored value that cannot be read is neither, so it is published as the lifespan being set.
func (s *agentService) publishLifespanChanged(ctx context.Context, agentID string, change lifespanChange,
	now time.Time) {
	previous, next := storedForm(change.previous), change.next.String()
	switch {
	case change.previous == nil:
		s.publishLifespanEvent(ctx, event.EventTypeAgentLifespanSet, agentID, previous, next,
			providers.StatusSuccess, "")
	case change.next.endsAfter(*change.previous):
		s.publishLifespanEvent(ctx, event.EventTypeAgentLifespanExtended, agentID, previous, next,
			providers.StatusSuccess, "")
		if change.previous.expired(now) && !change.next.expired(now) {
			s.publishLifespanEvent(ctx, event.EventTypeAgentReactivated, agentID, previous, next,
				providers.StatusSuccess, "")
		}
	default:
		s.publishLifespanEvent(ctx, event.EventTypeAgentLifespanReduced, agentID, previous, next,
			providers.StatusSuccess, "")
	}
}

// publishLifespanUpdateFailed publishes AGENT_LIFESPAN_UPDATE_FAILED for a refused or failed change.
func (s *agentService) publishLifespanUpdateFailed(ctx context.Context, agentID string,
	change lifespanChange, failureClass lifespanFailureClass) {
	s.publishLifespanEvent(ctx, event.EventTypeAgentLifespanUpdateFailed, agentID, storedForm(change.previous),
		change.next.String(), providers.StatusFailure, failureClass)
}

// publishLifespanEvent publishes a lifespan administration event carrying the agent, the caller,
// and the previous and new lifespan ends. A failure to publish does not undo the change.
func (s *agentService) publishLifespanEvent(ctx context.Context, eventType providers.EventType,
	agentID, previous, next string, status string, failureClass lifespanFailureClass) {
	if s.observabilitySvc == nil || !s.observabilitySvc.IsEnabled() {
		return
	}

	evt := event.NewEvent(syscontext.GetTraceID(ctx), string(eventType), event.ComponentAgentService).
		WithStatus(status).
		WithData(event.DataKey.Subject, agentID).
		WithData(event.DataKey.SubjectType, event.PrincipalTypeAgent)
	if actor := security.GetSubject(ctx); actor != "" {
		evt.WithData(event.DataKey.ActorSub, actor)
		if actorType := s.actorType(ctx, actor); actorType != "" {
			evt.WithData(event.DataKey.ActorType, actorType)
		}
	}
	if previous != "" {
		evt.WithData(event.DataKey.PreviousValidUntil, previous)
	}
	if next != "" {
		evt.WithData(event.DataKey.ValidUntil, next)
	}
	if failureClass != "" {
		evt.WithData(event.DataKey.Error, string(failureClass))
	}

	s.observabilitySvc.PublishEvent(ctx, evt)
}

// actorType returns the category of the calling entity, or an empty string when it cannot be read.
func (s *agentService) actorType(ctx context.Context, actor string) string {
	e, err := s.entityService.GetEntityProfile(ctx, actor)
	if err != nil || e == nil {
		s.logger.Debug(ctx, "Could not resolve the category of the lifespan change actor", log.Error(err))
		return ""
	}
	return event.PrincipalType(string(e.Category))
}

// parseLifespanEnd parses value as an RFC 3339 instant. The unbounded sentinel, and any instant past
// it, is a lifespan that never ends. An instant whose UTC year is negative is refused, since its
// stored form could not be read back.
func parseLifespanEnd(value string) (lifespanEnd, *tidcommon.ServiceError) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return lifespanEnd{}, &ErrorInvalidLifespan
	}
	end := boundedLifespanEnd(parsed)
	if end.at.Year() < 0 {
		return lifespanEnd{}, &ErrorInvalidLifespan
	}
	if !end.at.Before(unboundedLifespanAt) {
		return lifespanEnd{}, nil
	}
	return end, nil
}

// resolveRequestedValidUntil validates a lifespan end requested at registration through the API, a
// flow or an import. nil requests an unbounded lifespan. A past value is refused unless allowPast, as
// when an import restores an expired agent.
func resolveRequestedValidUntil(value *string, now time.Time, allowPast bool) (
	lifespanEnd, *tidcommon.ServiceError) {
	if value == nil {
		return lifespanEnd{}, nil
	}
	end, svcErr := parseLifespanEnd(*value)
	if svcErr != nil {
		return lifespanEnd{}, svcErr
	}
	if allowPast {
		return end, nil
	}
	if svcErr := validateFutureLifespanEnd(end, now); svcErr != nil {
		return lifespanEnd{}, svcErr
	}
	return end, nil
}

// validateFutureLifespanEnd refuses a lifespan end that is not later than now.
func validateFutureLifespanEnd(end lifespanEnd, now time.Time) *tidcommon.ServiceError {
	if end.bounded && !end.at.After(now) {
		return &ErrorLifespanNotInFuture
	}
	return nil
}

// parseUpdateValidUntil reads the validUntil of an update request. requested is false when the field
// is absent, which leaves the lifespan unchanged. JSON null requests an unbounded lifespan. Whether
// the value ends in the future is checked only once it is known to differ from the stored value, so
// resending an expired agent's lifespan unchanged is not refused.
func parseUpdateValidUntil(raw json.RawMessage) (end lifespanEnd, requested bool, svcErr *tidcommon.ServiceError) {
	if len(raw) == 0 {
		return lifespanEnd{}, false, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return lifespanEnd{}, true, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return lifespanEnd{}, true, &ErrorInvalidLifespan
	}
	end, svcErr = parseLifespanEnd(value)
	return end, true, svcErr
}

// ValidateLifespanFormat refuses a validUntil that is not an RFC 3339 date and time, whether or not it
// is in the past, so an import's dry run reports a value the import itself would refuse.
func ValidateLifespanFormat(value string) *tidcommon.ServiceError {
	_, svcErr := parseLifespanEnd(value)
	return svcErr
}

// resolveDeclarativeValidUntil reads the validUntil of a declarative agent into stored form. A past
// value is accepted, so a file keeps loading after its agent's lifespan ends. An absent value is
// returned empty and stored as absent, which is unbounded.
func resolveDeclarativeValidUntil(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	end, svcErr := parseLifespanEnd(value)
	if svcErr != nil {
		return "", errInvalidDeclarativeLifespan
	}
	return end.String(), nil
}

// storedLifespanEnd returns the lifespan end held in an agent's system attributes, or nil when the
// stored value cannot be read. An agent registered before lifespans existed has none and is unbounded.
func storedLifespanEnd(systemAttributes json.RawMessage) *lifespanEnd {
	at, bounded, err := authnprovidercm.ValidUntilOf(systemAttributes)
	if err != nil {
		return nil
	}
	end := lifespanEnd{}
	if bounded {
		end = boundedLifespanEnd(at)
	}
	return &end
}

// lifespanView returns the validUntil and lifespan reported on reads. validUntil is omitted when the
// lifespan is unbounded, and both are omitted when the stored value cannot be read.
func lifespanView(end *lifespanEnd, now time.Time) (string, *model.Lifespan) {
	switch {
	case end == nil:
		return "", nil
	case !end.bounded:
		return "", &model.Lifespan{Phase: model.LifespanPhaseUnbounded}
	case end.expired(now):
		return end.String(), &model.Lifespan{Phase: model.LifespanPhaseExpired}
	default:
		return end.String(), &model.Lifespan{Phase: model.LifespanPhaseActive}
	}
}

// storedForm returns the stored form of end, empty when it cannot be read.
func storedForm(end *lifespanEnd) string {
	if end == nil {
		return ""
	}
	return end.String()
}
