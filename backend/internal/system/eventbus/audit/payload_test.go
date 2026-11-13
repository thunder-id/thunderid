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
	"github.com/thunder-id/thunderid/tests/testhelpers"
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

// A request from a private or loopback peer, such as a proxy or a local caller, has no client: neither
// its address nor its user agent.
func (s *PayloadTestSuite) TestANonPublicPeerHasNoClient() {
	r := newRequest(http.MethodPost, "/users", accessToken(s.T(), map[string]interface{}{"sub": testUserID}), "")
	r.RemoteAddr = "127.0.0.1:52814"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")

	payload := NewPayload(securedContext(s.T(), r), testTarget, ActionCreate, OutcomeSucceeded)

	s.Equal(Client{}, payload.Client())
	s.Equal(Actor{Type: ActorTypeUser, ID: testUserID}, payload.Actor(), "the actor is resolved as before")
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
	const testTypeID = "01900000-0000-7000-8000-000000000033"
	ctx := context.Background()
	testNotificationSender := Ref{Type: TargetTypeNotificationSender, ID: "0196a1b2-0000-7000-8000-000000000001"}
	testNotificationTemplate := Ref{Type: TargetTypeNotificationTemplate, ID: "0196a1b2-0000-7000-8000-000000000002"}
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
		{"a sharing policy of a user", EventUserSharingPolicyCreate,
			NewPayload(ctx, Ref{Type: TargetTypeUserSharingPolicy}, ActionCreate, OutcomeSucceeded), nil},
		{"a sharing policy of an application", EventApplicationSharingPolicyDelete,
			NewPayload(ctx, Ref{Type: TargetTypeApplicationSharingPolicy}, ActionDelete, OutcomeSucceeded), nil},
		{"an organization unit update", EventOUUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeOU}, ActionUpdate, OutcomeFailed("ou_1003")), nil},
		{"a project delete", EventProjectDelete,
			NewPayload(ctx, Ref{Type: TargetTypeProject}, ActionDelete, OutcomeFailed("prj_1013")), nil},
		{"an application detail", EventApplicationUpdate,
			NewPayload(ctx, testApplication, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"client_secret": "s3cr3t"}), errDetailNotAllowed},
		{"a flow create", EventFlowCreate,
			NewPayload(ctx, Ref{Type: TargetTypeFlow, ID: "flow-1"}, ActionCreate, OutcomeSucceeded), nil},
		{"a flow update", EventFlowUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeFlow, ID: "flow-1"}, ActionUpdate, OutcomeFailed("flm_1003")), nil},
		{"a flow delete", EventFlowDelete,
			NewPayload(ctx, Ref{Type: TargetTypeFlow, ID: "flow-1"}, ActionDelete, OutcomeSucceeded), nil},
		{"a flow restore", EventFlowRestore,
			NewPayload(ctx, Ref{Type: TargetTypeFlow, ID: "flow-1"}, ActionRestore, OutcomeFailed("flm_1008")), nil},
		{"a flow detail", EventFlowUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeFlow, ID: "flow-1"}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"name": "Sign In"}), errDetailNotAllowed},
		{"a server config update", EventServerConfigUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeServerConfig, ID: "cors"}, ActionUpdate, OutcomeSucceeded), nil},
		{"a server config value", EventServerConfigUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeServerConfig, ID: "cors"}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"value": "https://app.example.com"}), errDetailNotAllowed},
		{"an agent create", EventAgentCreate,
			NewPayload(ctx, Ref{Type: TargetTypeAgent, ID: testAgentID}, ActionCreate, OutcomeSucceeded), nil},
		{"an agent update", EventAgentUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeAgent, ID: testAgentID}, ActionUpdate,
				OutcomeDenied(reasonForbidden)), nil},
		{"an agent delete", EventAgentDelete,
			NewPayload(ctx, Ref{Type: TargetTypeAgent, ID: testAgentID}, ActionDelete,
				OutcomeFailed("agt_1004")), nil},
		{"an agent detail", EventAgentUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeAgent, ID: testAgentID}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"client_secret": "s3cr3t"}), errDetailNotAllowed},
		{"an agent credential update", EventAgentCredentialUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeAgent, ID: testAgentID}, ActionUpdate, OutcomeSucceeded), nil},
		{"an agent credential detail", EventAgentCredentialUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeAgent, ID: testAgentID}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"credential_type": "password"}), errDetailNotAllowed},
		{"a user credential update", EventUserCredentialUpdate,
			NewPayload(ctx, testTarget, ActionUpdate, OutcomeDenied(reasonForbidden)), nil},
		{"a user credential detail", EventUserCredentialUpdate,
			NewPayload(ctx, testTarget, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"credential_type": "password"}), errDetailNotAllowed},
		{"a user credential enroll without a resolved user", EventUserCredentialEnroll,
			NewPayload(ctx, Ref{Type: TargetTypeUser}, ActionEnroll, OutcomeFailed("authn_mgr_1002")), nil},
		{"a user credential enroll refused by a suspension", EventUserCredentialEnroll,
			NewPayload(ctx, Ref{Type: TargetTypeUser}, ActionEnroll, OutcomeDenied("account_suspended")), nil},
		{"an agent type create", EventAgentTypeCreate,
			NewPayload(ctx, Ref{Type: TargetTypeAgentType}, ActionCreate, OutcomeFailed("usrs_1003")), nil},
		{"an agent type update", EventAgentTypeUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeAgentType, ID: testTypeID}, ActionUpdate, OutcomeSucceeded),
			nil},
		{"an agent type delete", EventAgentTypeDelete,
			NewPayload(ctx, Ref{Type: TargetTypeAgentType, ID: testTypeID}, ActionDelete,
				OutcomeFailed("usrs_1015")), nil},
		{"a user type create", EventUserTypeCreate,
			NewPayload(ctx, Ref{Type: TargetTypeUserType, ID: testTypeID}, ActionCreate, OutcomeSucceeded), nil},
		{"a user type update", EventUserTypeUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeUserType, ID: testTypeID}, ActionUpdate,
				OutcomeDenied(reasonForbidden)), nil},
		{"a user type delete", EventUserTypeDelete,
			NewPayload(ctx, Ref{Type: TargetTypeUserType, ID: testTypeID}, ActionDelete, OutcomeSucceeded), nil},
		{"a user type detail", EventUserTypeUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeUserType, ID: testTypeID}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"schema": "{}"}), errDetailNotAllowed},
		{"a refused client authentication", EventClientAuthenticate,
			NewPayload(ctx, testApplication, ActionAuthenticate, OutcomeDenied("invalid_client")), nil},
		{"a token request with its grant type", EventTokenIssue,
			NewPayload(ctx, testToken, ActionIssue, OutcomeSucceeded).
				WithDetails(map[string]string{DetailGrantType: "client_credentials"}), nil},
		{"a token request detail other than the grant type", EventTokenIssue,
			NewPayload(ctx, testToken, ActionIssue, OutcomeSucceeded).
				WithDetails(map[string]string{"refresh_token": "rt-value"}), errDetailNotAllowed},
		{"a token revocation", EventTokenRevoke,
			NewPayload(ctx, testToken, ActionRevoke, OutcomeDenied("invalid_grant")), nil},
		{"a token revocation detail", EventTokenRevoke,
			NewPayload(ctx, testToken, ActionRevoke, OutcomeSucceeded).
				WithDetails(map[string]string{DetailGrantType: "refresh_token"}), errDetailNotAllowed},
		{"a client authentication detail", EventClientAuthenticate,
			NewPayload(ctx, testApplication, ActionAuthenticate, OutcomeFailed("invalid_request")).
				WithDetails(map[string]string{"client_secret": "s3cr3t"}), errDetailNotAllowed},
		{"a gateway apply", EventGatewayApply,
			NewPayload(ctx, testGateway, ActionApply, OutcomeSucceeded), nil},
		{"a gateway revert", EventGatewayRevert,
			NewPayload(ctx, testGateway, ActionRevert, OutcomeFailed("gtw_1016")), nil},
		{"a gateway apply detail", EventGatewayApply,
			NewPayload(ctx, testGateway, ActionApply, OutcomeSucceeded).
				WithDetails(map[string]string{"version": "3f2a9c1"}), errDetailNotAllowed},
		{"a layout create", EventLayoutCreate,
			NewPayload(ctx, Ref{Type: TargetTypeLayout, ID: "layout-1"}, ActionCreate, OutcomeSucceeded), nil},
		{"a layout update", EventLayoutUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeLayout, ID: "layout-1"}, ActionUpdate, OutcomeFailed("lay_1003")), nil},
		{"a layout delete", EventLayoutDelete,
			NewPayload(ctx, Ref{Type: TargetTypeLayout, ID: "layout-1"}, ActionDelete, OutcomeSucceeded), nil},
		{"a layout detail", EventLayoutUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeLayout, ID: "layout-1"}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"display_name": "Sign In"}), errDetailNotAllowed},
		{"a theme create", EventThemeCreate,
			NewPayload(ctx, Ref{Type: TargetTypeTheme, ID: "theme-1"}, ActionCreate, OutcomeSucceeded), nil},
		{"a theme update", EventThemeUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeTheme, ID: "theme-1"}, ActionUpdate, OutcomeFailed("thm_1003")), nil},
		{"a theme delete", EventThemeDelete,
			NewPayload(ctx, Ref{Type: TargetTypeTheme, ID: "theme-1"}, ActionDelete, OutcomeSucceeded), nil},
		{"a theme detail", EventThemeUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeTheme, ID: "theme-1"}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"theme": "{}"}), errDetailNotAllowed},
		{"a connection create", EventConnectionCreate,
			NewPayload(ctx, Ref{Type: TargetTypeConnection, ID: "conn-1"}, ActionCreate, OutcomeSucceeded), nil},
		{"a connection update", EventConnectionUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeConnection, ID: "conn-1"}, ActionUpdate, OutcomeFailed("idp_1001")),
			nil},
		{"a connection delete", EventConnectionDelete,
			NewPayload(ctx, Ref{Type: TargetTypeConnection, ID: "conn-1"}, ActionDelete, OutcomeSucceeded), nil},
		{"a connection property", EventConnectionUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeConnection, ID: "conn-1"}, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"client_secret": "s3cr3t"}), errDetailNotAllowed},
		{"a resource server create", EventResourceServerCreate,
			NewPayload(ctx, Ref{Type: TargetTypeResourceServer, ID: "rs-1"}, ActionCreate, OutcomeSucceeded), nil},
		{"a resource server update", EventResourceServerUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeResourceServer, ID: "rs-1"}, ActionUpdate,
				OutcomeFailed("res_1003")), nil},
		{"a resource server delete", EventResourceServerDelete,
			NewPayload(ctx, Ref{Type: TargetTypeResourceServer, ID: "rs-1"}, ActionDelete, OutcomeSucceeded), nil},
		{"a resource create", EventResourceServerResourceCreate,
			NewPayload(ctx, Ref{Type: TargetTypeResource, ID: "res-1"}, ActionCreate, OutcomeSucceeded), nil},
		{"a resource update", EventResourceServerResourceUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeResource, ID: "res-1"}, ActionUpdate, OutcomeSucceeded), nil},
		{"a resource delete", EventResourceServerResourceDelete,
			NewPayload(ctx, Ref{Type: TargetTypeResource, ID: "res-1"}, ActionDelete, OutcomeSucceeded), nil},
		{"an action create", EventResourceServerActionCreate,
			NewPayload(ctx, Ref{Type: TargetTypeAction, ID: "act-1"}, ActionCreate, OutcomeSucceeded), nil},
		{"an action update", EventResourceServerActionUpdate,
			NewPayload(ctx, Ref{Type: TargetTypeAction, ID: "act-1"}, ActionUpdate, OutcomeSucceeded), nil},
		{"an action delete", EventResourceServerActionDelete,
			NewPayload(ctx, Ref{Type: TargetTypeAction, ID: "act-1"}, ActionDelete, OutcomeSucceeded), nil},
		{"an action permission", EventResourceServerActionCreate,
			NewPayload(ctx, Ref{Type: TargetTypeAction, ID: "act-1"}, ActionCreate, OutcomeSucceeded).
				WithDetails(map[string]string{"permission": "orders:read"}), errDetailNotAllowed},
		{"an import of the configuration, which has no ID", EventConfigurationImport,
			NewPayload(ctx, testConfiguration, ActionImport, OutcomeFailed(reasonItemFailed)).
				WithDetails(map[string]string{DetailDryRun: "true"}), nil},
		{"a delete of a declarative resource file", EventConfigurationDelete,
			NewPayload(ctx, testConfiguration, ActionDelete, OutcomeFailed("imp_1001")), nil},
		{"an export of the configuration", EventConfigurationExport,
			NewPayload(ctx, testConfiguration, ActionExport, OutcomeSucceeded), nil},
		{"an export carries no dry run", EventConfigurationExport,
			NewPayload(ctx, testConfiguration, ActionExport, OutcomeSucceeded).
				WithDetails(map[string]string{DetailDryRun: "true"}), errDetailNotAllowed},
		{"a notification sender create", EventNotificationSenderCreate,
			NewPayload(ctx, testNotificationSender, ActionCreate, OutcomeSucceeded), nil},
		{"a notification sender update", EventNotificationSenderUpdate,
			NewPayload(ctx, testNotificationSender, ActionUpdate, OutcomeFailed("mns_1005")), nil},
		{"a notification sender delete", EventNotificationSenderDelete,
			NewPayload(ctx, testNotificationSender, ActionDelete, OutcomeSucceeded), nil},
		{"a notification sender property", EventNotificationSenderUpdate,
			NewPayload(ctx, testNotificationSender, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"auth_token": "s3cr3t"}), errDetailNotAllowed},
		{"a notification template create", EventNotificationTemplateCreate,
			NewPayload(ctx, testNotificationTemplate, ActionCreate, OutcomeSucceeded), nil},
		{"a notification template update", EventNotificationTemplateUpdate,
			NewPayload(ctx, testNotificationTemplate, ActionUpdate, OutcomeFailed("ntm_1004")), nil},
		{"a notification template delete", EventNotificationTemplateDelete,
			NewPayload(ctx, testNotificationTemplate, ActionDelete, OutcomeSucceeded), nil},
		{"a notification template content", EventNotificationTemplateUpdate,
			NewPayload(ctx, testNotificationTemplate, ActionUpdate, OutcomeSucceeded).
				WithDetails(map[string]string{"subject": "Welcome"}), errDetailNotAllowed},
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

