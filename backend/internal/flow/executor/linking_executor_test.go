// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type LinkingExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory   *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerMock
	mockBaseExecutor  *coremock.ExecutorInterfaceMock
	executor          *linkingExecutor
}

func TestLinkingExecutorSuite(t *testing.T) {
	suite.Run(t, new(LinkingExecutorTestSuite))
}

func (suite *LinkingExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameLinking,
		providers.ExecutorTypeUtility,
		[]providers.Input{},
		[]providers.Input{}, mock.Anything).Return(suite.mockBaseExecutor)

	suite.executor = newLinkingExecutor(suite.mockFlowFactory, suite.mockAuthnProvider)
}

// linkingCandidateID is the account every test here matches on, and linkingOtherCandidateID the
// second account the tests with several candidates match on.
const (
	linkingCandidateID      = "user-1"
	linkingOtherCandidateID = "user-2"
)

// matchedCandidate is linkingCandidateID as ResolveFederatedCandidates returns it, found on matched.
func matchedCandidate(matched map[string]string) *providers.FederatedCandidates {
	return &providers.FederatedCandidates{
		EntityIDs:         []string{linkingCandidateID},
		MatchedAttributes: matched,
	}
}

// verificationPending is the RuntimeData of a flow that has been through this node once, named
// linkingCandidateID, and is coming back from the verification step it asked for.
func verificationPending() map[string]string {
	return verificationPendingFor(`["` + linkingCandidateID + `"]`)
}

// verificationPendingFor is verificationPending with the candidates stored as given.
func verificationPendingFor(candidates string) map[string]string {
	return map[string]string{
		common.RuntimeKeyLinkingCandidateUserIDs:      candidates,
		common.RuntimeKeyLinkingVerificationRequested: dataValueTrue,
	}
}

// returningFromVerification is runtime as the first pass leaves it for verification: saved in the
// checkpoint with pendingAuthUser, and with the external identity cleared.
func returningFromVerification(runtime map[string]string) map[string]string {
	return returningFromVerificationAs(runtime, pendingAuthUser())
}

// returningFromVerificationAs is returningFromVerification with authUser saved in the checkpoint.
func returningFromVerificationAs(runtime map[string]string, authUser providers.AuthUser) map[string]string {
	writes := map[string]string{}
	if err := saveLinkingCheckpoint(runtime, writes, linkingCheckpoint{AuthUser: authUser}); err != nil {
		panic(err)
	}
	maps.Copy(runtime, writes)
	return runtime
}

// authUserWithToken is an AuthUser holding one provider state whose tokens are token, so tests can
// tell AuthUsers apart in mock expectations.
func authUserWithToken(token string) providers.AuthUser {
	var authUser providers.AuthUser
	if err := authUser.UnmarshalJSON([]byte(
		`{"default":{"entityReferenceToken":"` + token + `","attributeToken":"` + token + `"}}`)); err != nil {
		panic(err)
	}
	return authUser
}

// pendingAuthUser is a federated identity with no link: it reads as authenticated and resolves to
// nobody. verifiedAuthUser and otherAuthUser are whoever a verification step authenticated.
func pendingAuthUser() providers.AuthUser  { return authUserWithToken("pending") }
func verifiedAuthUser() providers.AuthUser { return authUserWithToken("verified") }
func otherAuthUser() providers.AuthUser    { return authUserWithToken("other") }

// expectEntity has GetEntityReference resolve authUser to entityID.
func (suite *LinkingExecutorTestSuite) expectEntity(authUser providers.AuthUser, entityID string) {
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, &providers.EntityReference{EntityID: entityID}, (*tidcommon.ServiceError)(nil))
}

// expectNobody has GetEntityReference answer that authUser resolves to nobody, as it does for a
// pending federated identity.
func (suite *LinkingExecutorTestSuite) expectNobody(authUser providers.AuthUser) {
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, (*providers.EntityReference)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "AUTHN-MGR-1007"})
}

