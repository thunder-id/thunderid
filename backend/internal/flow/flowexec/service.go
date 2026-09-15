// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowexec provides the FlowExecService interface and its implementation.
package flowexec

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/session"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// serverConfigProvider is the narrow subset of serverconfig.ServerConfigService consumed by flowexec
// for reading the "flow" section (per-type default handles and expiries). Defined locally so flowexec
// does not import the serverconfig package.
type serverConfigProvider interface {
	GetMergedConfig(ctx context.Context, name string) (any, *tidcommon.ServiceError)
}

// flowExecService is the implementation of FlowExecServiceInterface
type flowExecService struct {
	flowEngine          flowEngineInterface
	flowProvider        providers.FlowProvider
	graphBuilder        graphbuilder.GraphBuilderInterface
	flowStore           flowStoreInterface
	actorProvider       providers.ActorProvider
	observabilitySvc    providers.ObservabilityProvider
	transactioner       providers.Transactioner
	cryptoSvc           providers.RuntimeCryptoProvider
	attestationVerifier providers.AttestationProvider
	jwtService          jwt.JWTServiceInterface
	serverConfigSvc     serverConfigProvider
	cfg                 flowconfig.Config
}

// newFlowExecService creates a new instance of flowExecService with the provided dependencies.
func newFlowExecService(flowProvider providers.FlowProvider,
	flowStore flowStoreInterface, flowEngine flowEngineInterface,
	actorProvider providers.ActorProvider,
	observabilitySvc providers.ObservabilityProvider,
	transactioner providers.Transactioner,
	cryptoSvc providers.RuntimeCryptoProvider,
	attestationVerifier providers.AttestationProvider,
	graphBuilder graphbuilder.GraphBuilderInterface,
	jwtService jwt.JWTServiceInterface,
	serverConfigSvc serverConfigProvider,
	cfg flowconfig.Config) FlowExecServiceInterface {
	return &flowExecService{
		flowProvider:        flowProvider,
		flowStore:           flowStore,
		flowEngine:          flowEngine,
		actorProvider:       actorProvider,
		observabilitySvc:    observabilitySvc,
		transactioner:       transactioner,
		cryptoSvc:           cryptoSvc,
		attestationVerifier: attestationVerifier,
		graphBuilder:        graphBuilder,
		jwtService:          jwtService,
		serverConfigSvc:     serverConfigSvc,
		cfg:                 cfg,
	}
}

// Execute executes a flow with the given data
func (s *flowExecService) Execute(ctx context.Context,
	appID, executionID, flowType string, verbose bool,
	action string, inputs map[string]string, challengeToken, flowSecret, attestationToken string) (
	*FlowStep, *tidcommon.ServiceError) {
	return s.execute(ctx, "", appID, executionID, flowType, verbose, action, inputs,
		challengeToken, flowSecret, attestationToken)
}

// ExecuteByID starts or continues an administrator-selected flow by its immutable flow ID.
func (s *flowExecService) ExecuteByID(ctx context.Context, flowID, executionID string, verbose bool,
	action string, inputs map[string]string, challengeToken string) (*FlowStep, *tidcommon.ServiceError) {
	return s.execute(ctx, flowID, "", executionID, "", verbose, action, inputs, challengeToken, "", "")
}

