// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

type AttributeVerificationExecutorTestSuite struct {
	suite.Suite
	factory    core.FlowFactoryInterface
	governance *identitygovernanceprovidermock.IdentityGovernanceProviderMock
	authn      *managermock.AuthnProviderManagerMock
	executor   *attributeVerificationExecutor
}

func TestAttributeVerificationExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeVerificationExecutorTestSuite))
}

func (s *AttributeVerificationExecutorTestSuite) SetupTest() {
	s.factory = newVerificationTestFactory(s.T())
	s.governance = identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	s.authn = managermock.NewAuthnProviderManagerMock(s.T())
	s.executor = newAttributeVerificationExecutor(s.factory, s.governance, s.authn)
}

func newVerificationTestFactory(t *testing.T) core.FlowFactoryInterface {
	if err := config.InitializeServerRuntime(t.TempDir(), &config.Config{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(config.ResetServerRuntime)
	factory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "verification-test"))
	return factory
}

func applyVerificationResponse(ctx *providers.NodeContext, resp *providers.ExecutorResponse) {
	for k, v := range resp.RuntimeData {
		ctx.RuntimeData[k] = v
	}
	for k, v := range resp.SharedRuntimeData {
		ctx.SharedRuntimeData[k] = v
	}
}

// verificationEvidenceData returns shared runtime data holding the given evidence.
func verificationEvidenceData(t *testing.T, proof *verificationEvidence) map[string]string {
	t.Helper()
	return sharedVerificationEvidence(*proof)
}

// emailPin is a pinned email contact for user-a.
func emailPin() *providers.AttributeVerificationPin {
	return &providers.AttributeVerificationPin{Subject: "user-a", Attribute: "email",
		Value: "User@example.com", Channel: providers.VerificationChannelEmail}
}

// pinnedEmailEvidence is shared runtime data for the pinned email with one receipt, whose active
// challenge is challenge.
func pinnedEmailEvidence(t *testing.T, challenge string, passed bool) map[string]string {
	return verificationEvidenceData(t, &verificationEvidence{Pin: emailPin(), Challenge: challenge, Passed: passed,
		Receipts: []verificationReceipt{{Channel: providers.VerificationChannelEmail, Value: emailPin().Value}}})
}

// evidenceIn decodes the evidence a response wrote to shared runtime data.
func evidenceIn(t *testing.T, shared map[string]string) *verificationEvidence {
	t.Helper()
	proof, err := pinnedVerificationEvidence(&providers.NodeContext{SharedRuntimeData: shared})
	require.NoError(t, err)
	require.NotNil(t, proof)
	return proof
}

// decodedEvidence decodes the evidence a response wrote.
func (s *AttributeVerificationExecutorTestSuite) decodedEvidence(
	resp *providers.ExecutorResponse) *verificationEvidence {
	return evidenceIn(s.T(), resp.SharedRuntimeData)
}

// selfPrepareContext is a prepare node of a self-service flow bound to the subject.
func selfPrepareContext(subject string, inputs, shared map[string]string) *providers.NodeContext {
	ctx := &providers.NodeContext{Context: context.Background(), ExecutionID: "verification-1",
		FlowType: providers.FlowTypeSelfService, ExecutorMode: ExecutorModePrepare,
		UserInputs: inputs, RuntimeData: map[string]string{}, SharedRuntimeData: shared}
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

// The bound user is pinned, a subject input is ignored, and the pin is exposed to the sender, the
// challenge node and the chooser.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareTakesTheBoundUser() {
	for name, test := range map[string]struct {
		pin         providers.AttributeVerificationPin
		contactType string
		senderInput string
	}{
		"email": {*emailPin(), "email", userAttributeEmail},
		"mobile": {providers.AttributeVerificationPin{Subject: "user-a", Attribute: "workMobile",
			Value: "+94771234567", Channel: providers.VerificationChannelSMS},
			"mobile", common.AttributeMobileNumber},
	} {
		s.Run(name, func() {
			s.SetupTest()
			pin := test.pin
			s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", pin.Attribute).
				Return(&pin, nil).Once()

			prepared, err := s.executor.Execute(selfPrepareContext("user-a",
				map[string]string{"subject": "user-b", "attribute": pin.Attribute}, map[string]string{}))

			s.Require().NoError(err)
			s.Require().Equal(providers.ExecComplete, prepared.Status)
			s.Equal(&verificationEvidence{Pin: &pin}, s.decodedEvidence(prepared))
			s.Equal(test.contactType, prepared.RuntimeData[common.RuntimeKeyContactType])
			s.Equal(pin.Value, prepared.RuntimeData[pin.Attribute])
			s.Equal(pin.Value, prepared.RuntimeData[test.senderInput])
			s.Equal("user-a", prepared.RuntimeData[userAttributeUserID])
		})
	}
}