// advance merges resp into ctx the way the engine does between nodes: RuntimeData entries are added
// or replaced, never removed, and an authenticated AuthUser replaces the one in the context. A
// forwarded action type does not outlive the hop it was raised for.
func advance(ctx *providers.NodeContext, resp *providers.ExecutorResponse) {
	if ctx.RuntimeData == nil {
		ctx.RuntimeData = map[string]string{}
	}
	maps.Copy(ctx.RuntimeData, resp.RuntimeData)
	if resp.AuthUser.IsAuthenticated() {
		ctx.AuthUser = resp.AuthUser
	}
	ctx.ForwardedData = nil
}

// assertCycleEnded checks that a settling pass cleared the verification flag and the checkpoint.
func (suite *LinkingExecutorTestSuite) assertCycleEnded(resp *providers.ExecutorResponse) {
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyLinkingVerificationRequested])
	assert.Contains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingVerificationRequested)
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyLinkingCheckpoint])
	assert.Contains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingCheckpoint)
}

// linkingAction is the ForwardedData a prompt leaves behind when the End-User picks one of its
// actions. An empty type stands for a pass that raised no action at all.
func linkingAction(actionType common.ActionType) map[string]interface{} {
	return map[string]interface{}{common.ForwardedDataKeyActionType: string(actionType)}
}

// A returning user whose link the authn provider already resolved: the identity is linked already,
// so there is nothing to decide and nothing to record. Nothing is matched on attributes and nothing
// is written, which the unstubbed ResolveFederatedCandidates and LinkFederatedIdentity assert on
// their own.
func (suite *LinkingExecutorTestSuite) TestExecute_AuthenticatedUserCompletes() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    authenticatedAuthUser(),
	}
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{EntityID: "user-1"},
			(*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateExists, resp.RuntimeData[common.RuntimeKeyEntityState])
}

// A pending federated identity fills both sides of the AuthUser, so it looks authenticated and
// resolves to nobody. That is the "nobody yet" answer, not a failure, and matching carries on.
func (suite *LinkingExecutorTestSuite) TestExecute_PendingFederatedIdentityIsNotAuthenticated() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    authenticatedAuthUser(),
	}
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (*providers.EntityReference)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "AUTHN-MGR-1007"})
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return((*providers.FederatedCandidates)(nil), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyEntityState)
}

// Nothing matched: a brand new federated user, for provisioning to create.
func (suite *LinkingExecutorTestSuite) TestExecute_NoCandidateCompletesAsNewUser() {
	ctx := &providers.NodeContext{ExecutionID: "flow-1", FlowType: providers.FlowTypeAuthentication}
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return((*providers.FederatedCandidates)(nil), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingCandidateUserIDs)
}

// Nobody authenticated at all: GetEntityReference must not be called, and being unauthenticated is
// not a failure. Not stubbing it means an unexpected call fails the test on its own.
func (suite *LinkingExecutorTestSuite) TestExecute_Unauthenticated_DoesNotCallGetEntityReference() {
	ctx := &providers.NodeContext{ExecutionID: "flow-1", FlowType: providers.FlowTypeAuthentication}
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return((*providers.FederatedCandidates)(nil), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotEqual(suite.T(), providers.ExecFailure, resp.Status)
	assert.Nil(suite.T(), resp.Error)
}

// A lookup that fails fails the node: one attribute matching several accounts returns no ids to
// verify against, and continuing would provision another.
func (suite *LinkingExecutorTestSuite) TestExecute_CandidateLookupFailureFails() {
	ctx := &providers.NodeContext{ExecutionID: "flow-1", FlowType: providers.FlowTypeAuthentication}
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return((*providers.FederatedCandidates)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: "AUTHN-MGR-1009"})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrFailedToIdentifyEntity.Code, resp.Error.Code)
}

