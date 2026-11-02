// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
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

	// extensionParamReason is the param of the FAILED and INCOMPLETE errors that carries failureReason.
	extensionParamReason = "reason"

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
	extensionErrorKindTimeout   extensionErrorKind = "timeout"
	extensionErrorKindTransport extensionErrorKind = "transport"
	extensionErrorKindStatus    extensionErrorKind = "status"
	extensionErrorKindBody      extensionErrorKind = "body"
	extensionErrorKindContract  extensionErrorKind = "contract"
	// extensionErrorKindInternal is a failure inside ThunderID, such as a connection that cannot be
	// resolved or a request that cannot be built.
	extensionErrorKindInternal extensionErrorKind = "internal"
)

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
	started       time.Time
	connectionID  string
	outcome       extensionOutcome
	errorKind     extensionErrorKind
	failureReason string
	httpStatus    int
}

// extensionExecutor calls an organization's service through an HTTP endpoint connection and acts on
// the actionStatus it answers with. It only adds data to the flow and never sets the authenticated
// user.
type extensionExecutor struct {
	providers.Executor
	connectionProvider httpconnection.HTTPConnectionProvider
	client             httpservice.HTTPClientInterface
	observabilitySvc   providers.ObservabilityProvider
	logger             *log.Logger
}

var _ providers.Executor = (*extensionExecutor)(nil)

// newExtensionExecutor creates an extension executor.
func newExtensionExecutor(flowFactory core.FlowFactoryInterface,
	connectionProvider httpconnection.HTTPConnectionProvider, client httpservice.HTTPClientInterface,
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
		connectionProvider: connectionProvider,
		client:             client,
		observabilitySvc:   observabilitySvc,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameExtension),
			log.String(log.LoggerKeyExecutorName, ExecutorNameExtension)),
	}
}

// Execute calls the step's service and routes on its answer. A failure inside ThunderID is returned
// as an error; every execution publishes one audit event.
func (e *extensionExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	call := &extensionCall{started: time.Now()}
	defer e.publishCallEvent(ctx, call)

	props, err := e.readProperties(ctx, call)
	if err != nil {
		return nil, err
	}
	attempts, err := e.attempts(ctx, call)
	if err != nil {
		return nil, err
	}
	if attempts >= props.MaxAttempts {
		return e.attemptsExceeded(ctx, call), nil
	}
	connection, err := e.resolveConnection(ctx.Context, props.ConnectionID, call)
	if err != nil {
		return nil, err
	}
	body, err := e.call(ctx, props, connection, call)
	if err != nil {
		return nil, err
	}
	// The request may have sent the step's earlier answer; once the service answers, it replaces it.
	if call.httpStatus != 0 {
		e.clearNamespace(ctx)
	}
	e.evaluate(body, call)
	return e.respond(ctx, props, body, attempts, call), nil
}

// readProperties returns the step's properties, or an error when one is unusable.
func (e *extensionExecutor) readProperties(ctx *providers.NodeContext, call *extensionCall) (
	common.ExtensionProperties, error) {
	props, invalidProperty := common.ParseExtensionProperties(ctx.NodeProperties)
	call.connectionID = props.ConnectionID
	if invalidProperty != "" {
		call.outcome = extensionOutcomeConfigError
		return props, fmt.Errorf("extension step property %s has an unusable value", invalidProperty)
	}
	return props, nil
}

// attempts returns the FAILED and INCOMPLETE answers the step has had in this execution.
func (e *extensionExecutor) attempts(ctx *providers.NodeContext, call *extensionCall) (int, error) {
	count := ctx.RuntimeData[common.ExtensionAttemptCountKey(ctx.CurrentNodeID)]
	if count == "" {
		return 0, nil
	}
	attempts, err := strconv.Atoi(count)
	if err != nil || attempts < 0 {
		call.outcome, call.errorKind = extensionOutcomeError, extensionErrorKindInternal
		return 0, errors.New("invalid extension attempt count")
	}
	return attempts, nil
}

