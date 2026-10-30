// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// attributeVerificationExecutor pins a user's contact before a challenge and records the proof after
// it. The contact type decides the channel, so one executor serves email and mobile. In a self-service
// flow the user is the one the engine bound to the execution at its start; in recovery, registration
// and onboarding it is the user the flow established, and a flow that creates its user may pin before
// the user exists.
type attributeVerificationExecutor struct {
	providers.Executor
	governance    providers.IdentityGovernanceProvider
	authnProvider providers.AuthnProviderManager
	logger        *log.Logger
}

var _ providers.Executor = (*attributeVerificationExecutor)(nil)

// newAttributeVerificationExecutor creates the attribute verification executor.
func newAttributeVerificationExecutor(factory core.FlowFactoryInterface,
	governance providers.IdentityGovernanceProvider,
	authnProvider providers.AuthnProviderManager) *attributeVerificationExecutor {
	base := factory.CreateExecutor(ExecutorNameAttributeVerification, providers.ExecutorTypeUtility,
		[]providers.Input{verificationAttributeInput}, nil, &providers.ExecutorMeta{
			SupportedModes: []string{ExecutorModePrepare, ExecutorModeRecord},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyVerificationAttribute, ApplicableModes: []string{ExecutorModePrepare}},
			},
		})
	return &attributeVerificationExecutor{
		Executor:      base,
		governance:    governance,
		authnProvider: authnProvider,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "AttributeVerificationExecutor"),
			log.String(log.LoggerKeyExecutorName, ExecutorNameAttributeVerification)),
	}
}

// Execute runs the node's mode. A refusal fails the node and clears the evidence; a server fault or an
// unknown mode is an error.
func (e *attributeVerificationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if e.governance == nil {
		return nil, errors.New("identity governance provider is not configured")
	}
	switch ctx.ExecutorMode {
	case ExecutorModePrepare:
		return e.prepare(ctx)
	case ExecutorModeRecord:
		return e.record(ctx)
	default:
		return nil, fmt.Errorf("invalid executor mode for AttributeVerificationExecutor: %s", ctx.ExecutorMode)
	}
}

// prepare pins the contact of the flow's user. The attribute is the node's property, else the input
// given when the flow starts; a resend keeps the attribute of the existing pin. Later nodes act on the
// pin, never on the inputs. A flow that will create its user pins the value it collected, checked
// against the user type.
func (e *attributeVerificationExecutor) prepare(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	resp := &providers.ExecutorResponse{RuntimeData: map[string]string{}}
	subject := e.verificationSubject(ctx, resp)
	// Only a flow that will create its user pins without one.
	if subject == "" && ctx.FlowType != providers.FlowTypeRegistration &&
		ctx.FlowType != providers.FlowTypeUserOnboarding {
		return failed(&ErrVerificationChannelUnavailable), nil
	}
	proof, err := pinnedVerificationEvidence(ctx)
	if err != nil {
		return nil, err
	}
	var attribute string
	if proof != nil {
		if proof.Pin.Subject != "" && proof.Pin.Subject != subject {
			return e.refusal(ctx, nil)
		}
		attribute = proof.Pin.Attribute
	} else if property, ok := ctx.NodeProperties[propertyKeyVerificationAttribute].(string); ok && property != "" {
		attribute = property
	} else {
		if !e.HasRequiredInputs(ctx, resp) {
			resp.Status = providers.ExecUserInputRequired
			return resp, nil
		}
		attribute, _ = ctx.ConsumeInput(userInputVerificationAttribute)
	}
	if strings.TrimSpace(attribute) == "" {
		return e.refusal(ctx, nil)
	}
	var pin *providers.AttributeVerificationPin
	var svcErr *tidcommon.ServiceError
	if subject != "" {
		pin, svcErr = e.governance.PrepareAttributeVerification(ctx.Context, subject, attribute)
	} else {
		pin, svcErr = e.governance.PrepareNewUserAttributeVerification(ctx.Context,
			ctx.RuntimeData[categoryTypeKey], attribute, collectedValue(ctx, attribute))
	}
	if svcErr != nil || pin == nil {
		return e.refusal(ctx, svcErr)
	}
	resp.SharedRuntimeData = sharedVerificationEvidence(verificationEvidence{Pin: pin})
	contactType, senderAttribute := channelContact(pin.Channel)
	// The sender resolves its recipient by its input identifier: the attribute name, or the channel's
	// default attribute in the shipped flows.
	resp.RuntimeData[pin.Attribute] = pin.Value
	if senderAttribute != pin.Attribute {
		resp.RuntimeData[senderAttribute] = pin.Value
	}
	if pin.Subject != "" {
		resp.RuntimeData[userAttributeUserID] = pin.Subject
	}
	resp.RuntimeData[common.RuntimeKeyContactType] = contactType
	clearIssuedChallenges(resp.RuntimeData)
	resp.Status = providers.ExecComplete
	return resp, nil
}