// A candidate nobody has proved yet: go incomplete so the node forwards to what the flow author
// wired on onIncomplete, naming the account it is offering for the prompt there to render. A match
// is never promoted on its own.
func (suite *LinkingExecutorTestSuite) TestExecute_UnprovenCandidateRequestsVerification() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
	}
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return(matchedCandidate(map[string]string{"email": "jane@example.com"}), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.JSONEq(suite.T(), `["user-1"]`, resp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyLinkingVerificationRequested])
	assert.JSONEq(suite.T(), `[{"label":"Email","value":"jane@example.com"}]`,
		resp.AdditionalData[common.DataLinkingPromptDetails])
}

// Attributes naming different accounts send them all to verification. The prompt shows the values
// that matched and nothing about how many accounts they matched.
func (suite *LinkingExecutorTestSuite) TestExecute_SeveralCandidatesRequestVerification() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
	}
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return(&providers.FederatedCandidates{
			EntityIDs:         []string{linkingCandidateID, linkingOtherCandidateID},
			MatchedAttributes: map[string]string{"email": "jane@example.com", "username": "jane"},
		}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.JSONEq(suite.T(), `["user-1","user-2"]`, resp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])
	assert.JSONEq(suite.T(),
		`[{"label":"Email","value":"jane@example.com"},{"label":"Username","value":"jane"}]`,
		resp.AdditionalData[common.DataLinkingPromptDetails])
}

// A connection matching on more than one attribute publishes a row for each, labeled by the
// attribute name and ordered by it, since the matched values arrive as a map.
func (suite *LinkingExecutorTestSuite) TestExecute_PublishesEveryMatchedAttributeInOrder() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
	}
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return(matchedCandidate(map[string]string{"mobileNumber": "+1555", "email": "jane@example.com"}),
			(*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(),
		`[{"label":"Email","value":"jane@example.com"},{"label":"MobileNumber","value":"+1555"}]`,
		resp.AdditionalData[common.DataLinkingPromptDetails])
}

// The candidate this node named proved itself. The verification step replaced the pending federated
// identity, so nothing is matched again: the unstubbed ResolveFederatedCandidates asserts that.
func (suite *LinkingExecutorTestSuite) TestExecute_VerifiedByTheCandidateCompletes() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    authenticatedAuthUser(),
		RuntimeData: returningFromVerification(verificationPending()),
	}
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{EntityID: "user-1"},
			(*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateExists, resp.RuntimeData[common.RuntimeKeyEntityState])
}

// Re-entry after a verification step that authenticated somebody else. Federated links are indexed
// by connection alone, so an offered federated branch proves an account at that connection, not
// which one. Picking the wrong account is the End-User's to retry: the prompt is offered again with
// the AuthUser put back to the pending identity, and the cycle stays armed.
func (suite *LinkingExecutorTestSuite) TestExecute_ReEntryByAnotherAccountRetries() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    otherAuthUser(),
		RuntimeData: returningFromVerification(withFederatedIdentity(verificationPending())),
	}
	suite.expectEntity(otherAuthUser(), "user-2")
	suite.expectNobody(pendingAuthUser())

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrCandidateNotVerified.Code, resp.Error.Code)
	assert.Equal(suite.T(), pendingAuthUser(), resp.AuthUser)
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyExternalIdentity])
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingVerificationRequested)
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingCheckpoint)
	assert.NotEmpty(suite.T(), ctx.RuntimeData[common.RuntimeKeyLinkingCheckpoint])
	assert.Equal(suite.T(), dataValueTrue, ctx.RuntimeData[common.RuntimeKeyLinkingVerificationRequested])
}

