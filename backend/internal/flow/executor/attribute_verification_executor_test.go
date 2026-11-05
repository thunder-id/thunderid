// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/attributeverificationprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

type AttributeVerificationExecutorTestSuite struct {
	suite.Suite
	factory      core.FlowFactoryInterface
	verification *attributeverificationprovidermock.AttributeVerificationProviderMock
	governance   *identitygovernanceprovidermock.IdentityGovernanceProviderMock
	authn        *managermock.AuthnProviderManagerMock
	executor     *attributeVerificationExecutor
}

func TestAttributeVerificationExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeVerificationExecutorTestSuite))
}

func (s *AttributeVerificationExecutorTestSuite) SetupTest() {
	s.factory = newVerificationTestFactory(s.T())
	s.verification = attributeverificationprovidermock.NewAttributeVerificationProviderMock(s.T())
	s.governance = identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	s.governance.On("EvaluateAccess", mock.Anything, mock.Anything, providers.AccessPlaneVerification,
		providers.AuthenticationMethod("")).Return(providers.AccessDecision{Admitted: true}, nil).Maybe()
	s.authn = managermock.NewAuthnProviderManagerMock(s.T())
	s.executor = newAttributeVerificationExecutor(s.factory, s.verification, s.authn, newAccessGuard(s.governance))
}

func applyVerificationResponse(ctx *providers.NodeContext, resp *providers.ExecutorResponse) {
	for k, v := range resp.RuntimeData {
		ctx.RuntimeData[k] = v
	}
	for k, v := range resp.SharedRuntimeData {
		ctx.SharedRuntimeData[k] = v
	}
}

// existingRequest asks to pin an existing user's stored attribute.
func existingRequest(subject, attribute string) providers.AttributeVerificationRequest {
	return providers.AttributeVerificationRequest{EntityID: subject, Attribute: attribute}
}

// pendingRequest asks to pin a value collected for a user of the type pending creation.
func pendingRequest(entityType, attribute, value string) providers.AttributeVerificationRequest {
	return providers.AttributeVerificationRequest{EntityType: entityType, Attribute: attribute, Value: value,
		PendingCreation: true}
}

// pinnedEmailEvidence is shared runtime data for the pinned email, its challenge passed or not.
func pinnedEmailEvidence(t *testing.T, passed bool) map[string]string {
	return verificationEvidenceData(t, &verificationEvidence{Pin: emailPin(), Passed: passed})
}

// recordableEvidence is shared runtime data for the pin, its challenge passed and delivered only to the
// pinned value.
func recordableEvidence(t *testing.T, pin *providers.AttributeVerificationPin) map[string]string {
	shared := verificationEvidenceData(t, &verificationEvidence{Pin: pin, Passed: true})
	shared[common.RuntimeKeyDeliveredRecipients] = deliveredTo(t, pin.Value)
	return shared
}

// clearedShared is the shared runtime data a spent or refused verification leaves: no evidence and no
// delivered recipients.
func clearedShared() map[string]string {
	return map[string]string{common.RuntimeKeyAttributeVerificationEvidence: "",
		common.RuntimeKeyDeliveredRecipients: ""}
}

// clearedRuntime is the runtime data a spent or refused verification leaves: no recipient or recipient
// type for the senders.
func clearedRuntime() map[string]string {
	return map[string]string{common.RuntimeKeyRecipientType: "", common.RuntimeKeyExpectedRecipient: ""}
}

// decodedEvidence decodes the evidence a response wrote.
func (s *AttributeVerificationExecutorTestSuite) decodedEvidence(
	resp *providers.ExecutorResponse) *verificationEvidence {
	return evidenceIn(s.T(), resp.SharedRuntimeData)
}

// selfPrepareContext is a prepare node of a self flow bound to the subject.
func selfPrepareContext(subject string, inputs, shared map[string]string) *providers.NodeContext {
	ctx := &providers.NodeContext{Context: context.Background(), ExecutionID: "verification-1",
		FlowType: providers.FlowTypeSelf, ExecutorMode: ExecutorModePrepare,
		UserInputs: inputs, RuntimeData: map[string]string{}, SharedRuntimeData: shared}
	ctx.SetBoundSubject(subject)
	return ctx
}

// boundPrepareContext is a prepare node of a flow of the given type, for the email attribute, bound to
// subject.
func boundPrepareContext(flowType providers.FlowType, subject string,
	shared map[string]string) *providers.NodeContext {
	ctx := &providers.NodeContext{Context: context.Background(), FlowType: flowType,
		ExecutorMode: ExecutorModePrepare, UserInputs: map[string]string{"attribute": "email"},
		RuntimeData: map[string]string{}, SharedRuntimeData: shared}
	ctx.SetBoundSubject(subject)
	return ctx
}

