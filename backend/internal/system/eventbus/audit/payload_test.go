// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/suite"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type PayloadTestSuite struct {
	suite.Suite
}

func TestPayloadTestSuite(t *testing.T) {
	suite.Run(t, new(PayloadTestSuite))
}

func (s *PayloadTestSuite) TestAnUnauthenticatedContextIsAnonymous() {
	payload := NewPayload(context.Background(), testTarget, ActionUpdate, OutcomeSucceeded)

	s.Equal(Actor{Type: ActorTypeAnonymous}, payload.Actor())
	s.Equal(Ref{}, payload.OnBehalfOf())
	s.Equal(Client{}, payload.Client())
}

// The client is what the client middleware recorded for the request: the TCP peer, never a forwarded
// header, and its user agent.
func (s *PayloadTestSuite) TestTheClientComesFromTheContext() {
	r := newRequest(http.MethodPost, "/users", accessToken(s.T(), map[string]interface{}{"sub": testUserID}), "")
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.Header.Set("Forwarded", "for=203.0.113.7")

	payload := NewPayload(securedContext(s.T(), r), testTarget, ActionCreate, OutcomeSucceeded)

	s.Equal(Client{Address: testClientIP, UserAgent: testUserAgent}, payload.Client())
	s.Equal(Client{Address: "198.51.100.4", UserAgent: "other/2.0"},
		NewPayload(syscontext.WithClient(context.Background(), "198.51.100.4", "other/2.0"), testTarget,
			ActionCreate, OutcomeSucceeded).Client())
}

func (s *PayloadTestSuite) TestTheManagementAPIKeyIsTheSystemActor() {
	ctx := securedCallContext(s.T(), "", testAPIKey)

	payload := NewPayload(ctx, testTarget, ActionUpdate, OutcomeSucceeded)

	s.Equal(Actor{Type: ActorTypeSystem, ID: "management-api-key"}, payload.Actor())
}

func (s *PayloadTestSuite) TestTheActorComesFromTheVerifiedToken() {
	cases := []struct {
		name       string
		claims     map[string]interface{}
		actor      Actor
		onBehalfOf Ref
	}{
		{
			name:   "a user token carries no sub_type",
			claims: map[string]interface{}{"sub": testUserID},
			actor:  Actor{Type: ActorTypeUser, ID: testUserID},
		},
		{
			name:   "a client credentials token of an application",
			claims: map[string]interface{}{"sub": testAppID, "sub_type": "application"},
			actor:  Actor{Type: ActorTypeApplication, ID: testAppID},
		},
		{
			name:   "a client credentials token of an agent",
			claims: map[string]interface{}{"sub": testAgentID, "sub_type": "agent"},
			actor:  Actor{Type: ActorTypeAgent, ID: testAgentID},
		},
		{
			name:   "an unknown sub_type is not trusted",
			claims: map[string]interface{}{"sub": testUserID, "sub_type": "system"},
			actor:  Actor{Type: ActorTypeUser, ID: testUserID},
		},
		{
			name: "an act claim makes the actor an agent acting for the subject",
			claims: map[string]interface{}{
				"sub": testUserID,
				"act": map[string]interface{}{"sub": testAgentID},
			},
			actor:      Actor{Type: ActorTypeAgent, ID: testAgentID},
			onBehalfOf: Ref{Type: string(ActorTypeUser), ID: testUserID},
		},
		{
			name:   "a token whose sub is the key's subject is not the key",
			claims: map[string]interface{}{"sub": "management-api-key"},
			actor:  Actor{Type: ActorTypeUser, ID: "management-api-key"},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			ctx := securedCallContext(s.T(), accessToken(s.T(), tc.claims), "")

			payload := NewPayload(ctx, testTarget, ActionUpdate, OutcomeSucceeded)

			s.Equal(tc.actor, payload.Actor())
			s.Equal(tc.onBehalfOf, payload.OnBehalfOf())
		})
	}
}