// The saved AuthUser is handed back only when it is the pending identity: one the engine would not
// adopt leaves the account that verified signed in, and one that names an entity, or whose lookup
// fails on the server, would sign the flow in as somebody. Each ends the cycle with a failure.
func (suite *LinkingExecutorTestSuite) TestExecute_UnrestorableSavedAuthUserFails() {
	for name, tc := range map[string]struct {
		saved  providers.AuthUser
		lookup *providers.EntityReference
		svcErr *tidcommon.ServiceError
	}{
		"not authenticated": {saved: providers.AuthUser{}},
		"names an entity":   {saved: pendingAuthUser(), lookup: &providers.EntityReference{EntityID: "user-9"}},
		"server error":      {saved: pendingAuthUser(), svcErr: &tidcommon.InternalServerError},
	} {
		suite.Run(name, func() {
			mockAuthnProvider := managermock.NewAuthnProviderManagerMock(suite.T())
			executor := newLinkingExecutor(suite.mockFlowFactory, mockAuthnProvider)
			ctx := &providers.NodeContext{
				ExecutionID: "flow-1",
				FlowType:    providers.FlowTypeAuthentication,
				AuthUser:    otherAuthUser(),
				RuntimeData: returningFromVerificationAs(withFederatedIdentity(verificationPending()), tc.saved),
			}
			mockAuthnProvider.On("GetEntityReference", mock.Anything, otherAuthUser()).
				Return(otherAuthUser(), &providers.EntityReference{EntityID: "user-2"},
					(*tidcommon.ServiceError)(nil))
			if tc.saved.IsAuthenticated() {
				mockAuthnProvider.On("GetEntityReference", mock.Anything, tc.saved).
					Return(tc.saved, tc.lookup, tc.svcErr)
			}

			resp, err := executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
			assert.Equal(suite.T(), ErrCandidateNotVerified.Code, resp.Error.Code)
			assert.Equal(suite.T(), otherAuthUser(), resp.AuthUser)
			suite.assertCycleEnded(resp)
			assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingCandidateUserIDs)
		})
	}
}

// With several candidates, whichever one the End-User verifies is the one linked.
func (suite *LinkingExecutorTestSuite) TestExecute_VerifiedByAnyCandidate_IsLinked() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    authenticatedAuthUser(),
		RuntimeData: returningFromVerification(withFederatedIdentity(
			verificationPendingFor(`["` + linkingCandidateID + `","` + linkingOtherCandidateID + `"]`))),
	}
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{EntityID: linkingOtherCandidateID},
			(*tidcommon.ServiceError)(nil))
	suite.expectLink(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateExists, resp.RuntimeData[common.RuntimeKeyEntityState])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

// An account outside the list is not linked however many candidates there are, and neither is one
// checked against a list that does not decode: nothing in it can be matched. Both get the retry.
func (suite *LinkingExecutorTestSuite) TestExecute_ReEntryByNonCandidateRetries() {
	for name, candidates := range map[string]string{
		"several":   `["` + linkingCandidateID + `","` + linkingOtherCandidateID + `"]`,
		"malformed": linkingCandidateID,
	} {
		suite.Run(name, func() {
			mockAuthnProvider := managermock.NewAuthnProviderManagerMock(suite.T())
			executor := newLinkingExecutor(suite.mockFlowFactory, mockAuthnProvider)
			ctx := &providers.NodeContext{
				ExecutionID: "flow-1",
				FlowType:    providers.FlowTypeAuthentication,
				AuthUser:    otherAuthUser(),
				RuntimeData: returningFromVerification(withFederatedIdentity(verificationPendingFor(candidates))),
			}
			entityID := "user-3"
			if name == "malformed" {
				entityID = linkingCandidateID
			}
			mockAuthnProvider.On("GetEntityReference", mock.Anything, otherAuthUser()).
				Return(otherAuthUser(), &providers.EntityReference{EntityID: entityID},
					(*tidcommon.ServiceError)(nil))
			mockAuthnProvider.On("GetEntityReference", mock.Anything, pendingAuthUser()).
				Return(pendingAuthUser(), (*providers.EntityReference)(nil), &tidcommon.ServiceError{
					Type: tidcommon.ClientErrorType, Code: "AUTHN-MGR-1007"})

			resp, err := executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
			assert.Equal(suite.T(), ErrCandidateNotVerified.Code, resp.Error.Code)
			assert.Equal(suite.T(), pendingAuthUser(), resp.AuthUser)
		})
	}
}

// Re-entry with nobody authenticated must fail. The engine has no visited-node tracking and no step
// cap, so going incomplete a second time would loop forever.
func (suite *LinkingExecutorTestSuite) TestExecute_ReEntryWithoutAuthenticationFails() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		RuntimeData: returningFromVerification(verificationPending()),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrVerificationNotCompleted.Code, resp.Error.Code)
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingCandidateUserIDs)
	suite.assertCycleEnded(resp)
}