func (s *flowExecService) execute(ctx context.Context, flowID, appID, executionID, flowType string, verbose bool,
	action string, inputs map[string]string, challengeToken, flowSecret, attestationToken string) (
	*FlowStep, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowExecService"))

	// Get trace ID from context
	traceID := sysContext.GetTraceID(ctx)

	var engineCtx *EngineContext
	var loadErr *tidcommon.ServiceError

	if isNewFlow(executionID) {
		engineCtx, loadErr = s.loadNewContext(ctx, flowID, appID, flowType, verbose, action, inputs,
			flowSecret, attestationToken, logger)
		if loadErr != nil {
			logger.Error(ctx, "Failed to load new flow context",
				log.String("appID", appID),
				log.String("flowType", flowType),
				log.String("error", loadErr.Error.DefaultValue))

			if s.observabilitySvc.IsEnabled() {
				evt := event.NewEvent(
					traceID,
					string(event.EventTypeFlowFailed),
					event.ComponentFlowEngine,
				).
					WithStatus(providers.StatusFailure).
					WithData(event.DataKey.EntityID, appID).
					WithData(event.DataKey.FlowType, flowType).
					// The flow never started, so there is no execution id to correlate on.
					WithData(event.DataKey.CorrelationID, traceID).
					WithData(event.DataKey.Error, processServiceErrorForEventPublish(loadErr))

				s.observabilitySvc.PublishEvent(ctx, evt)
			}
			return nil, loadErr
		}
	} else {
		engineCtx, loadErr = s.loadPrevContext(ctx, executionID, action, inputs, logger)
		if loadErr != nil {
			logger.Error(ctx, "Failed to load previous flow context",
				log.String(log.LoggerKeyExecutionID, executionID),
				log.String("error", loadErr.Error.DefaultValue))
			return nil, loadErr
		}
		setChallengeTokenInCtx(engineCtx, challengeToken)

		// Continuation re-checks the caller: the gate must hold on every step of an administration
		// flow, not only the one that started it. New flows are gated inside loadNewContext, as soon
		// as the flow type is known and before any lookup work is done on the caller's behalf.
		if svcErr := validateAdministrationCaller(ctx, engineCtx.FlowType); svcErr != nil {
			return nil, svcErr
		}
	}

	// Set trace ID to engine context (request context is already set during context loading)
	engineCtx.TraceID = traceID

	// Resolve the inbound SSO handle for this flow from the request-scoped transport inputs.
	applyInboundSSO(engineCtx, ctx)

	flowStep, flowErr := s.flowEngine.Execute(engineCtx)

	if flowErr != nil {
		if !isNewFlow(executionID) {
			if removeErr := s.removeContext(ctx, engineCtx.ExecutionID, logger); removeErr != nil {
				logger.Error(ctx, "Failed to remove flow context after engine failure",
					log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID), log.Error(removeErr))
				return nil, &tidcommon.InternalServerError
			}
		}
		// An engine failure is reported as a 4xx/5xx, which has no flow response to carry the error
		// assertion, so return a bare step alongside the error for the handler to serialize.
		errorType := common.FlowErrorTypeServer
		if flowErr.Type == tidcommon.ClientErrorType {
			errorType = common.FlowErrorTypeClient
		}
		if assertion := s.buildErrorAssertion(ctx, engineCtx, errorType,
			flowErr.ErrorDescription.String(), logger); assertion != "" {
			return &FlowStep{ErrorAssertion: assertion}, flowErr
		}
		return nil, flowErr
	}

	// Surface the OAuth callback type (grant type) from runtime data onto the terminal response so the
	// Gate/SDK routes the completion or failure to the correct callback handler (e.g. CIBA).
	// TODO: Remove once the OAuth callback handler can determine the grant type without runtime data.
	if flowStep.Status == providers.FlowStatusComplete || flowStep.Status == providers.FlowStatusError {
		if callbackType := engineCtx.RuntimeData[common.RuntimeKeyCallbackType]; callbackType != "" {
			if flowStep.Data.AdditionalData == nil {
				flowStep.Data.AdditionalData = make(map[string]string)
			}
			flowStep.Data.AdditionalData[common.DataCallbackType] = callbackType
		}
	}

	// Build a signed error assertion for an in-band flow failure so the OAuth callback can verify and
	// propagate it to the waiting authorization request.
	if flowStep.Status == providers.FlowStatusError {
		description := ""
		if flowStep.Error != nil {
			description = flowStep.Error.ErrorDescription.String()
		}
		flowStep.ErrorAssertion = s.buildErrorAssertion(ctx, engineCtx,
			common.FlowErrorTypeEndUser, description, logger)
	}

	if isComplete(flowStep) {
		if !isNewFlow(executionID) {
			if removeErr := s.removeContext(ctx, engineCtx.ExecutionID, logger); removeErr != nil {
				logger.Error(ctx, "Failed to remove flow context after completion",
					log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID), log.Error(removeErr))
				return nil, &tidcommon.InternalServerError
			}
		}
	} else {
		if isNewFlow(executionID) {
			if storeErr := s.storeContext(ctx, engineCtx, 0, logger); storeErr != nil {
				logger.Error(ctx, "Failed to store initial flow context",
					log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID), log.Error(storeErr))
				return nil, &tidcommon.InternalServerError
			}
		} else {
			if updateErr := s.updateContext(ctx, engineCtx, &flowStep, logger); updateErr != nil {
				logger.Error(ctx, "Failed to update flow context",
					log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID), log.Error(updateErr))
				return nil, &tidcommon.InternalServerError
			}
		}
	}

	return &flowStep, nil
}

// buildErrorAssertion signs an assertion binding the flow error type and description to the OAuth
// authorization request
func (s *flowExecService) buildErrorAssertion(ctx context.Context, engineCtx *EngineContext,
	errorType, description string, logger *log.Logger) string {
	authReqID := engineCtx.RuntimeData[common.RuntimeKeyAuthorizationRequestID]
	if authReqID == "" {
		return ""
	}

	// Bound to the same validity as the success assertion (AuthAssertExecutor), since both are
	// consumed by the callback within the same request cycle.
	validityPeriod := int64(0)
	if engineCtx.Application.Assertion != nil {
		validityPeriod = engineCtx.Application.Assertion.ValidityPeriod
	}

	claims := map[string]interface{}{
		"aud":                              engineCtx.AppID,
		common.ClaimAuthorizationRequestID: authReqID,
		common.ClaimFlowErrorType:          errorType,
		common.ClaimFlowErrorDescription:   description,
	}
	token, _, err := s.jwtService.GenerateJWT(ctx, "", "", validityPeriod, claims, jwt.TokenTypeJWT, "")
	if err != nil {
		logger.Error(ctx, "Failed to build flow error assertion",
			log.String("error", err.Error.DefaultValue))
		return ""
	}
	return token
}

// validateAdministrationCaller gates administration flows on an authenticated caller that holds the
// root system permission.
//
// The check is deliberately positive: it asserts what the caller must present rather than inferring
// it from the absence of the runtime marker. /flow/execute is a public path, so an unauthenticated
// request reaches the service with a runtime context, and an authenticated one only clears the
// middleware's authorization step because the path has no entry in the API permission table and
// therefore falls back to the root permission. Asserting the permission here keeps the gate intact
// if either of those tables changes.
//
// PermissionValidator remains in the administration flow templates as defense in depth; this is the
// boundary that must hold even for a hand-built flow that omits it.
func validateAdministrationCaller(ctx context.Context, flowType providers.FlowType) *tidcommon.ServiceError {
	if flowType != providers.FlowTypeAdministration {
		return nil
	}
	if security.IsRuntimeContext(ctx) || security.GetSubject(ctx) == "" {
		return &ErrorAdministrationAuthenticationRequired
	}
	if !security.HasSystemPermission(security.GetPermissions(ctx)) {
		return &ErrorAdministrationPermissionRequired
	}
	return nil
}