func (s *PayloadTestSuite) TestValidate() {
	ctx := context.Background()
	cases := []struct {
		name    string
		event   string
		payload Payload
		err     error
	}{
		{"a succeeded outcome needs no reason", EventUserCreate,
			NewPayload(ctx, testTarget, ActionCreate, OutcomeSucceeded), nil},
		{"a denied outcome with a reason", EventUserCreate,
			NewPayload(ctx, testTarget, ActionCreate, OutcomeDenied(reasonForbidden)), nil},
		{"a failed outcome with a reason", EventUserDelete,
			NewPayload(ctx, testTarget, ActionDelete, OutcomeFailed("usr_1003")), nil},
		{"an application create", EventApplicationCreate,
			NewPayload(ctx, testApplication, ActionCreate, OutcomeSucceeded), nil},
		{"an application update", EventApplicationUpdate,
			NewPayload(ctx, testApplication, ActionUpdate, OutcomeFailed("app_1001")), nil},
		{"an application delete", EventApplicationDelete,
			NewPayload(ctx, testApplication, ActionDelete, OutcomeDenied(reasonForbidden)), nil},
		{"a role create", EventRoleCreate, NewPayload(ctx, testRole, ActionCreate, OutcomeSucceeded), nil},
		{"a role update", EventRoleUpdate, NewPayload(ctx, testRole, ActionUpdate, OutcomeFailed("rol_1003")), nil},
		{"a role delete", EventRoleDelete, NewPayload(ctx, testRole, ActionDelete, OutcomeSucceeded), nil},
		{"a role assignment add", EventRoleAssignmentAdd,
			NewPayload(ctx, testRole, ActionAdd, OutcomeSucceeded).WithSubject(testTarget), nil},
		{"a role assignment remove", EventRoleAssignmentRemove,
			NewPayload(ctx, testRole, ActionRemove, OutcomeDenied(reasonForbidden)), nil},
		{"a group create", EventGroupCreate, NewPayload(ctx, testGroup, ActionCreate, OutcomeSucceeded), nil},
		{"a group update", EventGroupUpdate, NewPayload(ctx, testGroup, ActionUpdate, OutcomeFailed("grp_1003")), nil},
		{"a group delete", EventGroupDelete, NewPayload(ctx, testGroup, ActionDelete, OutcomeSucceeded), nil},
		{"a group member add", EventGroupMemberAdd,
			NewPayload(ctx, testGroup, ActionAdd, OutcomeSucceeded).WithSubject(testTarget), nil},
		{"a group member remove", EventGroupMemberRemove,
			NewPayload(ctx, testGroup, ActionRemove, OutcomeFailed("grp_1003")), nil},
		{"a group member detail", EventGroupMemberAdd,
			NewPayload(ctx, testGroup, ActionAdd, OutcomeSucceeded).
				WithDetails(map[string]string{"name": "Librarians"}), errDetailNotAllowed},
		{"an application detail", EventApplicationUpdate,
			NewPayload(ctx, testApplication, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"client_secret": "s3cr3t"}), errDetailNotAllowed},
		{"an event that is not audited", "user.rename",
			NewPayload(ctx, testTarget, ActionUpdate, OutcomeSucceeded), errUnknownEvent},
		{"a detail the event does not allow", EventUserUpdate,
			NewPayload(ctx, testTarget, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"password": "s3cr3t"}), errDetailNotAllowed},
		{"a denied outcome without a reason", EventUserCreate,
			NewPayload(ctx, testTarget, ActionCreate, OutcomeDenied("")), errMissingReason},
		{"a failed outcome without a reason", EventUserCreate,
			NewPayload(ctx, testTarget, ActionCreate, OutcomeFailed("")), errMissingReason},
		{"a free text reason", EventUserCreate,
			NewPayload(ctx, testTarget, ActionCreate, OutcomeFailed("Password was wrong for alice")),
			errInvalidReason},
		{"an unknown result", EventUserCreate,
			NewPayload(ctx, testTarget, ActionCreate, Outcome{Result: "maybe"}), errUnknownResult},
		{"a target without a type", EventUserCreate,
			NewPayload(ctx, Ref{ID: testUserID}, ActionCreate, OutcomeSucceeded), errMissingTarget},
		{"an empty action", EventUserCreate,
			NewPayload(ctx, testTarget, "", OutcomeSucceeded), errMissingAction},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.ErrorIs(tc.payload.validate(tc.event), tc.err)
		})
	}
}

// Every ID is cut to its first 128 characters, and a multi-byte character is never split.
func (s *PayloadTestSuite) TestEveryIDIsCutTo128Characters() {
	long := strings.Repeat("ü", maxIDLength+20)
	exact := strings.Repeat("a", maxIDLength)
	ctx := securedCallContext(s.T(), accessToken(s.T(), map[string]interface{}{
		"sub": long,
		"act": map[string]interface{}{"sub": long},
	}), "")

	payload := NewPayload(ctx, Ref{Type: TargetTypeUser, ID: long}, ActionUpdate, OutcomeSucceeded).
		WithSubject(Ref{Type: TargetTypeUser, ID: long})

	ids := []string{payload.Target().ID, payload.Subject().ID, payload.OnBehalfOf().ID, payload.Actor().ID}
	for _, id := range ids {
		s.True(utf8.ValidString(id))
		s.Equal(strings.Repeat("ü", maxIDLength), id)
	}
	s.Equal(exact, NewPayload(ctx, Ref{Type: TargetTypeUser, ID: exact}, ActionUpdate, OutcomeSucceeded).Target().ID)
}