// The first pass into the node raises no action, so it must still request one rather than read an
// absent type as a decision either way.
func (suite *LinkingExecutorTestSuite) TestExecute_NoDecisionRequestsPrompt() {
	ctx := &providers.NodeContext{
		ExecutionID:   "flow-1",
		FlowType:      providers.FlowTypeAuthentication,
		ForwardedData: linkingAction(""),
	}
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return(matchedCandidate(map[string]string{"email": "jane@example.com"}), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
}

// Refusal drops the candidate and continues as a new user: the End-User has answered "that is not
// me", and provisioning decides from there. Clearing the candidate is what lets it, since a
// candidate nobody settled is what provisioning refuses to provision over. This is the skip, and it
// is the button itself, not a value collected from a field.
func (suite *LinkingExecutorTestSuite) TestExecute_RefusedContinuesAsNewUser() {
	ctx := &providers.NodeContext{
		ExecutionID:   "flow-1",
		FlowType:      providers.FlowTypeAuthentication,
		ForwardedData: linkingAction(common.ActionTypeReject),
		RuntimeData:   returningFromVerification(verificationPending()),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyEntityState)
	suite.assertCycleEnded(resp)
}

// The confirm action points at the verification step, so a confirm type arriving here proves
// nothing: a flow that wires it back at this node instead must not link the account on the say-so
// alone.
func (suite *LinkingExecutorTestSuite) TestExecute_ConfirmAloneDoesNotLink() {
	ctx := &providers.NodeContext{
		ExecutionID:   "flow-1",
		FlowType:      providers.FlowTypeAuthentication,
		ForwardedData: linkingAction(common.ActionTypeConfirm),
		RuntimeData:   returningFromVerification(verificationPending()),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrVerificationNotCompleted.Code, resp.Error.Code)
}

// --- recording the link ---

const (
	linkingIdpID = "idp-a"
	linkingSub   = "sub-1"
)

// withFederatedIdentity adds the identity being linked to a case's RuntimeData, the way the
// federated auth executors publish it. Cases without it link nothing, which is every case above.
func withFederatedIdentity(runtime map[string]string) map[string]string {
	if runtime == nil {
		runtime = map[string]string{}
	}
	runtime[common.RuntimeKeyExternalIdentity] = externalIdentityEntry(linkingIdpID, linkingSub,
		map[string]interface{}{userAttributeSub: linkingSub})
	return runtime
}

func (suite *LinkingExecutorTestSuite) expectLink(err *tidcommon.ServiceError) {
	suite.mockAuthnProvider.On("LinkFederatedIdentity", mock.Anything, mock.Anything,
		linkingIdpID, linkingSub).Return(err)
}

// The candidate that proved itself is linked on the pass that settles the verification.
func (suite *LinkingExecutorTestSuite) TestExecute_VerifiedByTheCandidate_IsLinked() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    authenticatedAuthUser(),
		RuntimeData: returningFromVerification(withFederatedIdentity(verificationPending())),
	}
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{EntityID: linkingCandidateID},
			(*tidcommon.ServiceError)(nil))
	suite.expectLink(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])
	suite.assertCycleEnded(resp)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

// A write the provider rejects fails the node. Completing would sign the user in with nothing
// recorded, and the next sign-in would resolve nobody and provision a duplicate. The rejection is
// permanent, so the cycle ends, and the candidates stay for provisioning to refuse on.
func (suite *LinkingExecutorTestSuite) TestExecute_LinkWriteRejectedFails() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    verifiedAuthUser(),
		RuntimeData: returningFromVerification(withFederatedIdentity(verificationPending())),
	}
	suite.expectEntity(verifiedAuthUser(), linkingCandidateID)
	suite.expectLink(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AUTHN-MGR-1012"})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrLinkingWriteFailed.Code, resp.Error.Code)
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyEntityState)
	assert.NotContains(suite.T(), resp.RuntimeData, common.RuntimeKeyLinkingCandidateUserIDs)
	suite.assertCycleEnded(resp)
}