// An OAuth endpoint names the actor itself, so a bearer token the request also carries decides neither
// the actor nor the principal its act claim names.
func (s *PayloadTestSuite) TestAnExplicitActorReplacesTheVerifiedToken() {
	ctx := securedCallContext(s.T(), accessToken(s.T(), map[string]interface{}{
		"sub": testUserID,
		"act": map[string]interface{}{"sub": testAgentID},
	}), "")
	resolved := NewPayload(ctx, testToken, ActionIssue, OutcomeSucceeded)
	s.Require().Equal(Actor{Type: ActorTypeAgent, ID: testAgentID}, resolved.Actor())
	s.Require().Equal(Ref{Type: TargetTypeUser, ID: testUserID}, resolved.OnBehalfOf())

	client := resolved.WithActor(Actor{Type: ActorTypeApplication, ID: testAppID})
	anonymous := resolved.WithActor(Actor{Type: ActorTypeAnonymous})

	s.Equal(Actor{Type: ActorTypeApplication, ID: testAppID}, client.Actor())
	s.Equal(Ref{}, client.OnBehalfOf())
	s.Equal(Actor{Type: ActorTypeAnonymous}, anonymous.Actor())
	s.Equal(Ref{}, anonymous.OnBehalfOf())
	s.Equal(Actor{Type: ActorTypeAgent, ID: testAgentID}, resolved.Actor(), "the receiver is unchanged")
	s.Equal(strings.Repeat("ü", maxIDLength),
		resolved.WithActor(Actor{Type: ActorTypeApplication, ID: strings.Repeat("ü", maxIDLength+1)}).Actor().ID)
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

// A change to a record kept for a resource, such as a sharing policy, has the resource as the subject when
// it succeeded or was denied, even when the record does not exist yet; a failed change has none.
func (s *PayloadTestSuite) TestChangePayloadWithSubject() {
	policy := Ref{Type: TargetTypeUserSharingPolicy, ID: "policy-1"}
	unnamed := Ref{Type: TargetTypeUserSharingPolicy}
	cases := []struct {
		name    string
		target  Ref
		svcErr  *tidcommon.ServiceError
		outcome Outcome
		subject Ref
	}{
		{"no error", policy, nil, OutcomeSucceeded, testTarget},
		{"a refused create names no record", unnamed, &tidcommon.ErrorUnauthorized, OutcomeDenied("forbidden"),
			testTarget},
		{"a client error", policy, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "USR-1003"},
			OutcomeFailed("usr_1003"), Ref{}},
		{"a server error on a create", unnamed, &tidcommon.InternalServerError, OutcomeFailed("sse_5000"), Ref{}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			payload := ChangePayloadWithSubject(context.Background(), tc.target, testTarget, ActionCreate,
				tc.svcErr)

			s.Equal(tc.outcome, payload.Outcome())
			s.Equal(tc.target, payload.Target())
			s.Equal(tc.subject, payload.Subject())
			s.Equal(ActionCreate, payload.Action())
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
			s.Equal(Ref{Type: targetType, ID: testUserID}, PrincipalRef(context.Background(), principalType,
				testUserID))
		})
	}
}