// applyInboundSSO selects the SSO handle carried for this flow from the request-scoped
// transport inputs and stashes it on the engine context for the SSO-Check node to consume.
// It is a no-op when no inbound transport is present.
func applyInboundSSO(engineCtx *EngineContext, ctx context.Context) {
	if engineCtx == nil || engineCtx.Graph == nil {
		return
	}
	inbound, ok := session.InboundFrom(ctx)
	if !ok {
		return
	}
	// ssoFlowID resolves the flow whose session this execution operates on — the running flow for
	// login/SSO, or the login flow (SessionFlowID) for a sign-out flow — so the correct per-flow cookie
	// is selected.
	engineCtx.SSOHandleIn = inbound.HandleFor(ssoFlowID(engineCtx))
}

// initContext initializes a new flow context with the given details.
func (s *flowExecService) loadNewContext(ctx context.Context, flowID, appID, flowTypeStr string, verbose bool,
	action string, inputs map[string]string, flowSecret, attestationToken string, logger *log.Logger) (
	*EngineContext, *tidcommon.ServiceError) {
	var flowType providers.FlowType
	var resolvedFlow *providers.CompleteFlowDefinition
	if flowID == "" {
		var err *tidcommon.ServiceError
		flowType, err = validateFlowType(flowTypeStr)
		if err != nil {
			return nil, err
		}
	} else {
		// Initiating by flow ID is an administration-only entry point by construction, so the caller is
		// gated before the flow is resolved. Checking the flow type first would let an unauthenticated
		// caller on this public path distinguish a missing flow from an existing one, and an
		// administration flow from any other, purely from the error it gets back.
		if svcErr := validateAdministrationCaller(ctx, providers.FlowTypeAdministration); svcErr != nil {
			return nil, svcErr
		}
		flow, svcErr := s.flowProvider.GetFlow(ctx, flowID)
		if svcErr != nil {
			return nil, svcErr
		}
		// The flow the ID resolves to must itself be an administration flow: the entry point must not
		// become a way to start an authentication, registration, recovery or sign-out flow directly,
		// bypassing the application binding and initiation guards those types require.
		if flow.FlowType != providers.FlowTypeAdministration {
			return nil, &ErrorFlowIDExecutionNotPermitted
		}
		flowType = flow.FlowType
		resolvedFlow = flow
	}

	// Gate administration flows before any further resolution so an unauthenticated caller cannot
	// drive application lookups or context creation.
	if svcErr := validateAdministrationCaller(ctx, flowType); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := s.checkDirectFlowInitiationAllowed(
		ctx, appID, flowType, flowSecret, attestationToken, logger); svcErr != nil {
		return nil, svcErr
	}

	engineCtx, err := s.initContext(ctx, flowID, appID, flowType, resolvedFlow, verbose, logger)
	if err != nil {
		return nil, err
	}

	prepareContext(engineCtx, action, inputs)
	return engineCtx, nil
}

// checkDirectFlowInitiationAllowed governs which applications may initiate an authentication or
// sign-out flow directly over HTTP, based on how the application is classified:
//   - RedirectOnly — the application signs users in through a redirect-based protocol component
//     (currently OAuth 2.0 authorization_code apps) and must have its flows initiated by that
//     component, not via a direct HTTP call.
//   - FlowSecret — a backend / server-side application (including embedded apps with no protocol
//     profile) that must authenticate at flow initiation by presenting its Flow Secret.
//   - Attestation — a mobile application that authenticates at flow initiation by presenting a valid
//     platform attestation (e.g. a Google Play Integrity token) proving its binary identity.
//   - DevMode — a mobile application with attestation dev mode enabled, which may initiate a flow
//     without presenting an attestation. Disabled by default; intended for testing and trying out
//     sample/development mobile clients.
//
// Sign-out is guarded like authentication so a native caller must prove its identity before ending a
// session; a redirect-based app is pushed to the RP-initiated /oauth2/logout endpoint instead. Other
// flow types (registration, recovery, user onboarding) are not restricted. The classification is
// derived from neutral actor data resolved through the actor layer; credential verification
// (Flow Secret or attestation) is the only check that remains here.
func (s *flowExecService) checkDirectFlowInitiationAllowed(ctx context.Context, appID string,
	flowType providers.FlowType, flowSecret, attestationToken string,
	logger *log.Logger) *tidcommon.ServiceError {
	if flowType != providers.FlowTypeAuthentication && flowType != providers.FlowTypeSignOut {
		return nil
	}
	if appID == "" {
		return nil
	}

	mode, attestationCfg, svcErr := s.resolveFlowInitiationMode(ctx, appID)
	if svcErr != nil {
		if svcErr.Code == actorprovider.ErrorActorNotFound.Code {
			return &ErrorInvalidAppID
		}
		// Surface client errors (e.g. mobile attestation not configured) as-is.
		if svcErr.Type == tidcommon.ClientErrorType {
			return svcErr
		}
		logger.Error(ctx, "Failed to resolve flow initiation mode for guard",
			log.String("appID", appID))
		return &tidcommon.InternalServerError
	}

	switch mode {
	case flowInitiationNotPermitted:
		return &ErrorDirectFlowInitiationNotPermitted
	case flowInitiationFlowSecret:
		if flowSecret == "" {
			return &ErrorFlowSecretRequired
		}
		if authErr := s.actorProvider.AuthenticateActor(ctx,
			map[string]interface{}{authnprovidercm.UserAttributeUserID: appID},
			map[string]interface{}{fieldFlowSecret: flowSecret}); authErr != nil {
			logger.Debug(ctx, "Backend application provided an invalid flow secret",
				log.String("appID", appID))
			return &ErrorFlowSecretInvalid
		}
		return nil
	case flowInitiationAttestation:
		return s.verifyAttestation(ctx, attestationCfg, attestationToken)
	case flowInitiationDevMode:
		return nil
	default:
		logger.Error(ctx, "Unknown flow initiation mode for application",
			log.String("appID", appID))
		return &tidcommon.InternalServerError
	}
}

