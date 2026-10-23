// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// extensionContractVersion is the version of the request and response contract. An incompatible
	// contract gets a new value; the meaning of a published version never changes.
	extensionContractVersion = "1"
	// extensionMaxResponseBytes caps the response body read. A larger body is an error.
	extensionMaxResponseBytes = 64 << 10
	// extensionMaxFailureDescriptionLength is the number of characters of failureDescription shown.
	extensionMaxFailureDescriptionLength = 256

	// Fields of the response contract. They are never stored as data.
	extensionFieldActionStatus       = "actionStatus"
	extensionFieldFailureReason      = "failureReason"
	extensionFieldFailureDescription = "failureDescription"
	extensionFieldRedirectURL        = "redirectUrl"

	// Params of the FAILED and INCOMPLETE errors.
	extensionParamDescription = "description"
	extensionParamReason      = "reason"

	// Failure reasons that show ThunderID's own credential-failure message.
	// nolint:gosec // G101: This is a failure reason code, not a credential
	extensionReasonInvalidCredentials = "invalid_credentials"
	extensionReasonUserNotFound       = "user_not_found"
)

// extensionFailureReasonPattern is the form a failureReason must have to reach the client.
var extensionFailureReasonPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// extensionOutcome is how an extension execution ended. The first four are the actionStatus values a
// service answers with; extensionOutcomeConfigError and extensionOutcomeAttemptsExceeded are the
// executor's own and are never accepted from a service.
type extensionOutcome string

const (
	extensionOutcomeSuccess          extensionOutcome = "SUCCESS"
	extensionOutcomeFailed           extensionOutcome = "FAILED"
	extensionOutcomeIncomplete       extensionOutcome = "INCOMPLETE"
	extensionOutcomeError            extensionOutcome = "ERROR"
	extensionOutcomeConfigError      extensionOutcome = "CONFIG_ERROR"
	extensionOutcomeAttemptsExceeded extensionOutcome = "ATTEMPTS_EXCEEDED"
)

// extensionErrorKind tells an operator why an execution ended in ERROR.
type extensionErrorKind string

const (
	extensionErrorKindTimeout     extensionErrorKind = "timeout"
	extensionErrorKindTransport   extensionErrorKind = "transport"
	extensionErrorKindStatus      extensionErrorKind = "status"
	extensionErrorKindBody        extensionErrorKind = "body"
	extensionErrorKindContract    extensionErrorKind = "contract"
	extensionErrorKindReservedKey extensionErrorKind = "reserved_key"
	// extensionErrorKindInternal is a failure inside ThunderID, such as a connection that exists but
	// cannot be resolved or a request that cannot be built.
	extensionErrorKindInternal extensionErrorKind = "internal"
)

// extensionSecretRuntimeKeys hold engine state that grants access or would let a step be replayed.
// They are never sent, even when a node names them, and a response may not set them.
var extensionSecretRuntimeKeys = map[string]bool{
	common.RuntimeKeyOTPSessionToken:     true,
	common.RuntimeKeyConsentSessionToken: true,
	common.RuntimeKeyOAuthState:          true,
	common.RuntimeKeyOIDCNonce:           true,
	common.RuntimeKeyMagicLinkUsedJti:    true,
	common.RuntimeKeyStoredInviteToken:   true,
	common.RuntimeKeyInviteLink:          true,
	common.RuntimeKeyMagicLinkURL:        true,
	common.RuntimeKeySSOSessionHandle:    true,
	common.RuntimeKeySSOSessionID:        true,
	common.RuntimeKeyTokenFamilyID:       true,
	common.RuntimeKeyCandidateUsers:      true,
	common.RuntimeKeyOpenID4VPState:      true,
	common.RuntimeKeySSOSessionPresent:   true,
	common.RuntimeKeySSOSessionSaved:     true,
	// The linking checkpoint is a snapshot of the whole runtime data and authenticated user.
	common.RuntimeKeyLinkingCheckpoint: true,
	common.RuntimeKeyFailureReasonJSON: true,
	runtimePasskeySessionToken:         true,
	// The attempt count is the executor's own state: a service neither reads nor sets it.
	common.RuntimeKeyExtensionAttemptCount: true,
	// Proof of control lets a later step act on the account it names.
	common.RuntimeKeyProvenSubject: true,
	// A password is a credential; a service is given one only through inputKeys.
	userAttributePassword: true,
}