// flowPrepareContext is a prepare node of a flow of the given type with the given runtime data.
func flowPrepareContext(flowType providers.FlowType, inputs, runtime map[string]string) *providers.NodeContext {
	return &providers.NodeContext{Context: context.Background(), ExecutionID: "verification-1",
		FlowType: flowType, ExecutorMode: ExecutorModePrepare, UserInputs: inputs, RuntimeData: runtime,
		SharedRuntimeData: map[string]string{}}
}

// recordContext is a record node of a flow of the given type over the evidence.
func recordContext(flowType providers.FlowType, shared map[string]string) *providers.NodeContext {
	return &providers.NodeContext{Context: context.Background(), FlowType: flowType,
		ExecutorMode: ExecutorModeRecord, UserInputs: map[string]string{}, SharedRuntimeData: shared}
}

// prepare pins the bound user, sets the expected recipient and its type and returns the masked contact.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareTakesTheBoundUser() {
	for name, test := range map[string]struct {
		pin               providers.AttributeVerificationPin
		recipientType     string
		senderInput       string
		maskedDestination string
	}{
		"email": {*emailPin(), "email", userAttributeEmail, "U•••@example.com"},
		"work email": {providers.AttributeVerificationPin{Subject: "user-a", Attribute: "workEmail",
			Value: "work@example.com", Channel: providers.VerificationChannelEmail},
			"email", userAttributeEmail, "w•••@example.com"},
		"mobile": {providers.AttributeVerificationPin{Subject: "user-a", Attribute: "workMobile",
			Value: "+94771234567", Channel: providers.VerificationChannelSMS},
			"mobile", common.AttributeMobileNumber, "•••67"},
		"mobile too short to mask": {providers.AttributeVerificationPin{Subject: "user-a", Attribute: "workMobile",
			Value: "12", Channel: providers.VerificationChannelSMS},
			"mobile", common.AttributeMobileNumber, ""},
	} {
		s.Run(name, func() {
			s.SetupTest()
			pin := test.pin
			s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", pin.Attribute)).
				Return(&pin, nil).Once()
			ctx := selfPrepareContext("user-a", map[string]string{"subject": "user-b", "attribute": pin.Attribute},
				map[string]string{common.RuntimeKeyDeliveredRecipients: deliveredTo(s.T(), "earlier@example.com")})
			ctx.RuntimeData = map[string]string{common.RuntimeKeyExpectedRecipient: "earlier-recipient",
				common.RuntimeKeyRecipientType: "other"}

			prepared, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Require().Equal(providers.ExecComplete, prepared.Status)
			s.Equal(&verificationEvidence{Pin: &pin}, s.decodedEvidence(prepared))
			s.Equal("", prepared.SharedRuntimeData[common.RuntimeKeyDeliveredRecipients],
				"an earlier send must not count")
			s.Contains(prepared.SharedRuntimeData, common.RuntimeKeyDeliveredRecipients)
			s.Equal(pin.Value, prepared.RuntimeData[common.RuntimeKeyExpectedRecipient],
				"the pin's sender must send to the pin")
			s.Equal(test.recipientType, prepared.RuntimeData[common.RuntimeKeyRecipientType])
			s.NotContains(prepared.RuntimeData, pin.Attribute)
			s.NotContains(prepared.RuntimeData, test.senderInput)
			s.Equal("user-a", prepared.RuntimeData[userAttributeUserID])
			wantDisplay := map[string]string{common.DataVerificationChannel: string(pin.Channel)}
			if test.maskedDestination != "" {
				wantDisplay[common.DataMaskedDestination] = test.maskedDestination
			}
			s.Equal(wantDisplay, prepared.AdditionalData)
		})
	}
}