// verifyAttestation validates the platform attestation token presented by a mobile application at
// flow initiation. Decrypting the stored service account credentials, bounding the verification
// call with a deadline, and calling the Play Integrity API are all handled by the attestation
// provider.
func (s *flowExecService) verifyAttestation(ctx context.Context,
	attestationCfg *providers.AttestationConfig, attestationToken string) *tidcommon.ServiceError {
	if attestationToken == "" {
		return &ErrorAttestationRequired
	}

	verified, svcErr := s.attestationVerifier.Verify(ctx, attestationCfg, attestationToken)
	if svcErr != nil {
		return svcErr
	}
	if !verified {
		return &ErrorAttestationInvalid
	}
	return nil
}

// resolveFlowInitiationMode decides how an application may initiate a flow, based on its type. An
// unknown application returns ErrorActorNotFound so the caller can map it to an invalid app.
func (s *flowExecService) resolveFlowInitiationMode(
	ctx context.Context, appID string,
) (flowInitiationMode, *providers.AttestationConfig, *tidcommon.ServiceError) {
	client, clientErr := s.actorProvider.GetInboundClientByID(ctx, appID)
	if clientErr != nil {
		return 0, nil, clientErr
	}

	rawAppType, _ := client.Properties[applicationTypePropertyKey].(string)
	switch appmodel.ResolveApplicationType(rawAppType) {
	case appmodel.ApplicationTypeM2M, appmodel.ApplicationTypeBrowser:
		// M2M apps get tokens directly; browser apps are public redirect clients. Neither runs flows.
		return flowInitiationNotPermitted, nil, nil
	case appmodel.ApplicationTypeMobile:
		// Dev mode lets a mobile app initiate flows without a platform attestation, for testing or
		// trying out sample/development clients. Disabled by default.
		if client.Attestation != nil && client.Attestation.DevMode {
			return flowInitiationDevMode, nil, nil
		}
		// Otherwise, mobile apps authenticate with platform attestation, which must be configured first.
		if client.Attestation == nil || (client.Attestation.Android == nil && client.Attestation.Apple == nil) {
			return 0, nil, &ErrorAttestationNotConfigured
		}
		return flowInitiationAttestation, client.Attestation, nil
	default:
		// Full-stack, custom, and mcp apps may be embedded or redirect-based; derive from the OAuth profile.
		return s.resolveFlowInitiationModeFromProfile(ctx, appID)
	}
}

// resolveFlowInitiationModeFromProfile derives the mode from the OAuth profile, for types that do
// not encode embedded vs redirect in the type itself.
func (s *flowExecService) resolveFlowInitiationModeFromProfile(
	ctx context.Context, appID string,
) (flowInitiationMode, *providers.AttestationConfig, *tidcommon.ServiceError) {
	profile, svcErr := s.actorProvider.GetOAuthProfileByID(ctx, appID)
	if svcErr != nil && svcErr.Code != actorprovider.ErrorActorNotFound.Code {
		return 0, nil, svcErr
	}

	// No profile means an embedded app; it initiates flows directly with its Flow Secret.
	if profile == nil {
		return flowInitiationFlowSecret, nil, nil
	}

	// Redirect (authorization_code) apps initiate through the OAuth component; M2M apps get tokens
	// directly. Neither may initiate a flow directly.
	if slices.Contains(profile.GrantTypes, string(providers.GrantTypeAuthorizationCode)) ||
		isClientCredentialsOnly(profile.GrantTypes) {
		return flowInitiationNotPermitted, nil, nil
	}
	return flowInitiationFlowSecret, nil, nil
}

// isClientCredentialsOnly reports whether client_credentials is the only configured grant type.
func isClientCredentialsOnly(grantTypes []string) bool {
	return len(grantTypes) == 1 && grantTypes[0] == string(providers.GrantTypeClientCredentials)
}