// extensionSecretRuntimeKeyPrefixes match the secret keys whose names carry a suffix: the
// per-checkpoint SSO keys, since a saved checkpoint holds a session handle, and the per-step attempt
// counts.
var extensionSecretRuntimeKeyPrefixes = []string{
	common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, ""),
	common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, ""),
	common.ExtensionAttemptCountKey(""),
}

// extensionEngineRuntimeKeys are the other runtime-data keys the engine or a built-in step relies on.
// They are sent only when a node names them, except extensionDefaultRuntimeKeys, and a response may
// not set them.
var extensionEngineRuntimeKeys = map[string]bool{
	userAttributeUserID:              true,
	userAttributeGroups:              true,
	common.RuntimeKeyUserIDCamelCase: true,
	common.RuntimeKeyApplicationID:   true,
	common.RuntimeKeyIDPID:           true,
	common.RuntimeKeySenderID:        true,
	ouIDKey:                          true,
	defaultOUIDKey:                   true,
	categoryTypeKey:                  true,
	ownerIDKey:                       true,
	authorizedPermissionsKey:         true,

	common.RuntimeKeyAllowRegistrationWithExistingUser: true,
	common.RuntimeKeyAllowedLoginOptions:               true,
	common.RuntimeKeyAuthTime:                          true,
	common.RuntimeKeyAuthorizationRequestID:            true,
	common.RuntimeKeyBindingMessage:                    true,
	common.RuntimeKeyCallbackType:                      true,
	common.RuntimeKeyClientID:                          true,
	common.RuntimeKeyConsentID:                         true,
	common.RuntimeKeyConsentedAttributes:               true,
	common.RuntimeKeyConsentedPermissions:              true,
	common.RuntimeKeyEntityState:                       true,
	common.RuntimeKeyExternalIdentity:                  true,
	common.RuntimeKeyForceConsentReprompt:              true,
	common.RuntimeKeyForceReauth:                       true,
	common.RuntimeKeyLinkingCandidateUserIDs:           true,
	common.RuntimeKeyLinkingVerificationRequested:      true,
	common.RuntimeKeyLogoutPromptRequired:              true,
	common.RuntimeKeyMagicLinkDestinationAttribute:     true,
	common.RuntimeKeyMagicLinkExpiryMinutes:            true,
	common.RuntimeKeyMappedGroupIDs:                    true,
	common.RuntimeKeyMappedPermissions:                 true,
	common.RuntimeKeyMappedRoleIDs:                     true,
	common.RuntimeKeyMaxAge:                            true,
	common.RuntimeKeyOTPAttemptCount:                   true,
	common.RuntimeKeyPresentedOptionalInputs:           true,
	common.RuntimeKeyRequestedAuthClasses:              true,
	common.RuntimeKeyRequestedPermissions:              true,
	common.RuntimeKeyRequiredEssentialAttributes:       true,
	common.RuntimeKeyRequiredLocales:                   true,
	common.RuntimeKeyRequiredOptionalAttributes:        true,
	common.RuntimeKeyResourceServerIdentifier:          true,
	common.RuntimeKeyRevocationPlan:                    true,
	common.RuntimeKeySSOSessionCleared:                 true,
	common.RuntimeKeySelectedAuthClass:                 true,
	common.RuntimeKeySilentAuthOnly:                    true,
	common.RuntimeKeySkipDelivery:                      true,
	common.RuntimeKeyStepTimeout:                       true,
	common.RuntimeKeyUserAmbiguous:                     true,
	common.RuntimeKeyUserAttributesCacheTTLSeconds:     true,
	common.RuntimeKeyUserAutoProvisioned:               true,
	common.RuntimeKeyUserEligibleForProvisioning:       true,
}

// extensionDefaultRuntimeKeys are the runtime-data keys sent when a node does not set runtimeDataKeys.
var extensionDefaultRuntimeKeys = []string{common.RuntimeKeyApplicationID, userAttributeUserID}

// isSecretRuntimeKey reports whether key holds engine state that never leaves the server.
func isSecretRuntimeKey(key string) bool {
	if extensionSecretRuntimeKeys[key] {
		return true
	}
	return slices.ContainsFunc(extensionSecretRuntimeKeyPrefixes, func(prefix string) bool {
		return strings.HasPrefix(key, prefix)
	})
}