// A flow type with no user source, or a self flow without a bound user, or recovery before it
// identified the user, fails before the provider is asked, and earlier evidence is cleared.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareFailsWithoutAUserSource() {
	for name, ctx := range map[string]*providers.NodeContext{
		"self flow without a bound user": selfPrepareContext("", map[string]string{"attribute": "email"},
			map[string]string{}),
		// A self flow CALLed from an authentication flow runs with the callee's type and no bound user.
		"self flow callee of another flow type": selfPrepareContext("",
			map[string]string{"subject": "user-a", "attribute": "email"},
			pinnedEmailEvidence(s.T(), true)),
		"administration": {Context: context.Background(), FlowType: providers.FlowTypeAdministration,
			ExecutorMode: ExecutorModePrepare, UserInputs: map[string]string{"subject": "user-a",
				"attribute": "email"}, RuntimeData: map[string]string{}, SharedRuntimeData: map[string]string{}},
		"recovery without an identified user": flowPrepareContext(providers.FlowTypeRecovery,
			map[string]string{"attribute": "email"}, map[string]string{}),
		"sign-out": flowPrepareContext(providers.FlowTypeSignOut, map[string]string{"attribute": "email"},
			map[string]string{userAttributeUserID: "user-a"}),
		"authentication": boundPrepareContext(providers.FlowTypeAuthentication, "user-a",
			pinnedEmailEvidence(s.T(), true)),
	} {
		s.Run(name, func() {
			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, result.Status)
			s.Equal(ErrVerificationChannelUnavailable.Code, result.Error.Code)
			s.Equal(clearedShared(), result.SharedRuntimeData)
			s.Equal(clearedRuntime(), result.RuntimeData)
		})
	}
	s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
}

// Recovery, registration and onboarding pin the user the flow established; a self flow bound
// subject plays no part there.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareTakesTheEstablishedUser() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeRecovery, providers.FlowTypeRegistration,
		providers.FlowTypeUserOnboarding} {
		s.Run(string(flowType), func() {
			s.SetupTest()
			pin := emailPin()
			s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
				Return(pin, nil).Once()
			ctx := flowPrepareContext(flowType, map[string]string{"attribute": "email"},
				map[string]string{userAttributeUserID: "user-a", categoryTypeKey: "Customer"})
			ctx.SetBoundSubject("user-b")

			prepared, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Require().Equal(providers.ExecComplete, prepared.Status)
			s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
			s.Equal("user-a", prepared.RuntimeData[userAttributeUserID])
		})
	}
}

// The authenticated user is the established user when no node published a user ID.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareTakesTheAuthenticatedUser() {
	pin := emailPin()
	authUser := providers.AuthUser{}
	authUser.SetStateFor("default", providers.AuthState{EntityReference: &providers.EntityReference{},
		Attributes: &providers.AttributesResponse{}})
	s.authn.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, &providers.EntityReference{EntityID: "user-a"}, nil).Once()
	s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
		Return(pin, nil).Once()
	ctx := flowPrepareContext(providers.FlowTypeRegistration, map[string]string{"attribute": "email"},
		map[string]string{})
	ctx.AuthUser = authUser

	prepared, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, prepared.Status)
	s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
}