// initContext initializes a new flow context with the given details.
func (s *flowExecService) initContext(ctx context.Context, requestedFlowID, appID string, flowType providers.FlowType,
	resolvedFlow *providers.CompleteFlowDefinition, verbose bool,
	logger *log.Logger) (*EngineContext, *tidcommon.ServiceError) {
	graphID := requestedFlowID
	if graphID == "" {
		var svcErr *tidcommon.ServiceError
		graphID, svcErr = s.getFlowGraph(ctx, appID, flowType, logger)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	engineCtx := EngineContext{}
	executionID, err := sysutils.GenerateUUIDv7()
	if err != nil {
		logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	engineCtx.ExecutionID = executionID

	flow := resolvedFlow
	var svcErr *tidcommon.ServiceError
	if flow == nil {
		flow, svcErr = s.flowProvider.GetFlow(ctx, graphID)
	}
	if svcErr != nil {
		// The configured flow may have been deleted while still referenced by the
		// application. For authentication flows, fall back to the default flow instead
		// of failing the request.
		flow, svcErr = s.fallbackToDefaultFlow(ctx, graphID, flowType, svcErr, logger)
		if svcErr != nil {
			return nil, svcErr
		}
		graphID = flow.ID
	}

	engineCtx.FlowType = flow.FlowType
	engineCtx.SSOFlowVersion = flow.ActiveVersion
	graph, svcErr := s.graphBuilder.GetGraph(ctx, flow)
	if svcErr != nil {
		logger.Error(ctx, "Error retrieving graph from graph builder",
			log.String("graphID", graphID), log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}
	engineCtx.Graph = graph
	engineCtx.Context = ctx
	engineCtx.AppID = appID
	engineCtx.Verbose = verbose

	// Set application context if required
	if err := s.setApplicationToContext(&engineCtx, logger); err != nil {
		return nil, err
	}

	return &engineCtx, nil
}

// fallbackToDefaultFlow attempts to recover from a failure to retrieve the configured flow.
// If the flow was not found (e.g. it was deleted while still referenced by the application)
// and the flow type is authentication, it falls back to the system default authentication
// flow. Any other case surfaces an internal server error.
func (s *flowExecService) fallbackToDefaultFlow(ctx context.Context, graphID string,
	flowType providers.FlowType, origErr *tidcommon.ServiceError, logger *log.Logger) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if flowType != providers.FlowTypeAuthentication || origErr.Type != tidcommon.ClientErrorType {
		logger.Error(ctx, "Error retrieving flow from flow provider",
			log.String("graphID", graphID), log.String("error", origErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	handle := s.resolveDefaultFlowHandle(ctx, providers.FlowTypeAuthentication)
	logger.Warn(ctx, "Configured authentication flow not found; falling back to default flow",
		log.String("graphID", graphID), log.String("defaultFlowHandle", handle))

	flow, svcErr := s.flowProvider.GetFlowByHandle(ctx, handle, providers.FlowTypeAuthentication)
	if svcErr != nil {
		logger.Error(ctx, "Failed to retrieve default authentication flow",
			log.String("defaultFlowHandle", handle), log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}
	return flow, nil
}

// loadPrevContext retrieves the flow context from the store based on the given details.
func (s *flowExecService) loadPrevContext(ctx context.Context, executionID, action string,
	inputs map[string]string, logger *log.Logger) (*EngineContext, *tidcommon.ServiceError) {
	engineCtx, err := s.loadContextFromStore(ctx, executionID, logger)
	if err != nil {
		return nil, err
	}

	prepareContext(engineCtx, action, inputs)
	return engineCtx, nil
}

// loadContextFromStore retrieves the flow context from the store based on the given details.
func (s *flowExecService) loadContextFromStore(ctx context.Context, executionID string, logger *log.Logger) (
	*EngineContext, *tidcommon.ServiceError) {
	if executionID == "" {
		return nil, &ErrorInvalidExecutionID
	}

	dbModel, flowCtxErr := s.getFlowContext(ctx, executionID, logger)
	if flowCtxErr != nil {
		return nil, flowCtxErr
	}

	graphID, err := dbModel.GetGraphID(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to extract graph ID from flow context",
			log.String(log.LoggerKeyExecutionID, executionID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	flow, svcErr := s.flowProvider.GetFlow(ctx, graphID)
	if svcErr != nil {
		logger.Error(ctx, "Error retrieving flow graph from flow provider",
			log.String("graphID", graphID), log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	graph, svcErr := s.graphBuilder.GetGraph(ctx, flow)
	if svcErr != nil {
		logger.Error(ctx, "Error retrieving graph from graph builder",
			log.String("graphID", graphID), log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	graphResolver := graphResolverFunc(func(rctx context.Context, flowID string) (core.GraphInterface, error) {
		f, svcErr := s.flowProvider.GetFlow(rctx, flowID)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to get flow %s: %s", flowID, svcErr.Error.DefaultValue)
		}
		g, svcErr := s.graphBuilder.GetGraph(rctx, f)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to build graph for flow %s: %s", flowID, svcErr.Error.DefaultValue)
		}
		return g, nil
	})

	engineContext, err := dbModel.ToEngineContext(ctx, graph, graphResolver)
	if err != nil {
		logger.Error(ctx, "Failed to convert flow context from database format",
			log.String(log.LoggerKeyExecutionID, executionID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	engineContext.SSOFlowVersion = flow.ActiveVersion

	// Set application context if required
	if err := s.setApplicationToContext(&engineContext, logger); err != nil {
		return nil, err
	}

	return &engineContext, nil
}

// setApplicationToContext loads the inbound-client / entity records for the flow's owning entity
// and assembles a providers.Application view onto engineCtx.Application. Entity-agnostic: works for
// any entity (application, agent, ...) that has an inbound-client row.
func (s *flowExecService) setApplicationToContext(engineCtx *EngineContext,
	logger *log.Logger) *tidcommon.ServiceError {
	// Skip application loading for app-independent flows
	if engineCtx.FlowType == providers.FlowTypeUserOnboarding ||
		engineCtx.FlowType == providers.FlowTypeAdministration {
		return nil
	}

	app, svcErr := actorprovider.BuildApplication(engineCtx.Context, s.actorProvider, engineCtx.AppID)
	if svcErr != nil {
		if svcErr.Code == actorprovider.ErrorActorNotFound.Code {
			return &ErrorInvalidAppID
		}
		logger.Error(engineCtx.Context, "Failed to build flow application",
			log.String("appID", engineCtx.AppID), log.String("errorCode", svcErr.Code))
		return svcErr
	}
	engineCtx.Application = *app

	// A sign-out flow runs a different flow than the one that owns the SSO session. Carry the login
	// (auth) flow id so the session is resolved, and its cookie cleared, under that flow rather than
	// the running sign-out flow. Re-derived here on every context load, so it is never persisted.
	if engineCtx.FlowType == providers.FlowTypeSignOut {
		engineCtx.SessionFlowID = engineCtx.Application.AuthFlowID
	}
	return nil
}

// removeContext removes the flow context from the store.
func (s *flowExecService) removeContext(ctx context.Context, executionID string, logger *log.Logger) error {
	if executionID == "" {
		return fmt.Errorf("flow ID cannot be empty")
	}

	txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.flowStore.DeleteFlowContext(txCtx, executionID)
	})
	if txErr != nil {
		return fmt.Errorf("failed to remove flow context from database: %w", txErr)
	}

	logger.Debug(ctx, "Flow context removed successfully from database",
		log.String(log.LoggerKeyExecutionID, executionID))
	return nil
}

// updateContext updates the flow context in the store based on the flow step status.
func (s *flowExecService) updateContext(ctx context.Context, engineCtx *EngineContext,
	flowStep *FlowStep, logger *log.Logger) error {
	if flowStep.Status == providers.FlowStatusComplete {
		return s.removeContext(ctx, engineCtx.ExecutionID, logger)
	} else {
		logger.Debug(ctx, "Flow execution is incomplete, updating the flow context",
			log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID))

		if engineCtx.ExecutionID == "" {
			return fmt.Errorf("flow ID cannot be empty")
		}

		encryptedEngineCtx, err := s.encryptEngineContext(ctx, engineCtx)
		if err != nil {
			return fmt.Errorf("failed to encrypt flow context: %w", err)
		}

		txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
			return s.flowStore.UpdateFlowContext(txCtx, *encryptedEngineCtx)
		})
		if txErr != nil {
			return fmt.Errorf("failed to update flow context in database: %w", txErr)
		}

		logger.Debug(ctx, "Flow context updated successfully in database",
			log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID))
		return nil
	}
}

// storeContext stores the flow context in the store.
func (s *flowExecService) storeContext(ctx context.Context, engineCtx *EngineContext,
	expirySeconds int64, logger *log.Logger) error {
	if engineCtx.ExecutionID == "" {
		return fmt.Errorf("flow ID cannot be empty")
	}

	if expirySeconds <= 0 {
		expirySeconds = s.getFlowExpirySeconds(ctx, engineCtx.FlowType)
	}

	encryptedEngineCtx, err := s.encryptEngineContext(ctx, engineCtx)
	if err != nil {
		return fmt.Errorf("failed to encrypt flow context: %w", err)
	}

	txErr := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.flowStore.StoreFlowContext(txCtx, *encryptedEngineCtx, expirySeconds)
	})
	if txErr != nil {
		return fmt.Errorf("failed to store flow context in database: %w", txErr)
	}

	logger.Debug(ctx, "Flow context stored successfully in database",
		log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID))
	return nil
}

// encryptEngineContext serializes an EngineContext and encrypts the context field, returning
// an EncryptedEngineContext ready to be handed to the store.
func (s *flowExecService) encryptEngineContext(ctx context.Context, engineCtx *EngineContext) (*FlowContextDB, error) {
	serialized := &FlowContextDB{}
	err := serialized.FromEngineContext(*engineCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize engine context: %w", err)
	}
	ciphertext, _, err := s.cryptoSvc.Encrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM),
		nil, []byte(serialized.Context))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt context: %w", err)
	}
	serialized.Context = string(ciphertext)
	return serialized, nil
}

// getFlowGraph checks if the provided entity ID is valid and returns the associated flow ID.
// Entity-agnostic: works for any entity (application, agent, ...) that has an inbound-client row.
func (s *flowExecService) getFlowGraph(ctx context.Context, appID string, flowType providers.FlowType,
	logger *log.Logger) (string, *tidcommon.ServiceError) {
	// Handle app-independent system flows
	if flowType == providers.FlowTypeUserOnboarding {
		return s.getSystemFlowGraph(ctx, flowType, logger)
	}

	if appID == "" {
		return "", &ErrorInvalidAppID
	}

	client, svcErr := s.actorProvider.GetInboundClientByID(ctx, appID)
	if svcErr != nil {
		if svcErr.Code == actorprovider.ErrorActorNotFound.Code {
			return "", &ErrorInvalidAppID
		}
		logger.Error(ctx, "Server error while retrieving inbound client",
			log.String("appID", appID), log.String("error", svcErr.Error.DefaultValue))
		return "", &tidcommon.InternalServerError
	}
	if client == nil {
		return "", &ErrorInvalidAppID
	}

	if flowType == providers.FlowTypeRegistration {
		if !client.IsRegistrationFlowEnabled {
			return "", &ErrorRegistrationFlowDisabled
		} else if client.RegistrationFlowID == "" {
			logger.Error(ctx, "Registration flow is not configured for the entity",
				log.String("appID", appID))
			return "", &tidcommon.InternalServerError
		}
		return client.RegistrationFlowID, nil
	}

	if flowType == providers.FlowTypeRecovery {
		if !client.IsRecoveryFlowEnabled {
			return "", &ErrorRecoveryFlowDisabled
		} else if client.RecoveryFlowID == "" {
			logger.Error(ctx, "Recovery flow is not configured for the application",
				log.String("appID", appID))
			return "", &tidcommon.InternalServerError
		}
		return client.RecoveryFlowID, nil
	}

	if flowType == providers.FlowTypeSignOut {
		if client.SignOutFlowID == "" {
			logger.Error(ctx, "Sign-out flow is not configured for the application",
				log.String("appID", appID))
			return "", &tidcommon.InternalServerError
		}
		return client.SignOutFlowID, nil
	}

	// Default to authentication flow ID
	if client.AuthFlowID == "" {
		logger.Error(ctx, "Authentication flow is not configured for the entity",
			log.String("appID", appID))
		return "", &tidcommon.InternalServerError
	}

	return client.AuthFlowID, nil
}

// validateFlowType validates the provided flow type string and returns the corresponding FlowType.
func validateFlowType(flowTypeStr string) (providers.FlowType, *tidcommon.ServiceError) {
	switch providers.FlowType(flowTypeStr) {
	case providers.FlowTypeAuthentication, providers.FlowTypeRegistration, providers.FlowTypeUserOnboarding,
		providers.FlowTypeRecovery, providers.FlowTypeSignOut, providers.FlowTypeAdministration:
		return providers.FlowType(flowTypeStr), nil
	default:
		return "", &ErrorInvalidFlowType
	}
}

// isNewFlow checks if the flow is a new flow based on the provided input.
func isNewFlow(executionID string) bool {
	return executionID == ""
}

// getSystemFlowGraph retrieves the flow graph for system flows by handle.
func (s *flowExecService) getSystemFlowGraph(ctx context.Context, flowType providers.FlowType,
	logger *log.Logger) (string, *tidcommon.ServiceError) {
	switch flowType {
	case providers.FlowTypeUserOnboarding:
	default:
		return "", &ErrorInvalidFlowType
	}

	handle := s.resolveDefaultFlowHandle(ctx, flowType)

	flow, err := s.flowProvider.GetFlowByHandle(ctx, handle, flowType)
	if err != nil {
		logger.Error(ctx, "Failed to get system flow by handle",
			log.String("handle", handle), log.String("flowType", string(flowType)))
		return "", err
	}
	return flow.ID, nil
}

// isComplete checks if the flow step status indicates completion.
func isComplete(step FlowStep) bool {
	return step.Status == providers.FlowStatusComplete
}

// prepareContext prepares the flow context by merging any data.
func prepareContext(ctx *EngineContext, action string, inputs map[string]string) {
	// Append any inputs present to the context
	if len(inputs) > 0 {
		ctx.UserInputs = sysutils.MergeStringMaps(ctx.UserInputs, inputs)
	}

	if ctx.UserInputs == nil {
		ctx.UserInputs = make(map[string]string)
	}
	if ctx.RuntimeData == nil {
		ctx.RuntimeData = make(map[string]string)
	}
	if ctx.InterceptorSharedData == nil {
		ctx.InterceptorSharedData = make(map[string]string)
	}

	// Set the action if provided
	if action != "" {
		ctx.CurrentAction = action
	}
}

// setChallengeTokenInCtx copies incoming request data from the engine context into the
// interceptor shared data so that interceptors can access it during execution.
func setChallengeTokenInCtx(ctx *EngineContext, challengeTokenIn string) {
	if challengeTokenIn != "" {
		if ctx.InterceptorSharedData == nil {
			ctx.InterceptorSharedData = make(map[string]string)
		}
		ctx.InterceptorSharedData[common.InterceptorDataKeyChallengeTokenIn] = challengeTokenIn
	}
}

// InitiateFlow initiates a new flow with the provided context and returns the flowID without executing the flow.
// This allows external components to pre-initialize a flow with runtime data before actual execution begins.
func (s *flowExecService) InitiateFlow(ctx context.Context,
	initContext *FlowInitContext) (string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowExecService"))

	if initContext == nil || initContext.FlowType == "" {
		return "", &ErrorInvalidFlowInitContext
	}

	// Validate flow type
	flowType, err := validateFlowType(initContext.FlowType)
	if err != nil {
		return "", err
	}

	// Today's callers (authorization, CIBA, sign-out) only ever initiate their own flow types, but the
	// gate is applied here too so this entry point can never become a way around it.
	if svcErr := validateAdministrationCaller(ctx, flowType); svcErr != nil {
		return "", svcErr
	}

	// Application ID is required for all flows except Invite Registration
	if flowType != providers.FlowTypeUserOnboarding && initContext.ApplicationID == "" {
		return "", &ErrorInvalidFlowInitContext
	}

	// Initialize the engine context
	// This uses verbose true to ensure step layouts are returned during execution
	engineCtx, err := s.initContext(ctx, "", initContext.ApplicationID, flowType, nil, true, logger)
	if err != nil {
		logger.Error(ctx, "Failed to initialize flow context",
			log.String("appID", initContext.ApplicationID),
			log.String("flowType", initContext.FlowType),
			log.String("error", err.Error.DefaultValue))
		return "", err
	}

	// Replace the RuntimeData with initContext RuntimeData
	engineCtx.RuntimeData = initContext.RuntimeData
	engineCtx.SetInitiatorRequest(initContext.InitiatorRequest)

	// Store the context without executing the flow
	if storeErr := s.storeContext(ctx, engineCtx, 0, logger); storeErr != nil {
		logger.Error(ctx, "Failed to store initial flow context",
			log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID),
			log.Error(storeErr))
		return "", &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Flow initiated successfully",
		log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID))
	return engineCtx.ExecutionID, nil
}