// A flow type with no user source, or a self-service flow without a bound user, or recovery before it
// identified the user, fails before governance is asked, and earlier evidence is cleared.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareFailsWithoutAUserSource() {
	for name, ctx := range map[string]*providers.NodeContext{
		"self-service without a bound user": selfPrepareContext("", map[string]string{"attribute": "email"},
			map[string]string{}),
		// A self-service flow CALLed from an authentication flow runs with the callee's type and no bound user.
		"self-service callee of another flow type": selfPrepareContext("",
			map[string]string{"subject": "user-a", "attribute": "email"},
			pinnedEmailEvidence(s.T(), "session-1", true)),
		"administration": {Context: context.Background(), FlowType: providers.FlowTypeAdministration,
			ExecutorMode: ExecutorModePrepare, UserInputs: map[string]string{"subject": "user-a",
				"attribute": "email"}, RuntimeData: map[string]string{}, SharedRuntimeData: map[string]string{}},
		"recovery without an identified user": flowPrepareContext(providers.FlowTypeRecovery,
			map[string]string{"attribute": "email"}, map[string]string{}),
		"sign-out": flowPrepareContext(providers.FlowTypeSignOut, map[string]string{"attribute": "email"},
			map[string]string{userAttributeUserID: "user-a"}),
		"authentication": {Context: context.Background(), FlowType: providers.FlowTypeAuthentication,
			ExecutorMode: ExecutorModePrepare, UserInputs: map[string]string{"attribute": "email"},
			RuntimeData: map[string]string{}, SharedRuntimeData: pinnedEmailEvidence(s.T(), "session-1", true)},
	} {
		s.Run(name, func() {
			if name == "authentication" {
				ctx.SetBoundSubject("user-a")
			}

			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, result.Status)
			s.Equal(ErrVerificationChannelUnavailable.Code, result.Error.Code)
			s.Equal(map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}, result.SharedRuntimeData)
			s.Empty(result.RuntimeData)
		})
	}
	s.governance.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything,
		mock.Anything)
}

// Recovery, registration and onboarding pin the user the flow established; a self-service bound
// subject plays no part there.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareTakesTheEstablishedUser() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeRecovery, providers.FlowTypeRegistration,
		providers.FlowTypeUserOnboarding} {
		s.Run(string(flowType), func() {
			s.SetupTest()
			pin := emailPin()
			s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").
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
	s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").Return(pin, nil).Once()
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
			s.governance.On("PrepareNewUserAttributeVerification", mock.Anything, "Customer", "email",
				"User@example.com").Return(pin, nil).Once()

			prepared, err := s.executor.Execute(flowPrepareContext(test.flowType, test.inputs, test.runtime))

			s.Require().NoError(err)
			s.Require().Equal(providers.ExecComplete, prepared.Status)
			s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
			s.NotContains(prepared.RuntimeData, userAttributeUserID)
			s.Equal(pin.Value, prepared.RuntimeData[userAttributeEmail])
			s.Equal("email", prepared.RuntimeData[common.RuntimeKeyContactType])
		})
	}
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareBeforeTheUserExistsRefusedByGovernance() {
	s.governance.On("PrepareNewUserAttributeVerification", mock.Anything, "", "email", "").
		Return(nil, &tidcommon.ServiceError{Code: "IGS-1002", Type: tidcommon.ClientErrorType}).Once()

	result, err := s.executor.Execute(flowPrepareContext(providers.FlowTypeRegistration,
		map[string]string{"attribute": "email"}, map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}, result.SharedRuntimeData)
}