// record records the pinned contact once a challenge issued after the pin was passed and delivered
// only to the pinned value. The target comes from the pin; a pin made before the user existed is
// recorded for the user the flow established since, if that user's value is the pinned one.
func (e *attributeVerificationExecutor) record(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	proof, err := pinnedVerificationEvidence(ctx)
	if err != nil {
		return nil, err
	}
	if proof == nil || proof.Challenge == "" || !proof.Passed || len(proof.Receipts) == 0 {
		return e.refusal(ctx, nil)
	}
	for _, receipt := range proof.Receipts {
		if receipt.Channel != proof.Pin.Channel || receipt.Value != proof.Pin.Value {
			return e.refusal(ctx, nil)
		}
	}
	resp := &providers.ExecutorResponse{Status: providers.ExecComplete,
		SharedRuntimeData: map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}}
	pin := *proof.Pin
	if pin.Subject == "" {
		if pin.Subject = e.GetUserIDFromContext(ctx, resp, e.authnProvider); pin.Subject == "" {
			return e.refusal(ctx, nil)
		}
	}
	if svcErr := e.governance.RecordAttributeVerification(ctx.Context, pin); svcErr != nil {
		return e.refusal(ctx, svcErr)
	}
	return resp, nil
}

// verificationSubject returns the user prepare pins for, or "" when there is none. A self-service flow
// has only the user bound at its start, so a self-service flow CALLed from another flow type has none.
// Recovery has the user it identified. Registration and onboarding have the user they established, or
// none yet when they will create it.
func (e *attributeVerificationExecutor) verificationSubject(ctx *providers.NodeContext,
	resp *providers.ExecutorResponse) string {
	switch ctx.FlowType {
	case providers.FlowTypeSelfService:
		return ctx.GetBoundSubject()
	case providers.FlowTypeRecovery, providers.FlowTypeRegistration, providers.FlowTypeUserOnboarding:
		return e.GetUserIDFromContext(ctx, resp, e.authnProvider)
	default:
		// TODO: For ADMINISTRATION, authorize an explicit target user before pinning for challenge and record.
		// Keep the initiating administrator separate from the verification subject.
		return ""
	}
}

// refusal fails the node with the executor's single refusal code, so no refusal tells its cause. A
// server error from governance is returned as an error instead.
func (e *attributeVerificationExecutor) refusal(ctx *providers.NodeContext,
	svcErr *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
	if svcErr != nil && svcErr.Type == tidcommon.ServerErrorType {
		return nil, fmt.Errorf("attribute verification %s failed: %s", ctx.ExecutorMode,
			svcErr.Error.DefaultValue)
	}
	e.logger.Debug(ctx.Context, "Attribute verification refused",
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID), log.String("mode", ctx.ExecutorMode))
	return failed(&ErrIdentityGovernanceFailed), nil
}

// failed returns a response that fails the node with the error and clears the evidence.
func failed(err *tidcommon.ServiceError) *providers.ExecutorResponse {
	return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: err,
		SharedRuntimeData: map[string]string{common.RuntimeKeyAttributeVerificationEvidence: ""}}
}

// clearIssuedChallenges clears the invite and one-time password a send issued before this pin, so a
// re-entered flow issues a new challenge for it.
func clearIssuedChallenges(runtimeData map[string]string) {
	runtimeData[common.RuntimeKeyStoredInviteToken] = ""
	runtimeData[common.RuntimeKeyInviteLink] = ""
	runtimeData[common.RuntimeKeyOTPSessionToken] = ""
}