func (s *PayloadTestSuite) TestBuildersReturnIndependentCopies() {
	details := map[string]string{"reason": "requested"}
	base := NewPayload(context.Background(), testTarget, ActionCreate, OutcomeSucceeded)

	built := base.WithSubject(testTarget).WithDetails(details)
	details["reason"] = "changed"
	built.Details()["reason"] = "changed again"

	s.Equal(Ref{}, base.Subject(), "the receiver is unchanged")
	s.Nil(base.Details())
	s.Equal(testTarget, built.Subject())
	s.Equal(map[string]string{"reason": "requested"}, built.Details())
	s.Equal(testTarget, built.Target())
	s.Equal(ActionCreate, built.Action())
	s.Equal(OutcomeSucceeded, built.Outcome())
	s.Equal(CategoryAudit, built.Category())
}

func (s *PayloadTestSuite) TestOutcomeConstructors() {
	s.Equal(Outcome{Result: ResultSucceeded}, OutcomeSucceeded)
	s.Equal(Outcome{Result: ResultDenied, Reason: reasonForbidden}, OutcomeDenied(reasonForbidden))
	s.Equal(Outcome{Result: ResultFailed, Reason: "usr_1003"}, OutcomeFailed("usr_1003"))
}

// A change that succeeded or was denied on a named target has it as the subject; a failed change, or one
// that names no target, has none.
func (s *PayloadTestSuite) TestChangePayload() {
	unnamed := Ref{Type: TargetTypeApplication}
	cases := []struct {
		name    string
		target  Ref
		svcErr  *tidcommon.ServiceError
		outcome Outcome
		subject Ref
	}{
		{"no error", testApplication, nil, OutcomeSucceeded, testApplication},
		{"the access check refuses the caller", testApplication, &tidcommon.ErrorUnauthorized,
			OutcomeDenied("forbidden"), testApplication},
		{"a refused create names no target", unnamed, &tidcommon.ErrorUnauthorized,
			OutcomeDenied("forbidden"), Ref{}},
		{"the grant check refuses the caller", testGroup, &sysauthz.ErrorGrantNotPermitted,
			OutcomeDenied("forbidden"), testGroup},
		{"a client error", testApplication, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType,
			Code: "APP-1003"}, OutcomeFailed("app_1003"), Ref{}},
		{"a server error", testTarget, &tidcommon.InternalServerError, OutcomeFailed("sse_5000"), Ref{}},
		{"a code that makes no reason", testTarget, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType,
			Code: ""}, OutcomeFailed("server_error"), Ref{}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			payload := ChangePayload(context.Background(), tc.target, ActionUpdate, tc.svcErr)

			s.Equal(tc.outcome, payload.Outcome())
			s.Equal(tc.target, payload.Target())
			s.Equal(tc.subject, payload.Subject())
			s.Equal(ActionUpdate, payload.Action())
		})
	}
}

// A membership change that succeeded is one payload per distinct member, each with the member as subject,
// so the log names everyone who gained or lost the membership. One that did not succeed is one payload
// for the attempt, with the target only: a call may name several members.
func (s *PayloadTestSuite) TestMembershipPayloads() {
	agent := Ref{Type: TargetTypeAgent, ID: testAgentID}
	cases := []struct {
		name     string
		members  []Ref
		svcErr   *tidcommon.ServiceError
		outcome  Outcome
		subjects []Ref
	}{
		{"one member added", []Ref{testTarget}, nil, OutcomeSucceeded, []Ref{testTarget}},
		{"each member once", []Ref{testTarget, agent, testTarget}, nil, OutcomeSucceeded,
			[]Ref{testTarget, agent}},
		{"no member named", nil, nil, OutcomeSucceeded, []Ref{}},
		{"the grant check refuses the caller", []Ref{testTarget, agent}, &sysauthz.ErrorGrantNotPermitted,
			OutcomeDenied("forbidden"), []Ref{{}}},
		{"the access check refuses the caller", []Ref{testTarget}, &tidcommon.ErrorUnauthorized,
			OutcomeDenied("forbidden"), []Ref{{}}},
		{"the change fails", []Ref{testTarget, agent}, &tidcommon.InternalServerError,
			OutcomeFailed("sse_5000"), []Ref{{}}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			payloads := MembershipPayloads(context.Background(), testGroup, tc.members, ActionAdd, tc.svcErr)

			subjects := make([]Ref, 0, len(payloads))
			for _, payload := range payloads {
				s.Equal(testGroup, payload.Target())
				s.Equal(ActionAdd, payload.Action())
				s.Equal(tc.outcome, payload.Outcome())
				s.Empty(payload.Details())
				subjects = append(subjects, payload.Subject())
			}
			s.Equal(tc.subjects, subjects)
		})
	}
}

// A principal is named by the type its entity category or a group maps to.
func (s *PayloadTestSuite) TestPrincipalRef() {
	for principalType, targetType := range map[string]string{
		"user":  TargetTypeUser,
		"app":   TargetTypeApplication,
		"agent": TargetTypeAgent,
		"group": TargetTypeGroup,
	} {
		s.Run(principalType, func() {
			s.Equal(Ref{Type: targetType, ID: testUserID}, PrincipalRef(principalType, testUserID))
		})
	}
}