// The node's attribute property names the contact, so the flow does not ask for the input and an
// input cannot change it.
func (s *AttributeVerificationExecutorTestSuite) TestPrepareTakesTheAttributeProperty() {
	pin := emailPin()
	s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").Return(pin, nil).Once()
	ctx := flowPrepareContext(providers.FlowTypeRecovery, map[string]string{"attribute": "mobile"},
		map[string]string{userAttributeUserID: "user-a"})
	ctx.NodeProperties = map[string]interface{}{propertyKeyVerificationAttribute: "email"}

	prepared, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, prepared.Status)
	s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
}

// A resend after the user was created re-pins the pin made before, now for the user.
func (s *AttributeVerificationExecutorTestSuite) TestResendAfterTheUserExistsRepinsForTheUser() {
	before := emailPin()
	before.Subject = ""
	pin := emailPin()
	s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").Return(pin, nil).Once()
	ctx := flowPrepareContext(providers.FlowTypeRegistration, map[string]string{"attribute": "mobile"},
		map[string]string{userAttributeUserID: "user-a"})
	ctx.SharedRuntimeData = verificationEvidenceData(s.T(), &verificationEvidence{Pin: before,
		Challenge: "session-1"})

	prepared, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, prepared.Status)
	s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareAsksForTheAttribute() {
	shared := pinnedEmailEvidence(s.T(), "", false)
	shared[common.RuntimeKeyAttributeVerificationEvidence] = ""

	result, err := s.executor.Execute(selfPrepareContext("user-a", map[string]string{}, shared))

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, result.Status)
	s.Equal([]providers.Input{verificationAttributeInput}, result.Inputs)
	s.Nil(result.SharedRuntimeData, "a prompt must not clear existing evidence")
	s.governance.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything,
		mock.Anything)
}

// A resend re-pins the attribute of the existing pin, whatever attribute a later request names, and
// resets the proof state.
func (s *AttributeVerificationExecutorTestSuite) TestResendRepinsThePinnedAttribute() {
	pin := emailPin()
	s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").Return(pin, nil).Once()
	ctx := selfPrepareContext("user-a", map[string]string{"attribute": "workEmail"},
		pinnedEmailEvidence(s.T(), "session-old", true))
	ctx.RuntimeData = map[string]string{common.RuntimeKeyOTPSessionToken: "session-old",
		common.RuntimeKeyStoredInviteToken: "invite-old"}

	prepared, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, prepared.Status)
	s.Equal(&verificationEvidence{Pin: pin}, s.decodedEvidence(prepared))
	s.Subset(prepared.RuntimeData, map[string]string{
		common.RuntimeKeyOTPSessionToken: "", common.RuntimeKeyStoredInviteToken: "",
		common.RuntimeKeyInviteLink: ""})
}

// A pin of another user than the bound one is never re-pinned.
func (s *AttributeVerificationExecutorTestSuite) TestResendRefusesAPinOfAnotherUser() {
	result, err := s.executor.Execute(selfPrepareContext("user-b", map[string]string{"attribute": "email"},
		pinnedEmailEvidence(s.T(), "session-old", true)))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}, result.SharedRuntimeData)
	s.governance.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareRefusedByGovernancePinsNothing() {
	refusal := &tidcommon.ServiceError{Code: "IGS-1001", Type: tidcommon.ClientErrorType}
	s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").
		Return(nil, refusal).Once()

	result, err := s.executor.Execute(selfPrepareContext("user-a", map[string]string{"attribute": "email"},
		map[string]string{common.RuntimeKeyProvenSubject: "user-a"}))

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}, result.SharedRuntimeData)
	s.NotContains(result.RuntimeData, common.RuntimeKeyContactType)
}

