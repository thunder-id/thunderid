// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"fmt"
	"strings"

	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// attributeVerificationExecutor pins a user's contact before a challenge and records the proof after
// it. The contact type decides the channel, so one executor serves email and mobile. In a self
// flow the user is the one the engine bound to the execution at its start; in recovery, registration
// and onboarding it is the user the flow established, and a flow that creates its user may pin before
// the user exists.
type attributeVerificationExecutor struct {
	providers.Executor
	verification  providers.AttributeVerificationProvider
	authnProvider providers.AuthnProviderManager
	accessGuard   *accessGuard
	logger        *log.Logger
}

var _ providers.Executor = (*attributeVerificationExecutor)(nil)

// newAttributeVerificationExecutor creates the attribute verification executor.
func newAttributeVerificationExecutor(factory core.FlowFactoryInterface,
	verification providers.AttributeVerificationProvider, authnProvider providers.AuthnProviderManager,
	accessGuard *accessGuard) *attributeVerificationExecutor {
	defaultInputs := []providers.Input{
		{
			Identifier: userInputVerificationAttribute,
			Type:       providers.InputTypeText,
			Required:   true,
		},
	}

	base := factory.CreateExecutor(ExecutorNameAttributeVerification, providers.ExecutorTypeUtility,
		defaultInputs, nil, &providers.ExecutorMeta{
			SupportedModes: []string{ExecutorModePrepare, ExecutorModeRecord},
		})

	return &attributeVerificationExecutor{
		Executor:      base,
		verification:  verification,
		authnProvider: authnProvider,
		accessGuard:   accessGuard,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "AttributeVerificationExecutor"),
			log.String(log.LoggerKeyExecutorName, ExecutorNameAttributeVerification)),
	}
}