// A principal type with no target type is recorded as it is, and logged without the principal's ID.
func (s *PayloadTestSuite) TestPrincipalRef_KeepsAndLogsAnUnmappedType() {
	var ref Ref
	logged := testhelpers.CaptureLogs(s.T(), "ERROR", func() {
		ref = PrincipalRef(context.Background(), "workload", testUserID)
	})

	s.Equal(Ref{Type: "workload", ID: testUserID}, ref)
	s.Contains(logged, "Principal type has no audit target type")
	s.Contains(logged, "workload")
	s.NotContains(logged, testUserID)
}

// A request that completed with an item failed is recorded as failed with item_failed; any other is
// recorded as a change is. A target without an ID is never the subject.
func (s *PayloadTestSuite) TestBatchPayload() {
	cases := []struct {
		name       string
		svcErr     *tidcommon.ServiceError
		itemFailed bool
		outcome    Outcome
	}{
		{"every item succeeded", nil, false, OutcomeSucceeded},
		{"an item failed", nil, true, OutcomeFailed("item_failed")},
		{"the request was refused", &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "IMP-1001"},
			false, OutcomeFailed("imp_1001")},
		{"the access check refused the caller", &tidcommon.ErrorUnauthorized, false, OutcomeDenied("forbidden")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			payload := BatchPayload(context.Background(), testConfiguration, ActionImport, tc.svcErr,
				tc.itemFailed)

			s.Equal(tc.outcome, payload.Outcome())
			s.Equal(testConfiguration, payload.Target())
			s.Equal(Ref{}, payload.Subject())
			s.Equal(ActionImport, payload.Action())
			s.Empty(payload.Details())
		})
	}
}