func (s *AttributeVerificationExecutorTestSuite) TestPrepareRefusesABlankAttribute() {
	result, err := s.executor.Execute(selfPrepareContext("user-a", map[string]string{"attribute": " \t"},
		map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.governance.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *AttributeVerificationExecutorTestSuite) TestRefusalIsLoggedWithoutTheSubject() {
	refusal := &tidcommon.ServiceError{Code: "IGS-1002", Type: tidcommon.ClientErrorType}
	s.governance.On("PrepareAttributeVerification", mock.Anything, "user-alpha", "email").
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

// Without governance, or with an unknown mode, the node is misconfigured, which is an error rather than
// a refusal.
func (s *AttributeVerificationExecutorTestSuite) TestErrorsOutsideItsContract() {
	for name, test := range map[string]struct {
		executor *attributeVerificationExecutor
		mode     string
	}{
		"no governance": {&attributeVerificationExecutor{}, ExecutorModePrepare},
		"no mode":       {s.executor, ""},
		"unknown mode":  {s.executor, "verify"},
	} {
		s.Run(name, func() {
			ctx := selfPrepareContext("user-a", map[string]string{"attribute": "email"}, map[string]string{})
			ctx.ExecutorMode = test.mode

			result, err := test.executor.Execute(ctx)

			s.Error(err)
			s.Nil(result)
		})
	}
	s.governance.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything,
		mock.Anything)
}

// A governance server fault at either step is an error, not a refusal, so it is not answered with the
// refusal code.
func (s *AttributeVerificationExecutorTestSuite) TestGovernanceServerErrorIsAnError() {
	fault := &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "SSE-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "Internal server error"}}
	pin := emailPin()
	for name, test := range map[string]struct {
		arrange func()
		ctx     *providers.NodeContext
	}{
		"prepare": {
			arrange: func() {
				s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").
					Return(nil, fault).Once()
			},
			ctx: selfPrepareContext("user-a", map[string]string{"attribute": "email"}, map[string]string{}),
		},
		"record": {
			arrange: func() {
				s.governance.On("RecordAttributeVerification", mock.Anything, *pin).Return(fault).Once()
			},
			ctx: recordContext(providers.FlowTypeSelfService, pinnedEmailEvidence(s.T(), "session-1", true)),
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
		"record":  recordContext(providers.FlowTypeSelfService, corrupt),
		"prepare": selfPrepareContext("user-a", map[string]string{"attribute": "email"}, corrupt),
	} {
		s.Run(name, func() {
			result, err := s.executor.Execute(ctx)

			s.Error(err)
			s.Nil(result)
		})
	}
	s.governance.AssertNotCalled(s.T(), "PrepareAttributeVerification", mock.Anything, mock.Anything,
		mock.Anything)
	s.governance.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
}

func (s *AttributeVerificationExecutorTestSuite) TestRefusesIncompleteOrMismatchedEvidence() {
	for _, test := range []struct {
		name   string
		mutate func(*verificationEvidence)
	}{
		{"no challenge", func(p *verificationEvidence) { p.Challenge = "" }},
		{"not passed", func(p *verificationEvidence) { p.Passed = false }},
		{"no receipt", func(p *verificationEvidence) { p.Receipts = nil }},
		{"wrong channel", func(p *verificationEvidence) { p.Receipts[0].Channel = "sms" }},
		{"wrong destination", func(p *verificationEvidence) { p.Receipts[0].Value = "other@example.com" }},
		{"redirected second delivery", func(p *verificationEvidence) {
			p.Receipts = append(p.Receipts, verificationReceipt{Channel: "email", Value: "other@example.com"})
		}},
	} {
		s.Run(test.name, func() {
			s.SetupTest()
			proof := &verificationEvidence{Pin: emailPin(), Challenge: "session-1", Passed: true,
				Receipts: []verificationReceipt{{Channel: "email", Value: "User@example.com"}}}
			test.mutate(proof)

			result, err := s.executor.Execute(recordContext(providers.FlowTypeSelfService,
				verificationEvidenceData(s.T(), proof)))

			s.Require().NoError(err)
			s.Require().Equal(providers.ExecFailure, result.Status)
			s.Equal(map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}, result.SharedRuntimeData)
		})
	}
	s.governance.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
}

// Recording acts on the pin alone, in any flow type. A second send of the same code leaves a second,
// identical receipt.
func (s *AttributeVerificationExecutorTestSuite) TestRecordRecordsThePinInAnyFlowType() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeSelfService, providers.FlowTypeAuthentication} {
		s.Run(string(flowType), func() {
			s.SetupTest()
			pin := emailPin()
			s.governance.On("RecordAttributeVerification", mock.Anything, *pin).Return(nil).Once()
			receipt := verificationReceipt{Channel: providers.VerificationChannelEmail, Value: pin.Value}
			ctx := recordContext(flowType, verificationEvidenceData(s.T(), &verificationEvidence{Pin: pin,
				Challenge: "session-1", Passed: true, Receipts: []verificationReceipt{receipt, receipt}}))
			ctx.UserInputs = map[string]string{"subject": "user-b", "attribute": "workEmail"}
			ctx.SetBoundSubject("user-b")

			result, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, result.Status)
			s.Nil(result.Error)
			s.Equal(map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}, result.SharedRuntimeData,
				"a recorded proof is spent")
		})
	}
}