// InitiateAndExecute initializes a new flow with the provided context, sets runtime data and
// initial user inputs, then executes the flow until a user input is required, an error occurs,
// or the flow completes. Stores the flow context if the flow pauses and returns the FlowStep.
// The caller uses FlowStep.ExecutionID to associate the flow with their session.
//
// InitialInputs are placed directly into UserInputs before execution so executor nodes that
// read from ctx.UserInputs (e.g. the identifying executor) can resolve data without requiring
// an interactive input step from the user.
func (s *flowExecService) InitiateAndExecute(ctx context.Context,
	initContext *FlowInitContext) (*FlowStep, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowExecService"))

	if initContext == nil || initContext.FlowType == "" {
		return nil, &ErrorInvalidFlowInitContext
	}

	flowType, err := validateFlowType(initContext.FlowType)
	if err != nil {
		return nil, err
	}

	// See InitiateFlow: gated here so no initiation entry point bypasses the administration check.
	if svcErr := validateAdministrationCaller(ctx, flowType); svcErr != nil {
		return nil, svcErr
	}

	if flowType != providers.FlowTypeUserOnboarding && initContext.ApplicationID == "" {
		return nil, &ErrorInvalidFlowInitContext
	}

	engineCtx, err := s.initContext(ctx, "", initContext.ApplicationID, flowType, nil, true, logger)
	if err != nil {
		logger.Error(ctx, "Failed to initialize flow context",
			log.String("appID", initContext.ApplicationID),
			log.String("flowType", initContext.FlowType),
			log.String("error", err.Error.DefaultValue))
		return nil, err
	}

	engineCtx.RuntimeData = initContext.RuntimeData
	engineCtx.SetInitiatorRequest(initContext.InitiatorRequest)
	prepareContext(engineCtx, "", initContext.InitialInputs)

	flowStep, flowErr := s.flowEngine.Execute(engineCtx)
	if flowErr != nil {
		return nil, flowErr
	}

	if flowStep.Status == providers.FlowStatusIncomplete {
		if storeErr := s.storeContext(ctx, engineCtx, initContext.ExpirySeconds, logger); storeErr != nil {
			logger.Error(ctx, "Failed to store flow context after execution",
				log.String(log.LoggerKeyExecutionID, engineCtx.ExecutionID),
				log.Error(storeErr))
			return nil, &tidcommon.InternalServerError
		}
	}

	return &flowStep, nil
}