// A flow that will create its user pins the value it collected, checked against the resolved user type,
// and publishes no user ID.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareBeforeTheUserExistsPinsTheCollectedValue() {
	for name, test := range map[string]struct {
		flowType providers.FlowType
		inputs   map[string]string
		runtime  map[string]string
	}{
		"registration input": {providers.FlowTypeRegistration,
			map[string]string{"attribute": "email", "email": "User@example.com"},
			map[string]string{categoryTypeKey: "Customer"}},
		"onboarding runtime value": {providers.FlowTypeUserOnboarding, map[string]string{"attribute": "email"},
			map[string]string{categoryTypeKey: "Customer", "email": "User@example.com"}},
	} {
		s.Run(name, func() {
			s.SetupTest()
			pin := emailPin()
			pin.Subject = ""
			s.verification.On("PrepareAttributeVerification", mock.Anything,
				pendingRequest("Customer", "email", "User@example.com")).Return(pin, nil).Once()

			prepared, err := s.executor.Execute(flowPrepareContext(test.flowType, test.inputs, test.runtime))

			s.Require().NoError(err)
			s.Require().Equal(providers.ExecComplete, prepared.Status)
			s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
			s.NotContains(prepared.RuntimeData, userAttributeUserID)
			s.Equal("email", prepared.RuntimeData[common.RuntimeKeyRecipientType])
		})
	}
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareBeforeTheUserExistsRefused() {
	s.verification.On("PrepareAttributeVerification", mock.Anything, pendingRequest("", "email", "")).
		Return(nil, &tidcommon.ServiceError{Code: "AVS-1001", Type: tidcommon.ClientErrorType}).Once()

	result, err := s.executor.Execute(flowPrepareContext(providers.FlowTypeRegistration,
		map[string]string{"attribute": "email"}, map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(clearedShared(), result.SharedRuntimeData)
	s.Equal(clearedRuntime(), result.RuntimeData)
}

// constantAttributeNode is a prepare node over the executor whose attribute input is the constant value.
func (s *AttributeVerificationExecutorTestSuite) constantAttributeNode(value string) core.NodeInterface {
	node, err := s.factory.CreateNode("prepare_verification", string(common.NodeTypeTaskExecution),
		map[string]interface{}{}, false, false)
	s.Require().NoError(err)
	backed, ok := node.(core.ExecutorBackedNodeInterface)
	s.Require().True(ok)
	backed.SetMode(ExecutorModePrepare)
	backed.SetInputs([]providers.Input{{Identifier: userInputVerificationAttribute, Type: providers.InputTypeText,
		Required: true, DefaultValue: value, Constant: true}})
	backed.SetExecutor(s.executor)
	return node
}

// A constant attribute input names the contact, so the flow does not ask for it and an attribute input
// the caller sends cannot change it.
func (s *AttributeVerificationExecutorTestSuite) TestAConstantAttributeInputNamesTheContact() {
	for name, inputs := range map[string]map[string]string{
		"no input":          {},
		"another attribute": {"attribute": "mobile"},
	} {
		s.Run(name, func() {
			s.SetupTest()
			pin := emailPin()
			s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
				Return(pin, nil).Once()
			ctx := flowPrepareContext(providers.FlowTypeRecovery, inputs,
				map[string]string{userAttributeUserID: "user-a"})

			resp, svcErr := s.constantAttributeNode("email").Execute(ctx)

			s.Require().Nil(svcErr)
			s.Require().Equal(common.NodeStatusComplete, resp.Status)
			s.Empty(resp.Inputs, "a constant attribute must not be asked for")
			s.Equal(&verificationEvidence{Pin: pin}, evidenceIn(s.T(), resp.SharedRuntimeData))
		})
	}
}

// When the flow returns to a node with a constant attribute, the existing pin's attribute still wins.
func (s *AttributeVerificationExecutorTestSuite) TestAResendKeepsThePinnedAttributeOverAConstant() {
	pin := emailPin()
	pin.Attribute = "workEmail"
	s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "workEmail")).
		Return(pin, nil).Once()
	ctx := flowPrepareContext(providers.FlowTypeRecovery, map[string]string{},
		map[string]string{userAttributeUserID: "user-a"})
	ctx.SharedRuntimeData = verificationEvidenceData(s.T(), &verificationEvidence{Pin: pin})

	resp, svcErr := s.constantAttributeNode("email").Execute(ctx)

	s.Require().Nil(svcErr)
	s.Require().Equal(common.NodeStatusComplete, resp.Status)
	s.Equal("workEmail", evidenceIn(s.T(), resp.SharedRuntimeData).Pin.Attribute)
}

// A resend after the user was created re-pins the pin made before, now for the user.
func (s *AttributeVerificationExecutorTestSuite) TestResendAfterTheUserExistsRepinsForTheUser() {
	before := emailPin()
	before.Subject = ""
	pin := emailPin()
	s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
		Return(pin, nil).Once()
	ctx := flowPrepareContext(providers.FlowTypeRegistration, map[string]string{"attribute": "mobile"},
		map[string]string{userAttributeUserID: "user-a"})
	ctx.SharedRuntimeData = verificationEvidenceData(s.T(), &verificationEvidence{Pin: before})

	prepared, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, prepared.Status)
	s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareAsksForTheAttribute() {
	shared := pinnedEmailEvidence(s.T(), false)
	shared[common.RuntimeKeyAttributeVerificationEvidence] = ""

	result, err := s.executor.Execute(selfPrepareContext("user-a", map[string]string{}, shared))

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, result.Status)
	s.Equal([]providers.Input{
		{Identifier: userInputVerificationAttribute, Type: providers.InputTypeText, Required: true},
	}, result.Inputs)
	s.Nil(result.SharedRuntimeData, "a prompt must not clear existing evidence")
	s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
}

// A resend re-pins the existing pin's attribute and resets the proof.
func (s *AttributeVerificationExecutorTestSuite) TestResendRepinsThePinnedAttribute() {
	pin := emailPin()
	s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
		Return(pin, nil).Once()
	ctx := selfPrepareContext("user-a", map[string]string{"attribute": "workEmail"}, recordableEvidence(s.T(), pin))
	ctx.RuntimeData = map[string]string{common.RuntimeKeyStoredInviteToken: "invite-old",
		common.RuntimeKeyInviteLink:      "https://localhost/invite?token=invite-old",
		common.RuntimeKeyOTPSessionToken: "otp-old"}

	prepared, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, prepared.Status)
	s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
	s.Subset(prepared.SharedRuntimeData, map[string]string{common.RuntimeKeyDeliveredRecipients: ""})
	s.Subset(prepared.RuntimeData, map[string]string{common.RuntimeKeyStoredInviteToken: "",
		common.RuntimeKeyInviteLink: "", common.RuntimeKeyOTPSessionToken: "",
		common.RuntimeKeyExpectedRecipient: pin.Value, common.RuntimeKeyRecipientType: "email"})
}