// A pin made before the user existed is recorded for the user the flow established since.
func (s *AttributeVerificationExecutorTestSuite) TestRecordBindsANewUserPinToTheEstablishedUser() {
	before := emailPin()
	before.Subject = ""
	s.governance.On("RecordAttributeVerification", mock.Anything, *emailPin()).Return(nil).Once()
	ctx := recordContext(providers.FlowTypeRegistration, verificationEvidenceData(s.T(), &verificationEvidence{
		Pin: before, Challenge: "session-1", Passed: true,
		Receipts: []verificationReceipt{{Channel: providers.VerificationChannelEmail, Value: before.Value}}}))
	ctx.RuntimeData = map[string]string{userAttributeUserID: "user-a"}

	result, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, result.Status)
}

// With no user established yet, a pin made before the user existed records nothing.
func (s *AttributeVerificationExecutorTestSuite) TestRecordWithoutAUserRefusesANewUserPin() {
	before := emailPin()
	before.Subject = ""
	ctx := recordContext(providers.FlowTypeRegistration, verificationEvidenceData(s.T(), &verificationEvidence{
		Pin: before, Challenge: "session-1", Passed: true,
		Receipts: []verificationReceipt{{Channel: providers.VerificationChannelEmail, Value: before.Value}}}))
	ctx.RuntimeData = map[string]string{}

	result, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.governance.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
}

func (s *AttributeVerificationExecutorTestSuite) TestRecordRefusedByGovernanceFails() {
	pin := emailPin()
	s.governance.On("RecordAttributeVerification", mock.Anything, *pin).
		Return(&tidcommon.ServiceError{Code: "IGS-1002", Type: tidcommon.ClientErrorType}).Once()

	result, err := s.executor.Execute(recordContext(providers.FlowTypeSelfService,
		pinnedEmailEvidence(s.T(), "session-1", true)))

	s.Require().NoError(err)
	s.Require().Equal(providers.ExecFailure, result.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
	s.Equal(map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}, result.SharedRuntimeData)
}

// Evidence sent as an input is never read: a complete forged proof records nothing without asking
// governance, and a forged pin of another user does not choose whom a preparation verifies.
func (s *AttributeVerificationExecutorTestSuite) TestRefusesCallerSuppliedEvidence() {
	forge := func(pin *providers.AttributeVerificationPin) map[string]string {
		return verificationEvidenceData(s.T(), &verificationEvidence{Pin: pin, Challenge: "session-1", Passed: true,
			Receipts: []verificationReceipt{{Channel: pin.Channel, Value: pin.Value}}})
	}

	s.Run("record", func() {
		s.SetupTest()
		ctx := recordContext(providers.FlowTypeSelfService, map[string]string{})
		ctx.UserInputs = forge(emailPin())

		result, err := s.executor.Execute(ctx)

		s.Require().NoError(err)
		s.Require().Equal(providers.ExecFailure, result.Status)
		s.Equal(ErrIdentityGovernanceFailed.Code, result.Error.Code)
		s.governance.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
	})
	s.Run("prepare", func() {
		s.SetupTest()
		pin := emailPin()
		other := emailPin()
		other.Subject = "user-b"
		s.governance.On("PrepareAttributeVerification", mock.Anything, "user-a", "email").Return(pin, nil).Once()
		inputs := forge(other)
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
	s.Equal([]providers.Input{verificationAttributeInput}, s.executor.GetDefaultInputs())
}