// Execute runs the node's mode. A refusal fails the node and clears the evidence; a server fault or an
// unknown mode is an error.
func (e *attributeVerificationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if e.verification == nil {
		return nil, errors.New("attribute verification provider is not configured")
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

// prepare pins the contact of the flow's user. The attribute is the attribute input, which a flow fixes
// with a constant input; a resend keeps the attribute of the existing pin. Later nodes act on the
// pin, never on the inputs. A flow that will create its user pins the value it collected, checked
// against the user type. The pinned value becomes the senders' recipient, and the channel and masked
// contact are returned for display.
func (e *attributeVerificationExecutor) prepare(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	resp := &providers.ExecutorResponse{RuntimeData: map[string]string{}}
	subject := e.verificationSubject(ctx, resp)

	// Only a flow that will create its user pins without one.
	if subject == "" && ctx.FlowType != providers.FlowTypeRegistration &&
		ctx.FlowType != providers.FlowTypeUserOnboarding {
		return e.failed(&ErrVerificationChannelUnavailable), nil
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

	request := providers.AttributeVerificationRequest{EntityID: subject, Attribute: attribute,
		PendingCreation: subject == ""}
	if request.PendingCreation {
		request.EntityType, request.Value = ctx.RuntimeData[categoryTypeKey], collectedValue(ctx, attribute)
	} else if held, err := e.heldRefusal(ctx, subject); held != nil || err != nil {
		return held, err
	}

	pin, svcErr := e.verification.PrepareAttributeVerification(ctx.Context, request)
	if svcErr != nil || pin == nil {
		return e.refusal(ctx, svcErr)
	}

	shared, err := sharedVerificationEvidence(verificationEvidence{Pin: pin})
	if err != nil {
		return nil, err
	}

	resp.SharedRuntimeData = shared
	resp.SharedRuntimeData[common.RuntimeKeyDeliveredRecipients] = ""
	if pin.Subject != "" {
		resp.RuntimeData[userAttributeUserID] = pin.Subject
	}

	resp.RuntimeData[common.RuntimeKeyRecipientType] = entitytypemodel.ContactTypeForChannel(pin.Channel)
	resp.RuntimeData[common.RuntimeKeyExpectedRecipient] = pin.Value
	resp.RuntimeData[common.RuntimeKeyStoredInviteToken] = ""
	resp.RuntimeData[common.RuntimeKeyInviteLink] = ""
	resp.RuntimeData[common.RuntimeKeyOTPSessionToken] = ""

	resp.AdditionalData = map[string]string{common.DataVerificationChannel: string(pin.Channel)}
	if masked, ok := e.maskContact(pin.Channel, pin.Value); ok {
		resp.AdditionalData[common.DataMaskedDestination] = masked
	}

	resp.Status = providers.ExecComplete
	return resp, nil
}

// record records the pinned contact once a challenge was passed after the pin and every send since the
// pin delivered to the pinned value. The target comes from the pin; a pin made before the user
// existed is recorded for the user the flow established since, if that user's value is the pinned one.
func (e *attributeVerificationExecutor) record(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	proof, err := pinnedVerificationEvidence(ctx)
	if err != nil {
		return nil, err
	}

	if proof == nil || !proof.Passed || !e.deliveredToPin(ctx, proof.Pin) {
		return e.refusal(ctx, nil)
	}

	resp := &providers.ExecutorResponse{Status: providers.ExecComplete, RuntimeData: e.clearedRecipients(),
		SharedRuntimeData: e.clearedEvidence()}

	pin := *proof.Pin
	if pin.Subject == "" {
		if pin.Subject = e.GetUserIDFromContext(ctx, resp, e.authnProvider); pin.Subject == "" {
			return e.refusal(ctx, nil)
		}
	}

	if held, err := e.heldRefusal(ctx, pin.Subject); held != nil || err != nil {
		return held, err
	}

	if svcErr := e.verification.RecordAttributeVerification(ctx.Context, pin); svcErr != nil {
		return e.refusal(ctx, svcErr)
	}

	return resp, nil
}

// verificationSubject returns the user prepare pins for, or "" when there is none. A self flow
// has only the user bound at its start, so a self flow CALLed from another flow type has none.
// Recovery has the user it identified. Registration and onboarding have the user they established, or
// none yet when they will create it.
func (e *attributeVerificationExecutor) verificationSubject(ctx *providers.NodeContext,
	resp *providers.ExecutorResponse) string {
	switch ctx.FlowType {
	case providers.FlowTypeSelf:
		return ctx.GetBoundSubject()
	case providers.FlowTypeRecovery, providers.FlowTypeRegistration, providers.FlowTypeUserOnboarding:
		return e.GetUserIDFromContext(ctx, resp, e.authnProvider)
	default:
		// TODO: Support ADMINISTRATION with an authorized target user, apart from the administrator.
		return ""
	}
}

// heldRefusal returns the failed response for a user held at the verification plane, stating the hold,
// or nil when the user may verify. An automatic lock does not hold there.
func (e *attributeVerificationExecutor) heldRefusal(ctx *providers.NodeContext,
	subject string) (*providers.ExecutorResponse, error) {
	held, stated, err := e.accessGuard.heldAt(ctx.Context, subject, providers.AccessPlaneVerification, "",
		e.logger)
	if err != nil || !held {
		return nil, err
	}

	if stated == nil {
		return e.refusal(ctx, nil)
	}

	return e.failed(stated), nil
}

// refusal fails the node with the executor's refusal code, which does not tell which check failed. A
// server error from the provider is returned as an error instead.
func (e *attributeVerificationExecutor) refusal(ctx *providers.NodeContext,
	svcErr *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
	if svcErr != nil && svcErr.Type == tidcommon.ServerErrorType {
		return nil, fmt.Errorf("attribute verification failed: %s", svcErr.Error.DefaultValue)
	}

	e.logger.Debug(ctx.Context, "Attribute verification refused",
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID), log.String("mode", ctx.ExecutorMode))
	return e.failed(&ErrIdentityGovernanceFailed), nil
}

// failed returns a response that fails the node with the error and clears the evidence.
func (e *attributeVerificationExecutor) failed(err *tidcommon.ServiceError) *providers.ExecutorResponse {
	return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: err,
		RuntimeData: e.clearedRecipients(), SharedRuntimeData: e.clearedEvidence()}
}

// deliveredToPin reports whether there was a send since the pin and every one went to the pinned value. A
// recipient list that does not decode delivered to nothing.
func (e *attributeVerificationExecutor) deliveredToPin(ctx *providers.NodeContext,
	pin *providers.AttributeVerificationPin) bool {
	recipients, err := deliveredRecipients(ctx)
	if err != nil || len(recipients) == 0 {
		return false
	}

	for _, recipient := range recipients {
		if e.canonicalRecipient(pin.Channel, recipient) != pin.Value {
			return false
		}
	}

	return true
}

// clearedEvidence returns shared runtime data that clears the evidence and the delivered recipients.
func (e *attributeVerificationExecutor) clearedEvidence() map[string]string {
	return map[string]string{common.RuntimeKeyAttributeVerificationEvidence: "",
		common.RuntimeKeyDeliveredRecipients: ""}
}

// clearedRecipients returns runtime data that clears the recipient and its type prepare set for the senders.
func (e *attributeVerificationExecutor) clearedRecipients() map[string]string {
	return map[string]string{common.RuntimeKeyRecipientType: "", common.RuntimeKeyExpectedRecipient: ""}
}

// canonicalRecipient returns the comparable form of a recipient on the channel.
func (e *attributeVerificationExecutor) canonicalRecipient(channel providers.VerificationChannel,
	recipient string) string {
	return entitytypemodel.CanonicalContactValue(entitytypemodel.ContactTypeForChannel(channel), recipient)
}

// maskContact returns the contact for display: an email address keeps its first character and its
// domain, a mobile number its last digits. It returns false for a value that is not a contact of the
// channel.
func (e *attributeVerificationExecutor) maskContact(channel providers.VerificationChannel,
	value string) (string, bool) {
	switch channel {
	case providers.VerificationChannelEmail:
		local, domain, found := strings.Cut(value, "@")
		if !found || local == "" || domain == "" {
			return "", false
		}

		return string([]rune(local)[:1]) + contactMask + "@" + domain, true
	case providers.VerificationChannelSMS:
		runes := []rune(value)
		if len(runes) <= maskedMobileVisibleDigits {
			return "", false
		}

		return contactMask + string(runes[len(runes)-maskedMobileVisibleDigits:]), true
	default:
		return "", false
	}
}