// A pin of another user than the bound one is never re-pinned.
func (s *AttributeVerificationExecutorTestSuite) TestResendRefusesAPinOfAnotherUser() {
	result, err := s.executor.Execute(selfPrepareContext("user-b", map[string]string{"attribute": "email"},
		recordableEvidence(s.T(), emailPin())))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(clearedShared(), result.SharedRuntimeData)
	s.Equal(clearedRuntime(), result.RuntimeData)
	s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareRefusedPinsNothing() {
	refusal := &tidcommon.ServiceError{Code: "AVS-1001", Type: tidcommon.ClientErrorType}
	s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
		Return(nil, refusal).Once()

	result, err := s.executor.Execute(selfPrepareContext("user-a", map[string]string{"attribute": "email"},
		map[string]string{common.RuntimeKeyProvenSubject: "user-a"}))

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(clearedShared(), result.SharedRuntimeData)
	s.Equal(clearedRuntime(), result.RuntimeData, "a refused preparation must name no recipient")
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareRefusesABlankAttribute() {
	result, err := s.executor.Execute(selfPrepareContext("user-a", map[string]string{"attribute": " \t"},
		map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
}

func (s *AttributeVerificationExecutorTestSuite) TestRefusalIsLoggedWithoutTheSubject() {
	refusal := &tidcommon.ServiceError{Code: "AVS-1001", Type: tidcommon.ClientErrorType}
	s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-alpha", "email")).
		Return(nil, refusal).Once()
	logger := log.GetLogger()
	path := filepath.Join(s.T().TempDir(), "verification.log")
	s.Require().NoError(logger.Configure(log.OutputOptions{FileEnabled: true, File: rollingfile.Config{Path: path}}))
	s.Require().NoError(logger.SetLevel("DEBUG"))
	defer func() {
		s.Require().NoError(logger.SetLevel("INFO"))
		s.Require().NoError(logger.Configure(log.OutputOptions{ConsoleEnabled: true}))
	}()

	result, err := s.executor.Execute(selfPrepareContext("user-alpha", map[string]string{"attribute": "email"},
		map[string]string{}))

	s.Require().NoError(err)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	// #nosec G304 -- path is in the test's temp directory
	logged, err := os.ReadFile(path)
	s.Require().NoError(err)
	s.Contains(string(logged), "Attribute verification refused")
	s.NotContains(string(logged), "user-alpha")
}

// Without the provider, or with an unknown mode, the node is misconfigured, which is an error rather than
// a refusal.
func (s *AttributeVerificationExecutorTestSuite) TestErrorsOutsideItsContract() {
	for name, test := range map[string]struct {
		executor *attributeVerificationExecutor
		mode     string
	}{
		"no provider":  {&attributeVerificationExecutor{}, ExecutorModePrepare},
		"no mode":      {s.executor, ""},
		"unknown mode": {s.executor, "verify"},
	} {
		s.Run(name, func() {
			ctx := selfPrepareContext("user-a", map[string]string{"attribute": "email"}, map[string]string{})
			ctx.ExecutorMode = test.mode

			result, err := test.executor.Execute(ctx)

			s.Error(err)
			s.Nil(result)
		})
	}
	s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
}

// A provider server fault at either step is an error, not a refusal, so it is not answered with the
// refusal code.
func (s *AttributeVerificationExecutorTestSuite) TestProviderServerErrorIsAnError() {
	fault := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SSE-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "Internal server error"}}
	pin := emailPin()
	for name, test := range map[string]struct {
		arrange func()
		ctx     *providers.NodeContext
	}{
		"prepare": {
			arrange: func() {
				s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
					Return(nil, fault).Once()
			},
			ctx: selfPrepareContext("user-a", map[string]string{"attribute": "email"}, map[string]string{}),
		},
		"record": {
			arrange: func() {
				s.verification.On("RecordAttributeVerification", mock.Anything, *pin).Return(fault).Once()
			},
			ctx: recordContext(providers.FlowTypeSelf, recordableEvidence(s.T(), pin)),
		},
	} {
		s.Run(name, func() {
			s.SetupTest()
			test.arrange()

			result, err := s.executor.Execute(test.ctx)

			s.Error(err)
			s.Nil(result)
		})
	}
}

func (s *AttributeVerificationExecutorTestSuite) TestCorruptEvidenceStopsTheExecutor() {
	corrupt := map[string]string{common.RuntimeKeyAttributeVerificationEvidence: "{"}
	for name, ctx := range map[string]*providers.NodeContext{
		"record":  recordContext(providers.FlowTypeSelf, corrupt),
		"prepare": selfPrepareContext("user-a", map[string]string{"attribute": "email"}, corrupt),
	} {
		s.Run(name, func() {
			result, err := s.executor.Execute(ctx)

			s.Error(err)
			s.Nil(result)
		})
	}
	s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
	s.verification.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
}

// Recording needs a passed challenge and a send since the pin, every one to the pinned value; a send on
// the other channel cannot equal the pin.
func (s *AttributeVerificationExecutorTestSuite) TestRefusesIncompleteOrMismatchedEvidence() {
	for _, test := range []struct {
		name   string
		pin    *providers.AttributeVerificationPin
		mutate func(proof *verificationEvidence, shared map[string]string)
	}{
		{"not passed", emailPin(), func(p *verificationEvidence, _ map[string]string) { p.Passed = false }},
		{"nothing sent", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = ""
		}},
		{"no send recorded since the pin", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			delete(sh, common.RuntimeKeyDeliveredRecipients)
		}},
		{"an empty recipient list", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = "[]"
		}},
		{"a recipient list that does not decode", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = "not a list"
		}},
		{"sent to another address", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = deliveredTo(s.T(), "other@example.com")
		}},
		{"sent to another address before the pin", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = deliveredTo(s.T(), "other@example.com", "User@example.com")
		}},
		{"an email pin delivered by SMS", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = deliveredTo(s.T(), testPinnedMobile)
		}},
		{"an email pin sent by SMS, then by email", emailPin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = deliveredTo(s.T(), testPinnedMobile, "User@example.com")
		}},
		{"a mobile pin sent to another number", mobilePin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = deliveredTo(s.T(), "+94770000000")
		}},
		{"a mobile pin sent by email, then by SMS", mobilePin(), func(_ *verificationEvidence, sh map[string]string) {
			sh[common.RuntimeKeyDeliveredRecipients] = deliveredTo(s.T(), "User@example.com", testPinnedMobile)
		}},
	} {
		s.Run(test.name, func() {
			s.SetupTest()
			proof := &verificationEvidence{Pin: test.pin, Passed: true}
			shared := recordableEvidence(s.T(), test.pin)
			test.mutate(proof, shared)
			for k, v := range verificationEvidenceData(s.T(), proof) {
				shared[k] = v
			}

			result, err := s.executor.Execute(recordContext(providers.FlowTypeSelf, shared))

			s.Require().NoError(err)
			s.Require().Equal(providers.ExecFailure, result.Status)
			s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
			s.Equal(clearedShared(), result.SharedRuntimeData)
			s.Equal(clearedRuntime(), result.RuntimeData)
		})
	}
	s.verification.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
}