// attemptsExceeded returns the response of a step that has reached its attempt limit.
func (e *extensionExecutor) attemptsExceeded(ctx *providers.NodeContext,
	call *extensionCall) *providers.ExecutorResponse {
	e.logger.Debug(ctx.Context, "Extension step reached its attempt limit",
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	call.outcome = extensionOutcomeAttemptsExceeded
	return &providers.ExecutorResponse{
		Status:         providers.ExecFailure,
		Error:          &ErrExtensionAttemptsExceeded,
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}
}

// resolveConnection returns the step's connection, resolved on every execution so that a changed
// connection takes effect without editing the flow.
func (e *extensionExecutor) resolveConnection(ctx context.Context, connectionID string, call *extensionCall) (
	*httpconnection.ResolvedHTTPConnection, error) {
	if e.connectionProvider == nil || e.client == nil {
		call.outcome = extensionOutcomeConfigError
		return nil, errors.New("extension step runs without an HTTP connection provider")
	}
	connection, svcErr := e.connectionProvider.GetResolvedConnection(ctx, connectionID)
	if svcErr == nil {
		return connection, nil
	}
	if svcErr.Code == httpconnection.ErrorNotFound.Code {
		call.outcome = extensionOutcomeConfigError
		return nil, fmt.Errorf("extension step's HTTP connection %s does not exist", connectionID)
	}
	call.outcome, call.errorKind = extensionOutcomeError, extensionErrorKindInternal
	return nil, fmt.Errorf("failed to resolve the extension step's HTTP connection %s: %s", connectionID,
		svcErr.Code)
}

// call posts the request to the connection and returns the response body as a JSON object. A failure
// of the service is recorded on call; only a failure to build the request is returned as an error.
func (e *extensionExecutor) call(ctx *providers.NodeContext, props common.ExtensionProperties,
	connection *httpconnection.ResolvedHTTPConnection, call *extensionCall) (map[string]interface{}, error) {
	payload, err := json.Marshal(e.buildRequest(ctx, props))
	if err != nil {
		call.outcome, call.errorKind = extensionOutcomeError, extensionErrorKindInternal
		return nil, fmt.Errorf("failed to encode the extension request: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx.Context, connection.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, connection.URL, bytes.NewReader(payload))
	if err != nil {
		// The error carries the URL, which can hold a credential, so it is not wrapped.
		call.outcome, call.errorKind = extensionOutcomeError, extensionErrorKindInternal
		return nil, errors.New("failed to build the extension request")
	}
	req.Header = connection.Headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
	if req.Header.Get(serverconst.CorrelationIDHeaderName) == "" {
		req.Header.Set(serverconst.CorrelationIDHeaderName, sysContext.GetTraceID(ctx.Context))
	}

	httpResp, err := e.client.Do(req)
	if err != nil {
		call.errorKind = e.transportErrorKind(err)
		return nil, nil
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()
	call.httpStatus = httpResp.StatusCode
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		call.errorKind = extensionErrorKindStatus
		return nil, nil
	}

	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, extensionMaxResponseBytes+1))
	if err != nil {
		call.errorKind = e.transportErrorKind(err)
		return nil, nil
	}
	if len(raw) > extensionMaxResponseBytes {
		call.errorKind = extensionErrorKindBody
		return nil, nil
	}
	body, ok := e.decodeJSONObject(raw)
	if !ok {
		call.errorKind = extensionErrorKindBody
		return nil, nil
	}
	return body, nil
}

// transportErrorKind classifies a failure to complete the exchange.
func (e *extensionExecutor) transportErrorKind(err error) extensionErrorKind {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return extensionErrorKindTimeout
	}
	return extensionErrorKindTransport
}