// A server error on the write is not the End-User's to act on, so it ends the flow with an internal
// error rather than a failure the flow could route.
func (suite *LinkingExecutorTestSuite) TestExecute_LinkWriteServerErrorIsAnError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    verifiedAuthUser(),
		RuntimeData: returningFromVerification(withFederatedIdentity(verificationPending())),
	}
	suite.expectEntity(verifiedAuthUser(), linkingCandidateID)
	suite.expectLink(&tidcommon.InternalServerError)

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

func TestCapitalizeFirst(t *testing.T) {
	for in, want := range map[string]string{
		"email":        "Email",
		"mobileNumber": "MobileNumber",
		"employee_id":  "Employee_id",
		"Email":        "Email",
		"élan":         "Élan",
		"":             "",
	} {
		assert.Equal(t, want, capitalizeFirst(in), in)
	}
}

// --- the checkpoint ---

// Asking for verification saves RuntimeData, the candidates included, and the pending AuthUser, and
// clears the external identity, so no claim fills a verification input and a federated verification
// step finds no earlier subject to be compared with.
func (suite *LinkingExecutorTestSuite) TestExecute_RequestingVerificationSavesCheckpoint() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    pendingAuthUser(),
		RuntimeData: withFederatedIdentity(map[string]string{"applicationId": "app-1"}),
	}
	suite.expectNobody(pendingAuthUser())
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return(matchedCandidate(map[string]string{"email": "jane@example.com"}), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyExternalIdentity])

	var saved linkingCheckpoint
	assert.NoError(suite.T(), json.Unmarshal([]byte(resp.RuntimeData[common.RuntimeKeyLinkingCheckpoint]), &saved))
	assert.Equal(suite.T(), ctx.RuntimeData[common.RuntimeKeyExternalIdentity],
		saved.RuntimeData[common.RuntimeKeyExternalIdentity])
	assert.Equal(suite.T(), "app-1", saved.RuntimeData["applicationId"])
	assert.JSONEq(suite.T(), `["user-1"]`, saved.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])
	assert.Equal(suite.T(), dataValueTrue, saved.RuntimeData[common.RuntimeKeyLinkingVerificationRequested])
	assert.Equal(suite.T(), pendingAuthUser(), saved.AuthUser)
	assert.JSONEq(suite.T(), `[{"label":"Email","value":"jane@example.com"}]`, saved.PromptDetails)
}

// Verification steps in this frame leave entries behind, including the entry a federated
// verification step publishes. The restore removes them and brings the original identity back
// before the link is written, so the identity linked is the one that started the flow.
func (suite *LinkingExecutorTestSuite) TestExecute_RestoresCheckpointBeforeSettling() {
	runtime := returningFromVerification(withFederatedIdentity(verificationPending()))
	runtime[userAttributeUserID] = linkingCandidateID
	runtime[common.RuntimeKeySelectedAuthClass] = "password"
	runtime[common.RuntimeKeyExternalIdentity] = externalIdentityEntry("idp-verifier", "sub-verifier",
		map[string]interface{}{userAttributeSub: "sub-verifier"})
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    authenticatedAuthUser(),
		RuntimeData: runtime,
	}
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{EntityID: linkingCandidateID},
			(*tidcommon.ServiceError)(nil))
	suite.expectLink(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotEmpty(suite.T(), ctx.RuntimeData[common.RuntimeKeyLinkingCheckpoint])
	delete(ctx.RuntimeData, common.RuntimeKeyLinkingCheckpoint)
	assert.Equal(suite.T(), withFederatedIdentity(verificationPending()), ctx.RuntimeData)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

// A refusal also restores the checkpoint, so provisioning links the identity that started the flow.
func (suite *LinkingExecutorTestSuite) TestExecute_RefusalRestoresExternalIdentity() {
	ctx := &providers.NodeContext{
		ExecutionID:   "flow-1",
		FlowType:      providers.FlowTypeAuthentication,
		ForwardedData: linkingAction(common.ActionTypeReject),
		RuntimeData:   returningFromVerification(withFederatedIdentity(verificationPending())),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), withFederatedIdentity(nil)[common.RuntimeKeyExternalIdentity],
		ctx.RuntimeData[common.RuntimeKeyExternalIdentity])
	suite.assertCycleEnded(resp)
}