// A send recorded in another form of the pinned value, as a sender may write it, still counts.
func (s *AttributeVerificationExecutorTestSuite) TestRecordMatchesTheSendCanonically() {
	pin := emailPin()
	s.verification.On("RecordAttributeVerification", mock.Anything, *pin).Return(nil).Once()
	shared := recordableEvidence(s.T(), pin)
	shared[common.RuntimeKeyDeliveredRecipients] = deliveredTo(s.T(), " User@EXAMPLE.COM ", "User@example.com")

	result, err := s.executor.Execute(recordContext(providers.FlowTypeSelf, shared))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, result.Status)
}

// Recording acts on the pin alone, in any flow type and for either channel, and spends the proof: the
// evidence, the delivered recipient and the senders' recipient are cleared.
func (s *AttributeVerificationExecutorTestSuite) TestRecordRecordsThePinInAnyFlowType() {
	for name, test := range map[string]struct {
		flowType providers.FlowType
		pin      *providers.AttributeVerificationPin
	}{
		"self email":           {providers.FlowTypeSelf, emailPin()},
		"self mobile":          {providers.FlowTypeSelf, mobilePin()},
		"authentication email": {providers.FlowTypeAuthentication, emailPin()},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.verification.On("RecordAttributeVerification", mock.Anything, *test.pin).Return(nil).Once()
			ctx := recordContext(test.flowType, recordableEvidence(s.T(), test.pin))
			ctx.UserInputs = map[string]string{"subject": "user-b", "attribute": "workEmail"}
			ctx.RuntimeData = map[string]string{common.RuntimeKeyExpectedRecipient: test.pin.Value,
				common.RuntimeKeyRecipientType: "email"}
			ctx.SetBoundSubject("user-b")

			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, result.Status)
			s.Nil(result.Error)
			s.Equal(clearedShared(), result.SharedRuntimeData, "a recorded proof is spent")
			s.Equal(clearedRuntime(), result.RuntimeData, "a recorded proof leaves no recipient")
		})
	}
}