// getFlowContext retrieves the flow context from the store and decrypts it if needed.
func (s *flowExecService) getFlowContext(ctx context.Context, executionID string, logger *log.Logger) (
	*FlowContextDB, *tidcommon.ServiceError) {
	if executionID == "" {
		return nil, &ErrorInvalidExecutionID
	}

	dbModel, err := s.flowStore.GetFlowContext(ctx, executionID)
	if err != nil {
		logger.Error(ctx, "Error retrieving flow context from store",
			log.String(log.LoggerKeyExecutionID, executionID),
			log.String("error", err.Error()))
		return nil, &tidcommon.InternalServerError
	}

	if dbModel == nil {
		return nil, &ErrorInvalidExecutionID
	}

	if isContextEncrypted(dbModel.Context) {
		decrypted, decryptErr := s.cryptoSvc.Decrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM),
			nil, []byte(dbModel.Context))
		if decryptErr != nil {
			logger.Error(ctx, "Failed to decrypt flow context",
				log.String(log.LoggerKeyExecutionID, executionID), log.Error(decryptErr))
			return nil, &tidcommon.InternalServerError
		}
		dbModel.Context = string(decrypted)
	}

	return dbModel, nil
}

// getFlowSectionConfig reads the "flow" section from serverconfig and returns the merged value.
// Returns a zero FlowSectionConfig when the serverconfig service is not wired or the section is
// absent, letting callers fall back to built-in defaults.
func (s *flowExecService) getFlowSectionConfig(ctx context.Context) flowconfig.FlowSectionConfig {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowExecService"))

	if s.serverConfigSvc == nil {
		return flowconfig.FlowSectionConfig{}
	}

	merged, svcErr := s.serverConfigSvc.GetMergedConfig(ctx, "flow")
	if svcErr != nil {
		logger.Error(ctx, "Failed to retrieve merged flow section from serverconfig",
			log.String("error", svcErr.Error.DefaultValue))
		return flowconfig.FlowSectionConfig{}
	}

	cfg, ok := merged.(flowconfig.FlowSectionConfig)
	if !ok {
		logger.Error(ctx, "Unexpected type for merged flow server config; using built-in defaults",
			log.String("type", fmt.Sprintf("%T", merged)))
		return flowconfig.FlowSectionConfig{}
	}

	return cfg
}

