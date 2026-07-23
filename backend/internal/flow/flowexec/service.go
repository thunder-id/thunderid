/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package flowexec provides the FlowExecService interface and its implementation.
package flowexec

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/session"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// flowExecService is the implementation of FlowExecServiceInterface
type flowExecService struct {
	flowEngine          flowEngineInterface
	flowProvider        providers.FlowProvider
	graphBuilder        graphbuilder.GraphBuilderInterface
	flowStore           flowStoreInterface
	actorProvider       providers.ActorProvider
	observabilitySvc    providers.ObservabilityProvider
	transactioner       transaction.Transactioner
	cryptoSvc           kmprovider.RuntimeCryptoProvider
	attestationVerifier providers.AttestationProvider
	cfg                 flowconfig.Config
}

// newFlowExecService creates a new instance of flowExecService with the provided dependencies.
func newFlowExecService(flowProvider providers.FlowProvider,
	flowStore flowStoreInterface, flowEngine flowEngineInterface,
	actorProvider providers.ActorProvider,
	observabilitySvc providers.ObservabilityProvider,
	transactioner transaction.Transactioner,
	cryptoSvc kmprovider.RuntimeCryptoProvider,
	attestationVerifier providers.AttestationProvider,
	graphBuilder graphbuilder.GraphBuilderInterface,
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
		cfg:                 cfg,
	}
}