// A pin made before the user existed is recorded for the user the flow established since.
func (s *AttributeVerificationExecutorTestSuite) TestRecordBindsANewUserPinToTheEstablishedUser() {
	before := emailPin()
	before.Subject = ""
	s.verification.On("RecordAttributeVerification", mock.Anything, *emailPin()).Return(nil).Once()
	ctx := recordContext(providers.FlowTypeRegistration, recordableEvidence(s.T(), before))
	ctx.RuntimeData = map[string]string{userAttributeUserID: "user-a"}

	result, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, result.Status)
}

// With no user established yet, a pin made before the user existed records nothing.
func (s *AttributeVerificationExecutorTestSuite) TestRecordWithoutAUserRefusesANewUserPin() {
	before := emailPin()
	before.Subject = ""
	ctx := recordContext(providers.FlowTypeRegistration, recordableEvidence(s.T(), before))
	ctx.RuntimeData = map[string]string{}

	result, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.verification.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
}

func (s *AttributeVerificationExecutorTestSuite) TestRecordRefusedFails() {
	pin := emailPin()
	s.verification.On("RecordAttributeVerification", mock.Anything, *pin).
		Return(&tidcommon.ServiceError{Code: "AVS-1001", Type: tidcommon.ClientErrorType}).Once()

	result, err := s.executor.Execute(recordContext(providers.FlowTypeSelf, recordableEvidence(s.T(), pin)))

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(clearedShared(), result.SharedRuntimeData)
	s.Equal(clearedRuntime(), result.RuntimeData)
}

// Evidence sent as an input is never read: a complete forged proof records nothing without asking
// the provider, and a forged pin of another user does not choose whom a preparation verifies.
func (s *AttributeVerificationExecutorTestSuite) TestRefusesCallerSuppliedEvidence() {
	s.Run("record", func() {
		s.SetupTest()
		ctx := recordContext(providers.FlowTypeSelf, map[string]string{})
		ctx.UserInputs = recordableEvidence(s.T(), emailPin())

		result, err := s.executor.Execute(ctx)

		s.Require().NoError(err)
		s.Require().Equal(providers.ExecFailure, result.Status)
		s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
		s.verification.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
	})
	s.Run("prepare", func() {
		s.SetupTest()
		pin := emailPin()
		other := emailPin()
		other.Subject = "user-b"
		s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
			Return(pin, nil).Once()
		inputs := recordableEvidence(s.T(), other)
		inputs[userInputVerificationAttribute] = pin.Attribute

		prepared, err := s.executor.Execute(selfPrepareContext("user-a", inputs, map[string]string{}))

		s.Require().NoError(err)
		s.Require().Equal(providers.ExecComplete, prepared.Status)
		s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared), "the bound user must be pinned")
	})
}

func (s *AttributeVerificationExecutorTestSuite) TestSupportsEveryFlowTypeAndBothModes() {
	meta := s.executor.GetMeta()
	s.Require().NotNil(meta)
	s.Empty(meta.SupportedFlowTypes)
	s.ElementsMatch([]string{ExecutorModePrepare, ExecutorModeRecord}, meta.SupportedModes)
	s.Equal([]providers.Input{
		{Identifier: userInputVerificationAttribute, Type: providers.InputTypeText, Required: true},
	}, s.executor.GetDefaultInputs())
}

// withAccessDecision rebuilds the executor over a governance provider that answers every check of
// user-a at the verification plane with the decision, and returns that provider.
func (s *AttributeVerificationExecutorTestSuite) withAccessDecision(decision providers.AccessDecision,
	svcErr *tidcommon.ServiceError) *identitygovernanceprovidermock.IdentityGovernanceProviderMock {
	s.governance = identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	s.governance.On("EvaluateAccess", mock.Anything, "user-a", providers.AccessPlaneVerification,
		providers.AuthenticationMethod("")).Return(decision, svcErr).Once()
	s.executor = newAttributeVerificationExecutor(s.factory, s.verification, s.authn, newAccessGuard(s.governance))
	return s.governance
}