// getFlowExpirySeconds returns the context TTL for the given flow type.
func (s *flowExecService) getFlowExpirySeconds(ctx context.Context, flowType providers.FlowType) int64 {
	cfg := s.getFlowSectionConfig(ctx)

	switch flowType {
	case providers.FlowTypeAuthentication:
		return firstPositiveExpiry(cfg.AuthFlow.ExpirySeconds, defaultAuthFlowExpiry)
	case providers.FlowTypeRegistration:
		return firstPositiveExpiry(cfg.RegistrationFlow.ExpirySeconds, defaultRegistrationFlowExpiry)
	case providers.FlowTypeUserOnboarding:
		return firstPositiveExpiry(cfg.UserOnboardingFlow.ExpirySeconds, defaultUserOnboardingFlowExpiry)
	case providers.FlowTypeRecovery:
		return firstPositiveExpiry(cfg.RecoveryFlow.ExpirySeconds, defaultRecoveryFlowExpiry)
	case providers.FlowTypeSignOut:
		return firstPositiveExpiry(cfg.SignOutFlow.ExpirySeconds, defaultSignOutFlowExpiry)
	default:
		return defaultAuthFlowExpiry
	}
}

// resolveDefaultFlowHandle returns the server-level default handle for the given flow type, or ""
// when no default is configured.
func (s *flowExecService) resolveDefaultFlowHandle(ctx context.Context, flowType providers.FlowType) string {
	cfg := s.getFlowSectionConfig(ctx)

	switch flowType {
	case providers.FlowTypeAuthentication:
		return cfg.AuthFlow.DefaultHandle
	case providers.FlowTypeRegistration:
		return cfg.RegistrationFlow.DefaultHandle
	case providers.FlowTypeUserOnboarding:
		return cfg.UserOnboardingFlow.DefaultHandle
	case providers.FlowTypeRecovery:
		return cfg.RecoveryFlow.DefaultHandle
	case providers.FlowTypeSignOut:
		return cfg.SignOutFlow.DefaultHandle
	default:
		return ""
	}
}

// firstPositiveExpiry returns the first positive value between v and fallback, or fallback if v is non-positive.
func firstPositiveExpiry(v, fallback int64) int64 {
	if v > 0 {
		return v
	}
	return fallback
}

// isContextEncrypted reports whether a context string is in encrypted form by checking for an alg field.
func isContextEncrypted(context string) bool {
	var encCheck struct {
		Algorithm string `json:"alg"`
	}
	return json.Unmarshal([]byte(context), &encCheck) == nil && encCheck.Algorithm != ""
}