// Execute executes a flow with the given data
func (s *flowExecService) Execute(ctx context.Context,
	appID, executionID, flowType string, verbose bool,
	action string, inputs map[string]string, challengeToken, flowSecret, attestationToken string) (
	*FlowStep, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowExecService"))

	// Get trace ID from context
	traceID := sysContext.GetTraceID(ctx)

	var engineCtx *EngineContext
	var loadErr *tidcommon.ServiceError

	if isNewFlow(executionID) {
		engineCtx, loadErr = s.loadNewContext(ctx, appID, flowType, verbose, action, inputs,
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
		return nil, flowErr
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
func (s *flowExecService) loadNewContext(ctx context.Context, appID, flowTypeStr string, verbose bool,
	action string, inputs map[string]string, flowSecret, attestationToken string, logger *log.Logger) (
	*EngineContext, *tidcommon.ServiceError) {
	flowType, err := validateFlowType(flowTypeStr)
	if err != nil {
		return nil, err
	}

	if svcErr := s.checkDirectFlowInitiationAllowed(
		ctx, appID, flowType, flowSecret, attestationToken, logger); svcErr != nil {
		return nil, svcErr
	}

	engineCtx, err := s.initContext(ctx, appID, flowType, verbose, logger)
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

// resolveFlowInitiationMode derives how the given application is permitted to initiate a new
// authentication flow, using neutral actor data resolved through the actor layer. A non-existent
// application returns ErrorActorNotFound so the caller can distinguish an unknown app from a
// backend app. The resolved OAuth profile (nil for embedded apps) is returned for downstream
// credential checks.
func (s *flowExecService) resolveFlowInitiationMode(
	ctx context.Context, appID string,
) (flowInitiationMode, *providers.AttestationConfig, *tidcommon.ServiceError) {
	// The inbound client is protocol-agnostic and exists for every valid application. Platform
	// attestation is a client-level binary-identity check, so it is resolved here first and takes
	// precedence regardless of whether the application also has an OAuth2 protocol profile. An
	// unknown application surfaces as ErrorActorNotFound so the caller can map it to an invalid app.
	client, clientErr := s.actorProvider.GetInboundClientByID(ctx, appID)
	if clientErr != nil {
		return 0, nil, clientErr
	}
	if client.Attestation != nil && (client.Attestation.Android != nil || client.Attestation.Apple != nil) {
		return flowInitiationAttestation, client.Attestation, nil
	}

	// No attestation configured: classify by protocol profile.
	profile, svcErr := s.actorProvider.GetOAuthProfileByID(ctx, appID)
	if svcErr != nil && svcErr.Code != actorprovider.ErrorActorNotFound.Code {
		return 0, nil, svcErr
	}

	// No protocol profile means a server-side embedded app: it initiates flows directly by
	// presenting its Flow Secret.
	if profile == nil {
		return flowInitiationFlowSecret, nil, nil
	}

	// A redirect-based (authorization_code) profile — public or confidential — must initiate flows
	// through the protocol component, not via a direct HTTP call. A machine-to-machine app
	// (client_credentials as its only grant) obtains tokens directly and does not run flows. Neither
	// may initiate a flow directly.
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
func (s *flowExecService) initContext(ctx context.Context, appID string, flowType providers.FlowType,
	verbose bool, logger *log.Logger) (*EngineContext, *tidcommon.ServiceError) {
	graphID, svcErr := s.getFlowGraph(ctx, appID, flowType, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	engineCtx := EngineContext{}
	executionID, err := sysutils.GenerateUUIDv7()
	if err != nil {
		logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	engineCtx.ExecutionID = executionID

	flow, svcErr := s.flowProvider.GetFlow(ctx, graphID)
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

	handle := s.cfg.Flow.DefaultAuthFlowHandle
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

// getFlowExpirySeconds returns the expiry time for a flow in seconds.
func (s *flowExecService) getFlowExpirySeconds(flowType providers.FlowType) int64 {
	switch flowType {
	case providers.FlowTypeAuthentication:
		return defaultAuthFlowExpiry
	case providers.FlowTypeRegistration:
		return defaultRegistrationFlowExpiry
	case providers.FlowTypeUserOnboarding:
		return defaultUserOnboardingFlowExpiry
	case providers.FlowTypeRecovery:
		return defaultRecoveryFlowExpiry
	default:
		// Fallback to auth flow expiry
		return defaultAuthFlowExpiry
	}
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
	if engineCtx.FlowType == providers.FlowTypeUserOnboarding {
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
		expirySeconds = s.getFlowExpirySeconds(engineCtx.FlowType)
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
	params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}
	ciphertext, _, err := s.cryptoSvc.Encrypt(ctx, nil, params, []byte(serialized.Context))
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
		if !client.IsSignOutFlowEnabled {
			return "", &ErrorSignOutFlowDisabled
		} else if client.SignOutFlowID == "" {
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
		providers.FlowTypeRecovery, providers.FlowTypeSignOut:
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
	handle := ""
	switch flowType {
	case providers.FlowTypeUserOnboarding:
		handle = s.cfg.Flow.UserOnboardingFlowHandle
	default:
		return "", &ErrorInvalidFlowType
	}

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

	// Application ID is required for all flows except Invite Registration
	if flowType != providers.FlowTypeUserOnboarding && initContext.ApplicationID == "" {
		return "", &ErrorInvalidFlowInitContext
	}

	// Initialize the engine context
	// This uses verbose true to ensure step layouts are returned during execution
	engineCtx, err := s.initContext(ctx, initContext.ApplicationID, flowType, true, logger)
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

	if flowType != providers.FlowTypeUserOnboarding && initContext.ApplicationID == "" {
		return nil, &ErrorInvalidFlowInitContext
	}

	engineCtx, err := s.initContext(ctx, initContext.ApplicationID, flowType, true, logger)
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
		decryptParams := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}
		decrypted, decryptErr := s.cryptoSvc.Decrypt(ctx, nil, decryptParams, []byte(dbModel.Context))
		if decryptErr != nil {
			logger.Error(ctx, "Failed to decrypt flow context",
				log.String(log.LoggerKeyExecutionID, executionID), log.Error(decryptErr))
			return nil, &tidcommon.InternalServerError
		}
		dbModel.Context = string(decrypted)
	}

	return dbModel, nil
}

// isContextEncrypted reports whether a context string is in encrypted form by checking for an alg field.
func isContextEncrypted(context string) bool {
	var encCheck struct {
		Algorithm string `json:"alg"`
	}
	return json.Unmarshal([]byte(context), &encCheck) == nil && encCheck.Algorithm != ""
}