// accessCheckedSteps are a prepare for user-a and a record of user-a's pinned email.
func (s *AttributeVerificationExecutorTestSuite) accessCheckedSteps() map[string]*providers.NodeContext {
	return map[string]*providers.NodeContext{
		ExecutorModePrepare: selfPrepareContext("user-a", map[string]string{"attribute": "email"}, map[string]string{}),
		ExecutorModeRecord:  recordContext(providers.FlowTypeSelf, recordableEvidence(s.T(), emailPin())),
	}
}

// expectProviderCall expects the provider call of the access-checked step, once.
func (s *AttributeVerificationExecutorTestSuite) expectProviderCall(step string) {
	if step == ExecutorModePrepare {
		s.verification.On("PrepareAttributeVerification", mock.Anything, existingRequest("user-a", "email")).
			Return(emailPin(), nil).Once()
		return
	}
	s.verification.On("RecordAttributeVerification", mock.Anything, *emailPin()).Return(nil).Once()
}

// A suspended user is refused at both steps with the suspension stated, before the provider is asked.
func (s *AttributeVerificationExecutorTestSuite) TestASuspendedUserIsRefused() {
	for name, ctx := range s.accessCheckedSteps() {
		s.Run(name, func() {
			s.SetupTest()
			s.withAccessDecision(providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended}, nil)

			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, result.Status)
			s.Equal(ErrAccountSuspended.Code, result.Error.Code)
			s.Equal(clearedShared(), result.SharedRuntimeData)
			s.Equal(clearedRuntime(), result.RuntimeData)
			s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
			s.verification.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
		})
	}
}

// A refusal that names no hold is answered with the executor's refusal code.
func (s *AttributeVerificationExecutorTestSuite) TestAnUnstatedHoldIsRefused() {
	for name, ctx := range s.accessCheckedSteps() {
		s.Run(name, func() {
			s.SetupTest()
			s.withAccessDecision(providers.AccessDecision{}, nil)

			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, result.Status)
			s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
			s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
			s.verification.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
		})
	}
}

// Access is checked at the verification plane, where an automatic lock does not hold, so a user the
// plane admits verifies at both steps.
func (s *AttributeVerificationExecutorTestSuite) TestAccessIsCheckedAtTheVerificationPlane() {
	for name, ctx := range s.accessCheckedSteps() {
		s.Run(name, func() {
			s.SetupTest()
			governance := s.withAccessDecision(providers.AccessDecision{Admitted: true}, nil)
			s.expectProviderCall(name)

			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, result.Status)
			governance.AssertExpectations(s.T())
		})
	}
}

// Without a governance provider nothing is held, and verification still runs.
func (s *AttributeVerificationExecutorTestSuite) TestWithoutGovernanceVerificationRuns() {
	for name, ctx := range s.accessCheckedSteps() {
		s.Run(name, func() {
			s.SetupTest()
			s.executor = newAttributeVerificationExecutor(s.factory, s.verification, s.authn, newAccessGuard(nil))
			s.expectProviderCall(name)

			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, result.Status)
		})
	}
}

// An access check governance cannot answer is an error, and the provider is not asked.
func (s *AttributeVerificationExecutorTestSuite) TestAnUnreadableAccessDecisionIsAnError() {
	for name, ctx := range s.accessCheckedSteps() {
		s.Run(name, func() {
			s.SetupTest()
			s.withAccessDecision(providers.AccessDecision{}, &tidcommon.InternalServerError)

			result, err := s.executor.Execute(ctx)

			s.Error(err)
			s.Nil(result)
			s.verification.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything)
			s.verification.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
		})
	}
}

func (s *AttributeVerificationExecutorTestSuite) TestMaskContact() {
	for _, test := range []struct {
		channel       providers.VerificationChannel
		value, masked string
		ok            bool
	}{
		{"email", "alex@example.com", "a•••@example.com", true},
		{"email", "ü@example.com", "ü•••@example.com", true},
		{"email", "alex", "", false},
		{"email", "@example.com", "", false},
		{"email", "alex@", "", false},
		{"sms", "+94771234567", "•••67", true},
		{"sms", "+12", "•••12", true},
		{"sms", "12", "", false},
		{"sms", "+٩٤٧٧١٢٣٤٥٦", "•••٥٦", true},
		{"push", "alex@example.com", "", false},
	} {
		masked, ok := (&attributeVerificationExecutor{}).maskContact(test.channel, test.value)
		s.Equal(test.ok, ok, "%s %q", test.channel, test.value)
		s.Equal(test.masked, masked, "%s %q", test.channel, test.value)
	}
}
