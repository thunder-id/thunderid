// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"maps"
	"slices"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var _ eventbus.Payload = Payload{}

// OutcomeSucceeded is the outcome of an attempt that completed.
var OutcomeSucceeded = Outcome{Result: ResultSucceeded}

// OutcomeDenied is the outcome of an attempt a security decision refused, with a stable reason code.
func OutcomeDenied(reason string) Outcome {
	return Outcome{Result: ResultDenied, Reason: reason}
}

// OutcomeFailed is the outcome of an attempt that was allowed but did not complete, with a stable
// reason code.
func OutcomeFailed(reason string) Outcome {
	return Outcome{Result: ResultFailed, Reason: reason}
}

// outcomeFromServiceError is the outcome of an attempt a service ended with svcErr: succeeded without
// an error, denied as forbidden when the service's access check refused the caller or its grant check
// refused to confer permissions the caller does not hold, and failed with the reason derived from the
// error code otherwise.
func outcomeFromServiceError(svcErr *tidcommon.ServiceError) Outcome {
	switch {
	case svcErr == nil:
		return OutcomeSucceeded
	case svcErr.Code == tidcommon.ErrorUnauthorized.Code, svcErr.Code == sysauthz.ErrorGrantNotPermitted.Code:
		return OutcomeDenied(reasonForbidden)
	default:
		return OutcomeFailed(reasonFromCode(svcErr.Code))
	}
}

// Payload is the audit view of an event. Build it with NewPayload, so the actor and the client come
// from the request rather than from the call site.
type Payload struct {
	actor      Actor
	onBehalfOf Ref
	subject    Ref
	target     Ref
	action     Action
	outcome    Outcome
	client     Client
	details    map[string]string
}

// NewPayload builds the audit payload of an attempt on target. The actor, the principal it acted
// for and the client are resolved from ctx, which carries the verified security context and where the
// request came from. Every ID is cut to 128 characters.
func NewPayload(ctx context.Context, target Ref, action Action, outcome Outcome) Payload {
	actor, onBehalfOf := resolveActor(ctx)
	actor.ID = capID(actor.ID)
	onBehalfOf.ID = capID(onBehalfOf.ID)
	target.ID = capID(target.ID)
	address, userAgent := syscontext.GetClient(ctx)
	return Payload{
		actor:      actor,
		onBehalfOf: onBehalfOf,
		target:     target,
		action:     action,
		outcome:    outcome,
		client:     Client{Address: address, UserAgent: userAgent},
	}
}

// ChangePayload builds the audit payload of an attempt to change target that a service ended with
// svcErr. The target is the subject when the attempt succeeded or was denied and names it: a service
// refuses a change only to a target it found, while a failed attempt may name one that does not exist.
func ChangePayload(ctx context.Context, target Ref, action Action, svcErr *tidcommon.ServiceError) Payload {
	outcome := outcomeFromServiceError(svcErr)
	payload := NewPayload(ctx, target, action, outcome)
	if outcome.Result != ResultFailed && target.ID != "" {
		payload = payload.WithSubject(target)
	}
	return payload
}

// MembershipPayloads builds the audit payloads of an attempt to add members to target, or remove them,
// that a service ended with svcErr. An attempt that succeeded has one payload per distinct member, with
// the member as subject, so the log names everyone who gained or lost the membership. Any other attempt
// has one payload with the target only, since a call may name several members.
func MembershipPayloads(ctx context.Context, target Ref, members []Ref, action Action,
	svcErr *tidcommon.ServiceError) []Payload {
	outcome := outcomeFromServiceError(svcErr)
	if outcome.Result != ResultSucceeded {
		return []Payload{NewPayload(ctx, target, action, outcome)}
	}
	payloads := make([]Payload, 0, len(members))
	seen := make(map[Ref]struct{}, len(members))
	for _, member := range members {
		if _, ok := seen[member]; ok {
			continue
		}
		seen[member] = struct{}{}
		payloads = append(payloads, NewPayload(ctx, target, action, outcome).WithSubject(member))
	}
	return payloads
}

// BatchPayload builds the audit payload of a request on target that acts on several items and that a
// service ended with svcErr. A request that completed with itemFailed set failed with item_failed, so a
// partial result is never recorded as succeeded; any other is recorded as ChangePayload records it.
func BatchPayload(ctx context.Context, target Ref, action Action, svcErr *tidcommon.ServiceError,
	itemFailed bool) Payload {
	if svcErr == nil && itemFailed {
		return NewPayload(ctx, target, action, OutcomeFailed(reasonItemFailed))
	}
	return ChangePayload(ctx, target, action, svcErr)
}

// Category returns the audit category.
func (Payload) Category() eventbus.Category {
	return CategoryAudit
}

// WithActor replaces the actor resolved from the request and clears the principal it acted for. An
// OAuth endpoint authenticates a client rather than a token, and a bearer token sent to one must never
// decide the actor.
func (p Payload) WithActor(actor Actor) Payload {
	actor.ID = capID(actor.ID)
	p.actor = actor
	p.onBehalfOf = Ref{}
	return p
}

// WithSubject names the identity whose activity log the event belongs to.
func (p Payload) WithSubject(subject Ref) Payload {
	subject.ID = capID(subject.ID)
	p.subject = subject
	return p
}

// WithDetails records facts specific to one event. An event whose details carry a key the event does
// not allow is never written.
func (p Payload) WithDetails(details map[string]string) Payload {
	p.details = maps.Clone(details)
	return p
}

// Actor returns the principal that made the attempt.
func (p Payload) Actor() Actor {
	return p.actor
}

// OnBehalfOf returns the principal the actor acted for, the zero Ref when it acted for itself.
func (p Payload) OnBehalfOf() Ref {
	return p.onBehalfOf
}

// Subject returns the identity whose activity log the event belongs to, the zero Ref when none.
func (p Payload) Subject() Ref {
	return p.subject
}

// Target returns what the attempt touched.
func (p Payload) Target() Ref {
	return p.target
}

// Action returns what was attempted.
func (p Payload) Action() Action {
	return p.action
}

// Outcome returns how the attempt ended.
func (p Payload) Outcome() Outcome {
	return p.outcome
}

// Client returns where the request came from, the zero Client when its peer was not a public address.
func (p Payload) Client() Client {
	return p.client
}

// Details returns a copy of the facts specific to the event.
func (p Payload) Details() map[string]string {
	return maps.Clone(p.details)
}

// validate reports why the payload cannot be written as the event named name: an event that is not
// audited, a detail key the event does not allow, a target without a type, no action, an unknown
// result, or a denied or failed outcome without a reason code.
func (p Payload) validate(name string) error {
	allowed, audited := auditedEvents[name]
	if !audited {
		return errUnknownEvent
	}
	for key := range p.details {
		if !slices.Contains(allowed, key) {
			return errDetailNotAllowed
		}
	}
	if p.target.Type == "" {
		return errMissingTarget
	}
	if p.action == "" {
		return errMissingAction
	}
	switch p.outcome.Result {
	case ResultSucceeded:
		return nil
	case ResultDenied, ResultFailed:
		if p.outcome.Reason == "" {
			return errMissingReason
		}
		if !reasonPattern.MatchString(p.outcome.Reason) {
			return errInvalidReason
		}
		return nil
	default:
		return errUnknownResult
	}
}