// A pass that asked for verification always saved a checkpoint, so one that is missing or does not
// decode is a fault, and nothing is settled on it.
func (suite *LinkingExecutorTestSuite) TestExecute_MissingCheckpointIsAnError() {
	for name, runtime := range map[string]map[string]string{
		"missing":         verificationPending(),
		"malformed":       {common.RuntimeKeyLinkingCheckpoint: "{"},
		"no runtime data": {common.RuntimeKeyLinkingCheckpoint: `{"applicationId":"app-1"}`},
	} {
		suite.Run(name, func() {
			runtime[common.RuntimeKeyLinkingVerificationRequested] = dataValueTrue
			ctx := &providers.NodeContext{
				ExecutionID: "flow-1",
				FlowType:    providers.FlowTypeAuthentication,
				AuthUser:    authenticatedAuthUser(),
				RuntimeData: runtime,
			}

			resp, err := suite.executor.Execute(ctx)

			assert.Error(suite.T(), err)
			assert.Nil(suite.T(), resp)
		})
	}
}

// --- the verification cycle across passes ---

// firstPass runs the node on a pending federated identity that matches linkingCandidateID, and
// merges what it asked for, as the engine does before the prompt.
func (suite *LinkingExecutorTestSuite) firstPass() *providers.NodeContext {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    pendingAuthUser(),
		RuntimeData: withFederatedIdentity(map[string]string{"applicationId": "app-1"}),
		UserInputs:  map[string]string{"locale": "en"},
	}
	suite.expectNobody(pendingAuthUser())
	suite.mockAuthnProvider.On("ResolveFederatedCandidates", mock.Anything, mock.Anything).
		Return(matchedCandidate(map[string]string{"email": "jane@example.com"}), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(providers.ExecUserInputRequired, resp.Status)
	advance(ctx, resp)
	return ctx
}

// verifyAs stands in for verification steps that authenticated authUser, collected an identifier
// and left an entry behind.
func verifyAs(ctx *providers.NodeContext, authUser providers.AuthUser) {
	ctx.AuthUser = authUser
	ctx.UserInputs["username"] = "typed-during-verification"
	ctx.RuntimeData[userAttributeUserID] = "resolved-by-a-step"
}

// assertSettledClean checks RuntimeData after a cycle ended: the original identity is back, and
// nothing a verification step or a retry wrote survived.
func (suite *LinkingExecutorTestSuite) assertSettledClean(ctx *providers.NodeContext) {
	assert.Equal(suite.T(), withFederatedIdentity(nil)[common.RuntimeKeyExternalIdentity],
		ctx.RuntimeData[common.RuntimeKeyExternalIdentity])
	assert.Equal(suite.T(), "", ctx.RuntimeData[common.RuntimeKeyLinkingVerificationRequested])
	assert.Equal(suite.T(), "", ctx.RuntimeData[common.RuntimeKeyLinkingCheckpoint])
	assert.NotContains(suite.T(), ctx.RuntimeData, userAttributeUserID)
	assert.NotContains(suite.T(), ctx.RuntimeData, "failureReasonJSON")
}