// decodeJSONObject decodes raw as exactly one JSON object, keeping numbers as their source text.
func (e *extensionExecutor) decodeJSONObject(raw []byte) (map[string]interface{}, bool) {
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

// clearNamespace removes every key the step stored from an earlier answer.
func (e *extensionExecutor) clearNamespace(ctx *providers.NodeContext) {
	prefix := e.namespace(ctx.CurrentNodeID)
	maps.DeleteFunc(ctx.RuntimeData, func(key, _ string) bool {
		return strings.HasPrefix(key, prefix)
	})
}

// namespace returns the prefix of the runtime-data keys the step with nodeID stores.
func (e *extensionExecutor) namespace(nodeID string) string {
	return common.ExtensionDataKeyPrefix + nodeID + "."
}

// evaluate records on call the outcome of the service's answer and its failure reason.
func (e *extensionExecutor) evaluate(body map[string]interface{}, call *extensionCall) {
	call.failureReason = e.failureReason(body)
	if call.errorKind != "" {
		call.outcome = extensionOutcomeError
		return
	}
	call.outcome, call.errorKind = e.answerOutcome(body)
}

// answerOutcome returns the outcome a well-formed body reports, or ERROR with the kind of contract
// breach. This build cannot send the browser to the service's page, so a redirectUrl with INCOMPLETE
// is a breach rather than ignored.
func (e *extensionExecutor) answerOutcome(body map[string]interface{}) (extensionOutcome, extensionErrorKind) {
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

// respond returns the executor response for the outcome recorded on call.
func (e *extensionExecutor) respond(ctx *providers.NodeContext, props common.ExtensionProperties,
	body map[string]interface{}, attempts int, call *extensionCall) *providers.ExecutorResponse {
	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}
	description := e.cleanFailureDescription(body[extensionFieldFailureDescription])
	attemptKey := common.ExtensionAttemptCountKey(ctx.CurrentNodeID)
	switch call.outcome {
	case extensionOutcomeSuccess:
		execResp.RuntimeData = e.answerRuntimeData(body, ctx.CurrentNodeID, props.ResponseKeys)
		execResp.Status = providers.ExecComplete
	case extensionOutcomeFailed:
		execResp.Error = e.answerError(ErrExtensionActionFailed, call.failureReason, description)
		execResp.RuntimeData[attemptKey] = strconv.Itoa(attempts + 1)
		execResp.Status = providers.ExecFailure
	case extensionOutcomeIncomplete:
		execResp.RuntimeData = e.answerRuntimeData(body, ctx.CurrentNodeID, props.ResponseKeys)
		if description != "" || e.isCredentialFailure(call.failureReason) {
			execResp.Error = e.answerError(ErrExtensionInputRequired, call.failureReason, description)
		}
		execResp.RuntimeData[attemptKey] = strconv.Itoa(attempts + 1)
		execResp.Status = providers.ExecUserInputRequired
	default:
		execResp.Error = &ErrExtensionCallFailed
		execResp.Status = providers.ExecFailure
	}
	e.logger.Debug(ctx.Context, "Extension call completed", log.String(log.LoggerKeyExecutionID, ctx.ExecutionID),
		log.String("outcome", string(call.outcome)))
	return execResp
}

// failureReason returns the body's failureReason when it has the allowed form, otherwise "".
func (e *extensionExecutor) failureReason(body map[string]interface{}) string {
	reason, ok := body[extensionFieldFailureReason].(string)
	if !ok || !extensionFailureReasonPattern.MatchString(reason) {
		return ""
	}
	return reason
}

// cleanFailureDescription returns failureDescription as bounded plain text, without the format and
// control characters that could disguise it.
func (e *extensionExecutor) cleanFailureDescription(value interface{}) string {
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

// isCredentialFailure reports whether reason returns ThunderID's own credential error, as the
// built-in credential steps do, so an extension cannot reveal which accounts exist.
func (e *extensionExecutor) isCredentialFailure(reason string) bool {
	return reason == extensionReasonInvalidCredentials || reason == extensionReasonUserNotFound
}

// answerError returns the error a FAILED or INCOMPLETE answer shows: base with the service's
// description when it sent one, and its reason as a param.
func (e *extensionExecutor) answerError(base tidcommon.ServiceError,
	reason, description string) *tidcommon.ServiceError {
	if e.isCredentialFailure(reason) {
		return &ErrInvalidCredentials
	}
	var params map[string]string
	if reason != "" {
		params = map[string]string{extensionParamReason: reason}
	}
	if description == "" {
		return base.WithParams(params)
	}
	return tidcommon.CustomServiceError(base, tidcommon.I18nMessage{
		Key:          base.ErrorDescription.Key,
		DefaultValue: description,
		Params:       params,
	})
}

// buildRequest builds the request body from the node context, with the user inputs and runtime data
// the step selects.
func (e *extensionExecutor) buildRequest(ctx *providers.NodeContext,
	props common.ExtensionProperties) extensionRequest {
	request := extensionRequest{
		Version:       extensionContractVersion,
		ExecutionID:   ctx.ExecutionID,
		FlowType:      ctx.FlowType,
		CurrentAction: ctx.CurrentAction,
		CurrentNodeID: ctx.CurrentNodeID,
		UserInputs:    e.selectPresent(ctx.UserInputs, props.InputKeys),
		RuntimeData:   e.selectPresent(ctx.RuntimeData, props.RuntimeDataKeys),
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

// selectPresent returns the named entries of values that are present.
func (e *extensionExecutor) selectPresent(values map[string]string, keys []string) map[string]string {
	selected := make(map[string]string)
	for _, key := range keys {
		if value, ok := values[key]; ok {
			selected[key] = value
		}
	}
	return selected
}

// answerRuntimeData returns the top-level fields of the answer the step stores, under its namespace:
// every field outside the contract that is not null, limited to responseKeys when it names any.
func (e *extensionExecutor) answerRuntimeData(body map[string]interface{}, nodeID string,
	responseKeys []string) map[string]string {
	prefix := e.namespace(nodeID)
	data := make(map[string]string)
	for key, value := range body {
		switch key {
		case extensionFieldActionStatus, extensionFieldFailureReason, extensionFieldFailureDescription,
			extensionFieldRedirectURL:
			continue
		}
		if value == nil || (len(responseKeys) > 0 && !slices.Contains(responseKeys, key)) {
			continue
		}
		if text, ok := e.valueText(value); ok {
			data[prefix+key] = text
		}
	}
	return data
}

// valueText renders a JSON value as stored: strings as they are, numbers in their source text, and
// everything else as compact JSON.
func (e *extensionExecutor) valueText(value interface{}) (string, bool) {
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
func (e *extensionExecutor) publishCallEvent(ctx *providers.NodeContext, call *extensionCall) {
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
		WithData(event.DataKey.DurationMs, time.Since(call.started).Milliseconds())
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