// isReservedRuntimeKey reports whether key is one the engine or a built-in step relies on, so an
// extension's response may not carry it.
func isReservedRuntimeKey(key string) bool {
	return extensionEngineRuntimeKeys[key] || isSecretRuntimeKey(key)
}

// extensionRequest is the body sent to the service. It mirrors providers.NodeContext field for field,
// carrying only the parts that are safe to leave the server.
type extensionRequest struct {
	Version       string                        `json:"version"`
	ExecutionID   string                        `json:"executionId"`
	FlowType      providers.FlowType            `json:"flowType"`
	CurrentAction string                        `json:"currentAction,omitempty"`
	CurrentNodeID string                        `json:"currentNodeId"`
	UserInputs    map[string]string             `json:"userInputs"`
	RuntimeData   map[string]string             `json:"runtimeData"`
	Application   extensionApplication          `json:"application"`
	AuthUser      map[string]extensionAuthState `json:"authUser"`
}

// extensionApplication is the part of providers.Application sent to a service. The inbound
// authentication configuration carries client secrets and is left out.
type extensionApplication struct {
	ID   string `json:"id,omitempty"`
	OUID string `json:"ouId,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// extensionAuthState is the part of providers.AuthState sent to a service: the entity reference only,
// never provider tokens or attributes.
type extensionAuthState struct {
	EntityReference *providers.EntityReference `json:"entityReference,omitempty"`
}

// extensionCall is the record of one execution, published as the audit event.
type extensionCall struct {
	connectionID  string
	outcome       extensionOutcome
	errorKind     extensionErrorKind
	failureReason string
	httpStatus    int
}

// extensionExecutor calls an organization's service through an HTTP endpoint connection and acts on
// the actionStatus it answers with. It is blind to identity: it only adds data to the flow and never
// sets the authenticated user.
type extensionExecutor struct {
	providers.Executor
	connectionResolver httpconnection.HTTPConnectionResolver
	observabilitySvc   providers.ObservabilityProvider
	clientOnce         sync.Once
	client             httpservice.HTTPClientInterface
	logger             *log.Logger
}

var _ providers.Executor = (*extensionExecutor)(nil)

// newExtensionExecutor creates an extension executor.
func newExtensionExecutor(flowFactory core.FlowFactoryInterface,
	connectionResolver httpconnection.HTTPConnectionResolver,
	observabilitySvc providers.ObservabilityProvider) *extensionExecutor {
	base := flowFactory.CreateExecutor(ExecutorNameExtension, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: common.NodePropertyConnectionID, IsRequired: true},
				{Property: common.NodePropertyInputKeys},
				{Property: common.NodePropertyRuntimeDataKeys},
				{Property: common.NodePropertyResponseKeys},
				{Property: common.NodePropertyMaxAttempts},
			},
		})
	return &extensionExecutor{
		Executor:           base,
		connectionResolver: connectionResolver,
		observabilitySvc:   observabilitySvc,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameExtension),
			log.String(log.LoggerKeyExecutorName, ExecutorNameExtension)),
	}
}

// Execute calls the service and routes on its answer: SUCCESS completes, FAILED and ERROR fail
// towards onFailure, and INCOMPLETE asks for input through onIncomplete. A step whose FAILED and
// INCOMPLETE answers have reached maxAttempts in this execution fails without a call. Every
// execution publishes one audit event.
func (e *extensionExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	started := time.Now()
	execResp, call := e.execute(ctx)
	e.publishCallEvent(ctx, call, time.Since(started))
	return execResp, nil
}

// execute runs the step and returns its response with the record of the call.
func (e *extensionExecutor) execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, *extensionCall) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}
	props, invalidProperty := common.ParseExtensionProperties(ctx.NodeProperties)
	call := &extensionCall{connectionID: props.ConnectionID}
	if invalidProperty != "" {
		logger.Error(ctx.Context, "Extension step has an unusable property value",
			log.String("property", invalidProperty))
		call.outcome = extensionOutcomeConfigError
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrExtensionConfigInvalid
		return execResp, call
	}
	attemptKey := common.ExtensionAttemptCountKey(ctx.CurrentNodeID)
	attempts := extensionAttempts(ctx.RuntimeData[attemptKey], props.MaxAttempts)
	if attempts >= props.MaxAttempts {
		logger.Debug(ctx.Context, "Extension step reached its attempt limit", log.Int("attempts", attempts))
		call.outcome = extensionOutcomeAttemptsExceeded
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrExtensionAttemptsExceeded
		return execResp, call
	}

	var body map[string]interface{}
	var kind extensionErrorKind
	connection, svcErr := e.resolveConnection(ctx.Context, logger, props.ConnectionID)
	switch {
	case svcErr == nil:
		body, kind = e.call(ctx, props, connection, call)
		// The request may have sent the step's earlier answer. Once the service has answered, the
		// namespace holds only this answer; without an answer, the earlier one stays.
		if call.httpStatus != 0 {
			clearExtensionNamespace(ctx.RuntimeData, ctx.CurrentNodeID)
		}
	case svcErr.Code == ErrExtensionConfigInvalid.Code:
		call.outcome = extensionOutcomeConfigError
		execResp.Status = providers.ExecFailure
		execResp.Error = svcErr
		return execResp, call
	default:
		kind = extensionErrorKindInternal
	}
	if kind == "" {
		call.outcome, kind = evaluateExtensionResponse(body)
	}
	if kind == "" && (call.outcome == extensionOutcomeSuccess || call.outcome == extensionOutcomeIncomplete) {
		if key, found := firstReservedKey(body, props.ResponseKeys); found {
			logger.Error(ctx.Context, "Extension response carries a reserved runtime data key",
				log.String("key", key))
			kind = extensionErrorKindReservedKey
		}
	}
	reason := failureReason(body)
	call.failureReason = reason

	if kind != "" || call.outcome == extensionOutcomeError {
		logger.Error(ctx.Context, "Extension call ended in error", log.String("errorKind", string(kind)),
			log.Int("httpStatus", call.httpStatus), log.String("failureReason", reason))
		call.outcome = extensionOutcomeError
		call.errorKind = kind
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrExtensionCallFailed
		return execResp, call
	}

	description := cleanFailureDescription(body[extensionFieldFailureDescription])
	switch call.outcome {
	case extensionOutcomeSuccess:
		execResp.RuntimeData = extensionRuntimeData(body, ctx.CurrentNodeID, props.ResponseKeys)
		// A failure stored by an earlier attempt must not outlive this success.
		for _, key := range []string{extensionFieldFailureReason, extensionFieldFailureDescription} {
			if ctx.RuntimeData[key] != "" {
				execResp.RuntimeData[key] = ""
			}
		}
		execResp.Status = providers.ExecComplete
	case extensionOutcomeFailed:
		execResp.Error = failedError(reason, description)
		execResp.RuntimeData[extensionFieldFailureReason] = reason
		execResp.RuntimeData[extensionFieldFailureDescription] = execResp.Error.ErrorDescription.String()
		execResp.RuntimeData[attemptKey] = strconv.Itoa(attempts + 1)
		execResp.Status = providers.ExecFailure
	case extensionOutcomeIncomplete:
		execResp.RuntimeData = extensionRuntimeData(body, ctx.CurrentNodeID, props.ResponseKeys)
		execResp.Error = incompleteError(reason, description)
		execResp.RuntimeData[extensionFieldFailureReason] = reason
		execResp.RuntimeData[extensionFieldFailureDescription] = ""
		if execResp.Error != nil {
			execResp.RuntimeData[extensionFieldFailureDescription] = execResp.Error.ErrorDescription.String()
		}
		execResp.RuntimeData[attemptKey] = strconv.Itoa(attempts + 1)
		execResp.Status = providers.ExecUserInputRequired
	}
	logger.Debug(ctx.Context, "Extension call completed", log.String("outcome", string(call.outcome)))
	return execResp, call
}

// extensionAttempts returns the FAILED and INCOMPLETE answers the step has had in this execution.
// A count the executor could not have written reads as the limit, so the step fails closed.
func extensionAttempts(count string, maxAttempts int) int {
	if count == "" {
		return 0
	}
	attempts, err := strconv.Atoi(count)
	if err != nil {
		return maxAttempts
	}
	return attempts
}

// extensionNamespace returns the prefix of the runtime-data keys the step with nodeID stores.
func extensionNamespace(nodeID string) string {
	return common.ExtensionDataKeyPrefix + nodeID + "."
}

// clearExtensionNamespace removes every key the step stored from an earlier answer.
func clearExtensionNamespace(runtimeData map[string]string, nodeID string) {
	prefix := extensionNamespace(nodeID)
	maps.DeleteFunc(runtimeData, func(key, _ string) bool {
		return strings.HasPrefix(key, prefix)
	})
}

// resolveConnection returns the step's connection, resolved on every execution so that a changed
// connection takes effect without editing the flow. A step whose connection does not exist, or that
// runs without a connection resolver, is misconfigured and returns ErrExtensionConfigInvalid; a
// connection that exists but cannot be resolved returns ErrExtensionCallFailed.
func (e *extensionExecutor) resolveConnection(ctx context.Context, logger *log.Logger, connectionID string) (
	*httpconnection.ResolvedHTTPConnection, *tidcommon.ServiceError) {
	if e.connectionResolver == nil {
		logger.Error(ctx, "Extension step runs without a connection resolver",
			log.String("connectionID", connectionID))
		return nil, &ErrExtensionConfigInvalid
	}
	connection, svcErr := e.connectionResolver.GetResolvedConnection(ctx, connectionID)
	if svcErr == nil {
		return connection, nil
	}
	if svcErr.Code == httpconnection.ErrorNotFound.Code {
		logger.Error(ctx, "Extension step's connection does not exist", log.String("connectionID", connectionID))
		return nil, &ErrExtensionConfigInvalid
	}
	logger.Error(ctx, "Extension step's connection cannot be resolved", log.String("connectionID", connectionID),
		log.String("errorCode", svcErr.Code))
	return nil, &ErrExtensionCallFailed
}

// call posts the request to the connection and returns the response body as a JSON object, or the
// kind of error that stopped it. The status code is recorded on call.
func (e *extensionExecutor) call(ctx *providers.NodeContext, props common.ExtensionProperties,
	connection *httpconnection.ResolvedHTTPConnection, call *extensionCall) (
	map[string]interface{}, extensionErrorKind) {
	payload, err := json.Marshal(newExtensionRequest(ctx, props))
	if err != nil {
		return nil, extensionErrorKindInternal
	}
	callCtx, cancel := context.WithTimeout(ctx.Context, connection.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, connection.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, extensionErrorKindInternal
	}
	req.Header = connection.Headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
	if req.Header.Get(serverconst.CorrelationIDHeaderName) == "" {
		req.Header.Set(serverconst.CorrelationIDHeaderName, sysContext.GetTraceID(ctx.Context))
	}

	httpResp, err := e.httpClient().Do(req)
	if err != nil {
		return nil, transportErrorKind(err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()
	call.httpStatus = httpResp.StatusCode
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return nil, extensionErrorKindStatus
	}

	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, extensionMaxResponseBytes+1))
	if err != nil {
		return nil, transportErrorKind(err)
	}
	if len(raw) > extensionMaxResponseBytes {
		return nil, extensionErrorKindBody
	}
	body, ok := decodeJSONObject(raw)
	if !ok {
		return nil, extensionErrorKindBody
	}
	return body, ""
}

// httpClient returns the client every call shares. It is built on first use rather than with the
// executor, because building it reads the server's TLS settings, which executors registered without
// a server runtime do not have. Following a redirect would resend the headers, credential included,
// to another host, so the client never follows one. Private addresses are allowed because only
// administrators set the endpoint. Each call bounds itself with the connection's timeout.
func (e *extensionExecutor) httpClient() httpservice.HTTPClientInterface {
	e.clientOnce.Do(func() {
		e.client = httpservice.NewHTTPClientWithoutRedirects(0, false)
	})
	return e.client
}

// transportErrorKind classifies a failure to complete the exchange.
func transportErrorKind(err error) extensionErrorKind {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return extensionErrorKindTimeout
	}
	return extensionErrorKindTransport
}

// decodeJSONObject decodes raw as exactly one JSON object, keeping numbers as their source text.
func decodeJSONObject(raw []byte) (map[string]interface{}, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var body map[string]interface{}
	if err := decoder.Decode(&body); err != nil || body == nil {
		return nil, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	return body, true
}

// evaluateExtensionResponse returns the outcome a well-formed body reports, or ERROR with the kind of
// contract breach. A redirectUrl with INCOMPLETE is a breach: this build cannot send the browser to
// the service's page, so it fails rather than continue as though the service had not asked.
func evaluateExtensionResponse(body map[string]interface{}) (extensionOutcome, extensionErrorKind) {
	value, ok := body[extensionFieldActionStatus].(string)
	if !ok {
		return extensionOutcomeError, extensionErrorKindContract
	}
	switch outcome := extensionOutcome(value); outcome {
	case extensionOutcomeSuccess, extensionOutcomeFailed:
		return outcome, ""
	case extensionOutcomeIncomplete:
		if redirectURL, present := body[extensionFieldRedirectURL]; present && redirectURL != nil &&
			redirectURL != "" {
			return extensionOutcomeError, extensionErrorKindContract
		}
		return outcome, ""
	case extensionOutcomeError:
		return extensionOutcomeError, ""
	}
	return extensionOutcomeError, extensionErrorKindContract
}

// firstReservedKey returns a field the step would store, as extensionRuntimeData selects them, that
// names a reserved runtime-data key.
func firstReservedKey(body map[string]interface{}, responseKeys []string) (string, bool) {
	for key := range extensionStoredFields(body, responseKeys) {
		if isReservedRuntimeKey(key) {
			return key, true
		}
	}
	return "", false
}

// failureReason returns the body's failureReason when it has the allowed form, otherwise "".
func failureReason(body map[string]interface{}) string {
	reason, ok := body[extensionFieldFailureReason].(string)
	if !ok || !extensionFailureReasonPattern.MatchString(reason) {
		return ""
	}
	return reason
}

// cleanFailureDescription returns failureDescription as bounded plain text: format characters, such
// as bidirectional overrides and zero-width characters, and control characters other than
// whitespace are removed, whitespace runs collapse to one space, and the result is cut to the shown
// length.
func cleanFailureDescription(value interface{}) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	text = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) || (unicode.IsControl(r) && !unicode.IsSpace(r)) {
			return -1
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > extensionMaxFailureDescriptionLength {
		text = strings.TrimSpace(string(runes[:extensionMaxFailureDescriptionLength]))
	}
	return text
}

// isCredentialFailure reports whether reason is one that returns ThunderID's own credential error,
// as the built-in credential steps do, so an extension cannot reveal which accounts exist.
func isCredentialFailure(reason string) bool {
	return reason == extensionReasonInvalidCredentials || reason == extensionReasonUserNotFound
}

// failedError returns the error a FAILED answer shows.
func failedError(reason, description string) *tidcommon.ServiceError {
	if isCredentialFailure(reason) {
		return &ErrInvalidCredentials
	}
	err := &ErrExtensionActionFailed
	if description == "" {
		description = extensionRejectedDescription.DefaultValue
		err = tidcommon.CustomServiceError(ErrExtensionActionFailed, extensionRejectedDescription)
	}
	return err.WithParams(extensionFailureParams(reason, description))
}

// incompleteError returns the error an INCOMPLETE answer shows, or nil when it carries no message.
func incompleteError(reason, description string) *tidcommon.ServiceError {
	if isCredentialFailure(reason) {
		return &ErrInvalidCredentials
	}
	if description == "" {
		return nil
	}
	return ErrExtensionInputRequired.WithParams(extensionFailureParams(reason, description))
}

// extensionFailureParams returns the description params of a FAILED or INCOMPLETE error: the shown
// text, and the reason when it is valid.
func extensionFailureParams(reason, description string) map[string]string {
	params := map[string]string{extensionParamDescription: description}
	if reason != "" {
		params[extensionParamReason] = reason
	}
	return params
}

// newExtensionRequest builds the request body from the node context, with the user inputs and runtime
// data the step selects.
func newExtensionRequest(ctx *providers.NodeContext, props common.ExtensionProperties) extensionRequest {
	request := extensionRequest{
		Version:       extensionContractVersion,
		ExecutionID:   ctx.ExecutionID,
		FlowType:      ctx.FlowType,
		CurrentAction: ctx.CurrentAction,
		CurrentNodeID: ctx.CurrentNodeID,
		UserInputs:    selectPresent(ctx.UserInputs, props.InputKeys, nil),
		RuntimeData:   selectExtensionRuntimeData(ctx.RuntimeData, props.RuntimeDataKeys),
		Application: extensionApplication{
			ID:   ctx.Application.ID,
			OUID: ctx.Application.OUID,
			Name: ctx.Application.Name,
			Type: ctx.Application.Type,
		},
		AuthUser: make(map[string]extensionAuthState),
	}
	for _, name := range ctx.AuthUser.ProviderNames() {
		state, _ := ctx.AuthUser.StateFor(name)
		request.AuthUser[name] = extensionAuthState{EntityReference: state.EntityReference}
	}
	return request
}

// selectExtensionRuntimeData returns the runtime data a service receives: the keys runtimeDataKeys
// names, or the default keys without it, never a secret one.
func selectExtensionRuntimeData(runtimeData map[string]string, runtimeDataKeys []string) map[string]string {
	if len(runtimeDataKeys) == 0 {
		runtimeDataKeys = extensionDefaultRuntimeKeys
	}
	return selectPresent(runtimeData, runtimeDataKeys, isSecretRuntimeKey)
}

// selectPresent returns the named entries of values that are present and not excluded.
func selectPresent(values map[string]string, keys []string, excluded func(string) bool) map[string]string {
	selected := make(map[string]string)
	for _, key := range keys {
		if excluded != nil && excluded(key) {
			continue
		}
		if value, ok := values[key]; ok {
			selected[key] = value
		}
	}
	return selected
}

// extensionRuntimeData renders the top-level response fields for runtime data under the step's
// namespace, extension.<nodeId>.<field>, leaving out the contract fields, null values and any field
// outside responseKeys when it is set.
func extensionRuntimeData(body map[string]interface{}, nodeID string, responseKeys []string) map[string]string {
	prefix := extensionNamespace(nodeID)
	data := make(map[string]string)
	for key, value := range extensionStoredFields(body, responseKeys) {
		if text, ok := extensionValueText(value); ok {
			data[prefix+key] = text
		}
	}
	return data
}

// extensionStoredFields returns the top-level fields of body the step stores: every field outside
// the contract that is not null, limited to responseKeys when it names any.
func extensionStoredFields(body map[string]interface{}, responseKeys []string) map[string]interface{} {
	fields := make(map[string]interface{}, len(body))
	for key, value := range body {
		switch key {
		case extensionFieldActionStatus, extensionFieldFailureReason, extensionFieldFailureDescription,
			extensionFieldRedirectURL:
			continue
		}
		if value == nil || (len(responseKeys) > 0 && !slices.Contains(responseKeys, key)) {
			continue
		}
		fields[key] = value
	}
	return fields
}

// extensionValueText renders a JSON value as stored: strings as they are, numbers in their source
// text, and everything else as compact JSON.
func extensionValueText(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

// publishCallEvent publishes the audit event of one execution. It carries no URL, header value,
// body, user input, runtime-data value or failureDescription.
func (e *extensionExecutor) publishCallEvent(ctx *providers.NodeContext, call *extensionCall,
	elapsed time.Duration) {
	if e.observabilitySvc == nil || !e.observabilitySvc.IsEnabled() {
		return
	}
	status := providers.StatusFailure
	switch call.outcome {
	case extensionOutcomeSuccess:
		status = providers.StatusSuccess
	case extensionOutcomeIncomplete:
		status = providers.StatusPending
	}
	evt := event.NewEvent(ctx.ExecutionID, string(event.EventTypeExtensionCall), event.ComponentExtensionExecutor).
		WithStatus(status).
		WithData(event.DataKey.ExecutionID, ctx.ExecutionID).
		WithData(event.DataKey.NodeID, ctx.CurrentNodeID).
		WithData(event.DataKey.FlowType, string(ctx.FlowType)).
		WithData(event.DataKey.ConnectionID, call.connectionID).
		WithData(event.DataKey.Outcome, string(call.outcome)).
		WithData(event.DataKey.DurationMs, elapsed.Milliseconds())
	if call.failureReason != "" {
		evt = evt.WithData(event.DataKey.FailureReason, call.failureReason)
	}
	if call.httpStatus != 0 {
		evt = evt.WithData(event.DataKey.HTTPStatus, call.httpStatus)
	}
	if call.errorKind != "" {
		evt = evt.WithData(event.DataKey.ErrorKind, string(call.errorKind))
	}
	e.observabilitySvc.PublishEvent(ctx.Context, evt)
}