// The wrong account verifies twice, then the candidate does. Each retry starts from the pending
// identity, and the candidate is linked with the identity that started the flow.
func (suite *LinkingExecutorTestSuite) TestCycle_WrongAccountThenCandidateLinks() {
	ctx := suite.firstPass()
	suite.expectEntity(otherAuthUser(), "user-2")
	suite.expectEntity(verifiedAuthUser(), linkingCandidateID)

	for range 2 {
		verifyAs(ctx, otherAuthUser())
		resp, err := suite.executor.Execute(ctx)
		suite.Require().NoError(err)
		assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
		assert.Equal(suite.T(), ErrCandidateNotVerified.Code, resp.Error.Code)
		advance(ctx, resp)
		ctx.RuntimeData["failureReasonJSON"] = `{"code":"FET-1093"}`

		assert.Equal(suite.T(), pendingAuthUser(), ctx.AuthUser)
		assert.Equal(suite.T(), map[string]string{"locale": "en"}, ctx.UserInputs,
			"the identifier the attempt collected must not fill the retry")
		assert.JSONEq(suite.T(), `[{"label":"Email","value":"jane@example.com"}]`,
			resp.AdditionalData[common.DataLinkingPromptDetails])
		assert.Equal(suite.T(), "", ctx.RuntimeData[common.RuntimeKeyExternalIdentity])
		assert.Equal(suite.T(), dataValueTrue, ctx.RuntimeData[common.RuntimeKeyLinkingVerificationRequested])
	}

	verifyAs(ctx, verifiedAuthUser())
	suite.expectLink(nil)
	resp, err := suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	advance(ctx, resp)

	suite.assertSettledClean(ctx)
	assert.Equal(suite.T(), "", ctx.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])
	assert.Equal(suite.T(), entityStateExists, ctx.RuntimeData[common.RuntimeKeyEntityState])
	suite.mockAuthnProvider.AssertNumberOfCalls(suite.T(), "LinkFederatedIdentity", 1)
}

// A refusal after the wrong account verified continues as a new user, not signed in as that account.
func (suite *LinkingExecutorTestSuite) TestCycle_WrongAccountThenRefusalContinuesAsNewUser() {
	ctx := suite.firstPass()
	suite.expectEntity(otherAuthUser(), "user-2")

	verifyAs(ctx, otherAuthUser())
	resp, err := suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	advance(ctx, resp)

	ctx.ForwardedData = linkingAction(common.ActionTypeReject)
	resp, err = suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	advance(ctx, resp)

	assert.Equal(suite.T(), pendingAuthUser(), ctx.AuthUser)
	suite.assertSettledClean(ctx)
	assert.Equal(suite.T(), "", ctx.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])
}

// After a rejected write the cycle has ended and the candidates stay. A later linking node starts a
// first pass rather than failing on a missing checkpoint.
func (suite *LinkingExecutorTestSuite) TestCycle_RejectedWriteThenLaterLinkingNode() {
	ctx := suite.firstPass()
	suite.expectEntity(verifiedAuthUser(), linkingCandidateID)
	suite.expectLink(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AUTHN-MGR-1012"})

	verifyAs(ctx, verifiedAuthUser())
	resp, err := suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), ErrLinkingWriteFailed.Code, resp.Error.Code)
	advance(ctx, resp)
	suite.assertSettledClean(ctx)
	assert.JSONEq(suite.T(), `["user-1"]`, ctx.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs])

	resp, err = suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

// After a link, a later linking node finds the account authenticated and completes.
func (suite *LinkingExecutorTestSuite) TestCycle_LinkedThenLaterLinkingNode() {
	ctx := suite.firstPass()
	suite.expectEntity(verifiedAuthUser(), linkingCandidateID)
	suite.expectLink(nil)

	verifyAs(ctx, verifiedAuthUser())
	resp, err := suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	advance(ctx, resp)

	resp, err = suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateExists, resp.RuntimeData[common.RuntimeKeyEntityState])
	suite.mockAuthnProvider.AssertNumberOfCalls(suite.T(), "LinkFederatedIdentity", 1)
}

// After a refusal, a later linking node matches again and asks for verification afresh.
func (suite *LinkingExecutorTestSuite) TestCycle_RefusedThenLaterLinkingNodePromptsAgain() {
	ctx := suite.firstPass()

	ctx.ForwardedData = linkingAction(common.ActionTypeReject)
	resp, err := suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	advance(ctx, resp)

	resp, err = suite.executor.Execute(ctx)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeyLinkingVerificationRequested])
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyLinkingCheckpoint])
}
