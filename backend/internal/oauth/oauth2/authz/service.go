// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authz implements the OAuth2 authorization functionality.
package authz

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz/requestvalidator"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// runtimeDataTrue is the value the flow layer reads as a set boolean flag on RuntimeData, which
// carries string values only.
const runtimeDataTrue = "true"

// AuthorizeServiceInterface defines the interface for authorization services.
type AuthorizeServiceInterface interface {
	GetAuthorizationCodeDetails(ctx context.Context, clientID string, code string) (*AuthorizationCode, error)
	HandleInitialAuthorizationRequest(
		ctx context.Context, msg *OAuthMessage,
	) (*AuthorizationInitResult, *AuthorizationError)
	HandleAuthorizationCallback(ctx context.Context, authID string, assertion string) (string, *AuthorizationError)
}

// authorizeService implements the AuthorizeService for managing OAuth2 authorization flows.
type authorizeService struct {
	cfg             oauthconfig.Config
	inboundClient   providers.ActorProvider
	resourceService providers.ResourceServerProvider
	authZValidator  AuthorizationValidatorInterface
	authCodeStore   AuthorizationCodeStoreInterface
	authReqStore    authorizationRequestStoreInterface
	parService      par.PARServiceInterface
	jwtService      jwt.JWTServiceInterface
	flowExecService flowexec.FlowExecServiceInterface
	transactioner   providers.Transactioner
	criteriaRevoker revocation.CriteriaRevokerInterface
	// ssoSession resolves an existing SSO session so prompt=none can be answered from it, and
	// flowProvider supplies the client's authentication flow, whose id names the session's cookie
	// and whose active version the session must match. Both are nil in deployments without a
	// session store (the embedded engine), where prompt=none keeps answering login_required.
	ssoSession   flowsession.Service
	flowProvider providers.FlowProvider
	logger       *log.Logger
}

// newAuthorizeService creates a new instance of authorizeService with injected dependencies.
func newAuthorizeService(
	actorProvider providers.ActorProvider,
	resourceService providers.ResourceServerProvider,
	jwtService jwt.JWTServiceInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	authCodeStore AuthorizationCodeStoreInterface,
	authReqStore authorizationRequestStoreInterface,
	parService par.PARServiceInterface,
	transactioner providers.Transactioner,
	criteriaRevoker revocation.CriteriaRevokerInterface,
	ssoSession flowsession.Service,
	flowProvider providers.FlowProvider,
	cfg oauthconfig.Config,
) AuthorizeServiceInterface {
	return &authorizeService{
		cfg:             cfg,
		inboundClient:   actorProvider,
		resourceService: resourceService,
		authZValidator:  newAuthorizationValidator(),
		authCodeStore:   authCodeStore,
		authReqStore:    authReqStore,
		parService:      parService,
		jwtService:      jwtService,
		flowExecService: flowExecService,
		transactioner:   transactioner,
		criteriaRevoker: criteriaRevoker,
		ssoSession:      ssoSession,
		flowProvider:    flowProvider,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizeService")),
	}
}

// checkPromptNone decides whether a prompt=none request can be answered without interaction,
// returning an empty error code when it can (including when prompt=none was not requested).
//
// Per OIDC Core 3.1.2.1 the request succeeds only when the End-User is already authenticated, so
// all three conditions must hold: a live SSO session exists, it belongs to the subject named by
// id_token_hint when one is supplied, and its authentication satisfies max_age. Any failure is
// login_required — the specification's answer for "authentication is needed but cannot be asked
// for".
func (as *authorizeService) checkPromptNone(
	ctx context.Context, oauthParams *oauth2model.OAuthParameters, app *providers.OAuthClient,
) (string, string) {
	if !slices.Contains(strings.Fields(oauthParams.Prompt), oauth2const.PromptNone) {
		return "", ""
	}

	sess := as.resolveSSOSession(ctx, app)
	if sess == nil {
		return oauth2const.ErrorLoginRequired, "User authentication is required"
	}

	if hint := oauthParams.IDTokenHint; hint != "" {
		subject, err := as.subjectFromIDTokenHint(ctx, hint)
		if err != nil {
			return oauth2const.ErrorInvalidRequest, "Invalid id_token_hint"
		}
		if subject != sess.SubjectID {
			// A hint naming someone other than the signed-in subject cannot be satisfied silently.
			return oauth2const.ErrorLoginRequired,
				"The session does not belong to the subject named by id_token_hint"
		}
	}

	if oauthParams.MaxAge != "" {
		maxAge, err := strconv.ParseInt(oauthParams.MaxAge, 10, 64)
		// A malformed max_age is no constraint, matching how the flow's assurance check reads it.
		// max_age=0 admits no elapsed time, so no existing session can satisfy it and the request
		// cannot be answered silently.
		if err == nil && maxAge >= 0 &&
			(maxAge == 0 || time.Now().UTC().Unix()-sess.AuthenticatedAt.Unix() > maxAge) {
			return oauth2const.ErrorLoginRequired,
				"The existing authentication is older than the requested max_age"
		}
	}

	return "", ""
}

// subjectFromIDTokenHint verifies the hint was issued by this server and returns its subject. The
// token's expiry is deliberately not enforced: the hint identifies who was authenticated, and OIDC
// Core requires it to be accepted even once expired.
func (as *authorizeService) subjectFromIDTokenHint(ctx context.Context, idTokenHint string) (string, error) {
	if svcErr := as.jwtService.VerifyJWTSignature(ctx, idTokenHint); svcErr != nil {
		return "", errors.New("id_token_hint signature is not valid")
	}
	payload, err := jwt.DecodeJWTPayload(idTokenHint)
	if err != nil {
		return "", errors.New("id_token_hint could not be decoded")
	}
	if iss, _ := payload[oauth2const.ClaimIss].(string); iss != as.cfg.JWT.Issuer {
		return "", errors.New("id_token_hint was not issued by this server")
	}
	subject, _ := payload[oauth2const.ClaimSub].(string)
	if subject == "" {
		return "", errors.New("id_token_hint carries no subject")
	}
	return subject, nil
}

// resolveSSOSession returns the live SSO session backing this request's client, or nil when there
// is none. The handle is carried on a per-flow cookie, so the client's authentication flow has to
// be resolved before the right cookie can be selected.
func (as *authorizeService) resolveSSOSession(
	ctx context.Context, app *providers.OAuthClient,
) *flowsession.Session {
	if as.ssoSession == nil || as.flowProvider == nil {
		return nil
	}
	inbound, ok := flowsession.InboundFrom(ctx)
	if !ok {
		return nil
	}

	client, svcErr := as.inboundClient.GetInboundClientByID(ctx, app.ID)
	if svcErr != nil || client == nil || client.AuthFlowID == "" {
		return nil
	}
	flow, flowErr := as.flowProvider.GetFlow(ctx, client.AuthFlowID)
	if flowErr != nil || flow == nil {
		return nil
	}

	handle := inbound.HandleFor(client.AuthFlowID)
	sess, err := as.ssoSession.Resolve(ctx, handle, client.AuthFlowID, flow.ActiveVersion, time.Now().UTC())
	if err != nil {
		as.logger.Debug(ctx, "Failed to resolve the SSO session for the authorization request",
			log.Error(err))
		return nil
	}
	return sess
}

// GetAuthorizationCodeDetails retrieves and consumes the authorization code.
func (as *authorizeService) GetAuthorizationCodeDetails(
	ctx context.Context, clientID string, code string,
) (*AuthorizationCode, error) {
	var record *AuthorizationCode
	err := as.transactioner.Transact(ctx, func(ctx context.Context) error {
		var err error
		record, err = as.authCodeStore.GetAuthorizationCode(ctx, code)
		if err != nil {
			return err
		}

		if record.ClientID != clientID {
			return errors.New("client ID mismatch for authorization code")
		}

		consumed, err := as.authCodeStore.ConsumeAuthorizationCode(ctx, code)
		if err != nil {
			return err
		}
		if !consumed {
			// The code was already consumed: this second redemption is a replay (RFC 9700). The token
			// family is revoked on the error path below, via the replay marker.
			return errAuthorizationCodeAlreadyConsumed
		}

		// Record a short-lived replay marker carrying the grant's tfid. The code is removed on consume,
		// so this marker is what lets a later replay of the same code revoke the whole family. Best
		// effort: a failed marker write does not fail the legitimate redemption.
		if record.TokenFamilyID != "" {
			if mErr := as.authCodeStore.MarkConsumedTokenFamily(ctx, code, record.TokenFamilyID,
				time.Until(record.ExpiryTime)); mErr != nil {
				as.logger.Error(ctx, "Failed to record consumed authorization code replay marker",
					log.Error(mErr))
			}
		}
		return nil
	})
	if err != nil {
		// A failed redemption of an already-consumed code is a replay: if a marker exists for the code,
		// revoke the family issued from the first redemption. It is a no-op when no marker exists (e.g. a
		// genuinely unknown code or a client-id mismatch on a still-unconsumed code).
		as.revokeTokenFamilyOnCodeReplay(ctx, code)
		as.logger.Error(ctx, "Failed to get authorization code details", log.Error(err))
		return nil, err
	}
	return record, nil
}

// revokeTokenFamilyOnCodeReplay revokes the token family of a replayed authorization code, when
// enabled. The code itself is removed on consume, so it looks up the replay marker written at first
// redemption to recover the tfid and drop the whole family. It is best-effort: a missing marker or a
// failed revoke is logged and does not change the replay rejection.
func (as *authorizeService) revokeTokenFamilyOnCodeReplay(ctx context.Context, code string) {
	if as.criteriaRevoker == nil || !as.cfg.OAuth.Revocation.TokenFamily.OnCodeReplayEnabled() {
		return
	}
	tokenFamilyID, found, err := as.authCodeStore.ConsumedTokenFamily(ctx, code)
	if err != nil {
		as.logger.Error(ctx, "Failed to look up consumed authorization code replay marker", log.Error(err))
		return
	}
	if !found || tokenFamilyID == "" {
		return
	}
	if err := as.criteriaRevoker.RevokeTokenFamily(ctx, tokenFamilyID,
		revocation.RevocationReasonCodeReplay); err != nil {
		as.logger.Error(ctx, "Failed to revoke token family on authorization code replay", log.Error(err))
	}
}

// HandleInitialAuthorizationRequest processes an initial authorization request from the client.
// Returns the query params needed to redirect to the login page, or a structured authorization error.
func (as *authorizeService) HandleInitialAuthorizationRequest(ctx context.Context, msg *OAuthMessage) (
	*AuthorizationInitResult, *AuthorizationError) {
	queryParams := url.Values(msg.RequestQueryParams)
	clientID := queryParams.Get(oauth2const.RequestParamClientID)
	requestURI := queryParams.Get(oauth2const.RequestParamRequestURI)

	if clientID == "" {
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Missing client_id parameter",
		}
	}

	// Retrieve the OAuth client based on the client ID.
	app, lookupErr := as.inboundClient.GetOAuthClientByClientID(ctx, clientID)
	if lookupErr != nil {
		as.logger.Error(ctx, "Failed to retrieve OAuth client",
			log.String("error", lookupErr.Error.DefaultValue))
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process authorization request",
		}
	}
	if app == nil {
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Invalid client_id",
		}
	}

	// If request_uri is present, resolve the pushed authorization request. A request_uri that is
	// not a PAR handle is a client-supplied request object by reference (RFC 9101), which is not
	// supported: reject it rather than ignoring it, so the client learns its request was not
	// honored (OIDC Core 6.1).
	if requestURI != "" {
		if !par.IsPARRequestURI(requestURI) {
			return nil, as.newRequestObjectError(ctx, msg, app,
				oauth2const.ErrorRequestURINotSupported, "The request_uri parameter is not supported")
		}
		return as.handlePARAuthorizationRequest(ctx, requestURI, clientID, app)
	}

	// Enforce PAR requirement: if PAR is required (per-client or global), reject requests without request_uri.
	if app.RequiresPAR() {
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Pushed authorization request is required for this client",
		}
	}

	initiatorReq := &providers.InitiatorRequest{
		Headers:     utils.FilterSensitiveHeaders(msg.RequestHeaders),
		QueryParams: msg.RequestQueryParams,
	}

	return as.handleStandardAuthorizationRequest(ctx, msg, app, initiatorReq)
}

// newRequestObjectError builds the rejection for an unsupported request object. The error is
// returned to the client's redirect_uri only when that redirect_uri validates against the
// client's registration, since an unvalidated redirect_uri must not be used as a redirect
// target; otherwise it is shown on the server error page.
func (as *authorizeService) newRequestObjectError(
	ctx context.Context, msg *OAuthMessage, app *providers.OAuthClient, code string, message string,
) *AuthorizationError {
	queryParams := url.Values(msg.RequestQueryParams)
	redirectURI := queryParams.Get(oauth2const.RequestParamRedirectURI)

	authErr := &AuthorizationError{
		Code:    code,
		Message: message,
		State:   queryParams.Get(oauth2const.RequestParamState),
	}
	if err := app.ValidateRedirectURI(ctx, redirectURI); err != nil {
		as.logger.Debug(ctx, "Validation failed for redirect URI", log.Error(err))
		return authErr
	}
	// ValidateRedirectURI only accepts an omitted redirect_uri when the client has exactly one
	// fully qualified URI registered; fall back to that so the rejection still reaches the client.
	if redirectURI == "" {
		redirectURI = app.RedirectURIs[0]
	}
	authErr.SendErrorToClient = true
	authErr.ClientRedirectURI = redirectURI
	return authErr
}

// handlePARAuthorizationRequest resolves a request_uri from a PAR and continues the authorization flow.
func (as *authorizeService) handlePARAuthorizationRequest(
	ctx context.Context, requestURI string, clientID string,
	app *providers.OAuthClient) (
	*AuthorizationInitResult, *AuthorizationError) {
	oauthParams, initiatorReq, err := as.parService.ResolvePushedAuthorizationRequest(ctx, requestURI, clientID)
	if err != nil {
		as.logger.Debug(ctx, "Failed to resolve PAR request", log.Error(err))
		if errors.Is(err, par.ErrPARResolutionFailed) {
			return nil, &AuthorizationError{
				Code:    oauth2const.ErrorServerError,
				Message: "Failed to process authorization request",
			}
		}
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Invalid, expired, or already used request_uri",
		}
	}

	return as.initiateFlowAndStoreRequest(ctx, oauthParams, app, initiatorReq)
}

// handleStandardAuthorizationRequest processes a standard authorization request (without PAR).
func (as *authorizeService) handleStandardAuthorizationRequest(
	ctx context.Context, msg *OAuthMessage, app *providers.OAuthClient,
	initiatorReq *providers.InitiatorRequest,
) (*AuthorizationInitResult, *AuthorizationError) {
	queryParams := url.Values(msg.RequestQueryParams)

	// Extract required parameters.
	redirectURI := queryParams.Get(oauth2const.RequestParamRedirectURI)
	scope := queryParams.Get(oauth2const.RequestParamScope)
	state := queryParams.Get(oauth2const.RequestParamState)
	responseType := queryParams.Get(oauth2const.RequestParamResponseType)

	// Extract PKCE parameters.
	codeChallenge := queryParams.Get(oauth2const.RequestParamCodeChallenge)
	codeChallengeMethod := queryParams.Get(oauth2const.RequestParamCodeChallengeMethod)

	resources := msg.Resources
	// A token is bound to exactly one resource server, so an authorization request may target at
	// most one resource (RFC 8707 allows repetition, but multi-resource codes are not supported).
	if len(resources) > 1 {
		return nil, &AuthorizationError{
			Code:    oauth2const.ErrorInvalidTarget,
			Message: "Only a single resource parameter is supported",
		}
	}

	// Extract claims parameter.
	claimsParam := queryParams.Get(oauth2const.RequestParamClaims)

	// Extract claims_locales parameter.
	claimsLocales := queryParams.Get(oauth2const.RequestParamClaimsLocales)

	// Extract ui_locales parameter.
	uiLocales := queryParams.Get(oauth2const.RequestParamUILocales)

	nonce := queryParams.Get(oauth2const.RequestParamNonce)
	acrValues := queryParams.Get(oauth2const.RequestParamAcrValues)
	maxAge := queryParams.Get(oauth2const.RequestParamMaxAge)
	dpopJkt := queryParams.Get(oauth2const.RequestParamDPoPJkt)
	prompt := queryParams.Get(oauth2const.RequestParamPrompt)

	// Parse the claims parameter if present.
	var claimsRequest *oauth2model.ClaimsRequest
	if claimsParam != "" {
		var err error
		claimsRequest, err = oauth2utils.ParseClaimsRequest(claimsParam)
		if err != nil {
			as.logger.Debug(ctx, "Failed to parse claims parameter", log.Error(err))
			return nil, &AuthorizationError{
				Code:    oauth2const.ErrorInvalidRequest,
				Message: "The claims request parameter is malformed or contains invalid values",
			}
		}
	}

	// Validate the authorization request.
	sendErrorToApp, errorCode, errorMessage := as.authZValidator.validateInitialAuthorizationRequest(ctx, msg, app)
	if errorCode != "" {
		authErr := &AuthorizationError{
			Code:    errorCode,
			Message: errorMessage,
			State:   state,
		}
		if sendErrorToApp && redirectURI != "" {
			authErr.SendErrorToClient = true
			authErr.ClientRedirectURI = redirectURI
		}
		return nil, authErr
	}

	oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(scope, app.ScopeClaims)

	// The single target resource server, downscoping, and audience binding are resolved in
	// initiateFlowAndStoreRequest, the path shared by both standard and PAR-based requests.

	// Construct authorization request context.
	oauthParams := &oauth2model.OAuthParameters{
		State:               state,
		ClientID:            app.ClientID,
		RedirectURI:         redirectURI,
		RedirectURIProvided: redirectURI != "",
		ResponseType:        responseType,
		StandardScopes:      oidcScopes,
		PermissionScopes:    nonOidcScopes,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Resources:           resources,
		ClaimsRequest:       claimsRequest,
		ClaimsLocales:       claimsLocales,
		UILocales:           uiLocales,
		Nonce:               nonce,
		AcrValues:           acrValues,
		MaxAge:              maxAge,
		DPoPJkt:             dpopJkt,
		Prompt:              prompt,
		IDTokenHint:         queryParams.Get(oauth2const.RequestParamIDTokenHint),
	}

	// Set the redirect URI if not provided in the request. Invalid cases are already handled at this point.
	// TODO: This should be removed when supporting other means of authorization.
	if redirectURI == "" {
		if len(app.RedirectURIs) == 0 {
			as.logger.Error(ctx, "OAuth application has no registered redirect URIs",
				log.String("client_id", app.ClientID))
			return nil, &AuthorizationError{
				Code:    oauth2const.ErrorServerError,
				Message: "Failed to process authorization request",
			}
		}
		oauthParams.RedirectURI = app.RedirectURIs[0]
	}

	return as.initiateFlowAndStoreRequest(ctx, oauthParams, app, initiatorReq)
}

// initiateFlowAndStoreRequest initiates the authentication flow and stores the authorization request context.
// This is the common path shared by both standard and PAR-based authorization requests.
func (as *authorizeService) initiateFlowAndStoreRequest(
	ctx context.Context, oauthParams *oauth2model.OAuthParameters,
	app *providers.OAuthClient, initiatorReq *providers.InitiatorRequest,
) (*AuthorizationInitResult, *AuthorizationError) {
	// prompt=none forbids any user interaction, so it can only be honored by an existing session
	// that also satisfies id_token_hint and max_age. This is checked here, where both the standard
	// and PAR paths converge, and before the flow starts: the flow would otherwise prompt.
	if errCode, errMsg := as.checkPromptNone(ctx, oauthParams, app); errCode != "" {
		return nil, &AuthorizationError{
			Code:              errCode,
			Message:           errMsg,
			SendErrorToClient: oauthParams.RedirectURI != "",
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}

	// Bind the request to a single target resource server before the flow starts. OIDC-only or
	// scopeless requests stay unbound and their audience is the client_id. A permission-bearing
	// request resolves an explicit resource or the configured default, rejecting with invalid_target
	// when neither is available. The resolved resource server id is threaded into the flow so the
	// authorization executor scopes its permission evaluation to it.
	targetRS, errResp := resourceindicators.ResolveAudienceBinding(
		ctx, as.resourceService, oauthParams.Resources, oauthParams.PermissionScopes)
	if errResp != nil {
		return nil, &AuthorizationError{
			Code:              errResp.Error,
			Message:           errResp.ErrorDescription,
			SendErrorToClient: oauthParams.RedirectURI != "",
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}
	resourceServerIdentifier := ""
	if targetRS != nil {
		downscoped, dErr := resourceindicators.DownscopeToResourceServer(
			ctx, as.resourceService, targetRS.ID, oauthParams.PermissionScopes)
		if dErr != nil {
			return nil, &AuthorizationError{
				Code:              dErr.Error,
				Message:           dErr.ErrorDescription,
				SendErrorToClient: oauthParams.RedirectURI != "",
				ClientRedirectURI: oauthParams.RedirectURI,
				State:             oauthParams.State,
			}
		}
		oauthParams.PermissionScopes = downscoped
		oauthParams.Resources = []string{targetRS.Identifier}
		resourceServerIdentifier = targetRS.Identifier
	}

	effectiveAcrValues := requestvalidator.ResolveACRValues(oauthParams.AcrValues, app.AcrValues)
	essentialAttributes, optionalAttributes := getRequiredAttributes(
		oauthParams.StandardScopes, oauthParams.ClaimsRequest, app)

	authRequestCtx := authRequestContext{
		OAuthParameters: *oauthParams,
	}

	// Store authorization request context in the store.
	identifier, storeErr := as.authReqStore.AddRequest(ctx, authRequestCtx)
	if storeErr != nil {
		as.logger.Error(ctx, "Failed to store authorization request context", log.Error(storeErr))
		return nil, &AuthorizationError{
			Code:              oauth2const.ErrorServerError,
			Message:           "Failed to process authorization request",
			SendErrorToClient: true,
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}

	// Initiate flow with OAuth context.
	runtimeData := map[string]string{
		flowcm.RuntimeKeyClientID:                      oauthParams.ClientID,
		flowcm.RuntimeKeyRequestedPermissions:          utils.StringifyStringArray(oauthParams.PermissionScopes, " "),
		flowcm.RuntimeKeyResourceServerIdentifier:      resourceServerIdentifier,
		flowcm.RuntimeKeyRequiredEssentialAttributes:   essentialAttributes,
		flowcm.RuntimeKeyRequiredOptionalAttributes:    optionalAttributes,
		flowcm.RuntimeKeyRequiredLocales:               oauthParams.ClaimsLocales,
		flowcm.RuntimeKeyUserAttributesCacheTTLSeconds: fmt.Sprintf("%d", as.resolveUserAttributesCacheTTL(app)),
		flowcm.RuntimeKeyAuthorizationRequestID:        identifier,
	}
	if effectiveAcrValues != "" {
		runtimeData[flowcm.RuntimeKeyRequestedAuthClasses] = effectiveAcrValues
	}
	if slices.Contains(strings.Fields(oauthParams.Prompt), oauth2const.PromptConsent) {
		runtimeData[flowcm.RuntimeKeyForceConsentReprompt] = runtimeDataTrue
	}
	// prompt=login requires a fresh authentication regardless of any existing session (OIDC Core
	// 3.1.2.1), so tell the SSO-Check node not to reuse one.
	if slices.Contains(strings.Fields(oauthParams.Prompt), oauth2const.PromptLogin) {
		runtimeData[flowcm.RuntimeKeyForceReauth] = runtimeDataTrue
	}
	// prompt=none reached this point only because checkPromptNone found a session that satisfies the
	// request, so tell the SSO-Check node to honor that decision instead of re-evaluating max_age
	// against its own clock. Both comparisons are strictly greater against a fresh time.Now(), so a
	// session exactly on the boundary passes here and could fail there, prompting a request that
	// forbids prompting.
	if slices.Contains(strings.Fields(oauthParams.Prompt), oauth2const.PromptNone) {
		runtimeData[flowcm.RuntimeKeySilentAuthOnly] = runtimeDataTrue
	}
	if oauthParams.MaxAge != "" {
		runtimeData[flowcm.RuntimeKeyMaxAge] = oauthParams.MaxAge
	}
	flowInitCtx := &flowexec.FlowInitContext{
		ApplicationID:    app.ID,
		FlowType:         string(providers.FlowTypeAuthentication),
		RuntimeData:      runtimeData,
		InitiatorRequest: initiatorReq,
	}

	executionID, flowErr := as.flowExecService.InitiateFlow(ctx, flowInitCtx)
	if flowErr != nil {
		as.logger.Error(ctx, "Failed to initiate authentication flow",
			log.String("error_code", flowErr.Code))
		return nil, &AuthorizationError{
			Code:              oauth2const.ErrorServerError,
			Message:           "Failed to process authorization request",
			SendErrorToClient: true,
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}

	// Build query parameters for login page redirect.
	queryParams := make(map[string]string)
	queryParams[oauth2const.AuthID] = identifier
	queryParams[oauth2const.AppID] = app.ID
	queryParams[oauth2const.ExecutionID] = executionID
	if oauthParams.UILocales != "" {
		queryParams[oauth2const.RequestParamUILocales] = oauthParams.UILocales
	}

	// Add insecure warning if the redirect URI is not using TLS.
	// TODO: May require another redirection to a warn consent page when it directly goes to a federated IDP.
	parsedRedirectURI, err := utils.ParseURL(oauthParams.RedirectURI)
	if err != nil {
		as.logger.Error(ctx, "Failed to parse redirect URI", log.Error(err))
		return nil, &AuthorizationError{
			Code:              oauth2const.ErrorServerError,
			Message:           "Failed to process authorization request",
			SendErrorToClient: true,
			ClientRedirectURI: oauthParams.RedirectURI,
			State:             oauthParams.State,
		}
	}
	if parsedRedirectURI.Scheme == "http" {
		queryParams[oauth2const.ShowInsecureWarning] = "true"
	}

	return &AuthorizationInitResult{QueryParams: queryParams}, nil
}

// HandleAuthorizationCallback processes the callback assertion from the flow engine. The assertion is
// either an authentication assertion from a completed flow or a signed error assertion minted when the
// flow terminated in failure. Returns the client redirect URI (with authorization code) on success, or a structured
// error.
func (as *authorizeService) HandleAuthorizationCallback(ctx context.Context, authID string, assertion string) (
	string, *AuthorizationError) {
	if assertion == "" {
		return "", &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Invalid authorization request",
		}
	}

	// Verify before either branch runs. This keeps an unverified assertion from burning a live authID,
	// and it means the branch below is selected by a claim that the signature covers.
	if verifyErr := as.verifyAssertion(ctx, assertion); verifyErr != nil {
		as.logger.Debug(ctx, "Assertion verification failed", log.Error(verifyErr))
		return "", &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Authorization request failed",
		}
	}

	claims, authTime, decodeErr := decodeAttributesFromAssertion(assertion)
	if decodeErr != nil {
		// An assertion whose claims cannot be read cannot be shown to be bound to this request, so it is
		// rejected without loading it. Loading consumes the request, and a caller holding a malformed
		// assertion must not be able to destroy a live authID
		as.logger.Debug(ctx, "Failed to decode the assertion", log.Error(decodeErr))
		return "", &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Authorization request failed",
		}
	}

	if claims.flowErrorType != "" {
		errClaims, _ := oauth2utils.DecodeFlowErrorAssertionClaims(assertion)
		return "", as.handleFailedCallback(ctx, authID, errClaims)
	}

	return as.handleSuccessCallback(ctx, authID, claims, authTime)
}

// handleSuccessCallback mints an authorization code for a verified authentication assertion and
// returns the client redirect URI carrying it.
func (as *authorizeService) handleSuccessCallback(ctx context.Context, authID string,
	claims assertionClaims, authTime time.Time) (string, *AuthorizationError) {
	var redirectURI string
	var authErr *AuthorizationError

	err := func() error {
		// Load the authorization request context.
		authRequestCtx, err := as.loadAuthRequestContext(ctx, authID)
		if err != nil {
			if errors.Is(err, errAuthRequestNotFound) {
				authErr = &AuthorizationError{
					Code:    oauth2const.ErrorInvalidRequest,
					Message: "Invalid authorization request",
				}
				return err
			}

			authErr = &AuthorizationError{
				Code:    oauth2const.ErrorServerError,
				Message: "Failed to process authorization request",
			}
			return err
		}

		// Bind the assertion to the specific authorization request
		if claims.authorizationRequestID == "" || claims.authorizationRequestID != authID {
			as.logger.Debug(ctx, "Assertion is not bound to the authorization request")
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorAccessDenied,
				Message:           "Assertion does not match the authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return errors.New("assertion not bound to authorization request")
		}

		if claims.userID == "" {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Authorization request failed",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return errors.New("user ID is empty")
		}

		// Validate sub claim constraint if specified in claims parameter.
		// If sub claim is requested with a value constraint and doesn't match, authentication must fail.
		hasOpenIDScope := slices.Contains(authRequestCtx.OAuthParameters.StandardScopes, oauth2const.ScopeOpenID)
		if hasOpenIDScope {
			if err := validateSubClaimConstraint(
				authRequestCtx.OAuthParameters.ClaimsRequest, claims.userID,
			); err != nil {
				as.logger.Debug(ctx, "Sub claim validation failed", log.Error(err))
				authErr = &AuthorizationError{
					Code:              oauth2const.ErrorAccessDenied,
					Message:           "Authorization request failed",
					SendErrorToClient: true,
					ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
					State:             authRequestCtx.OAuthParameters.State,
				}
				return err
			}
		}

		// Extract authorized permissions for permission scopes.
		// Overwrite the non-OIDC scopes in auth request context with the authorized scopes from the assertion.
		if claims.authorizedPermissions != "" {
			authRequestCtx.OAuthParameters.PermissionScopes = utils.ParseStringArray(
				claims.authorizedPermissions, " ")
		} else {
			// Clear permission scopes if no authorized permissions in assertion.
			authRequestCtx.OAuthParameters.PermissionScopes = []string{}
		}

		// Generate the authorization code.
		authzCode, err := createAuthorizationCode(as.cfg, authRequestCtx, &claims, authTime)
		if err != nil {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Failed to process authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return err
		}

		// Persist the authorization code.
		if persistErr := as.authCodeStore.InsertAuthorizationCode(ctx, authzCode); persistErr != nil {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Failed to process authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return persistErr
		}

		// Construct the redirect URI with the authorization code.
		queryParams := map[string]string{
			"code":                      authzCode.Code,
			oauth2const.RequestParamIss: as.cfg.JWT.Issuer,
		}
		if authRequestCtx.OAuthParameters.State != "" {
			queryParams[oauth2const.RequestParamState] = authRequestCtx.OAuthParameters.State
		}
		redirectURI, err = oauth2utils.GetURIWithQueryParams(authzCode.RedirectURI, queryParams)
		if err != nil {
			authErr = &AuthorizationError{
				Code:              oauth2const.ErrorServerError,
				Message:           "Failed to process authorization request",
				SendErrorToClient: true,
				ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
				State:             authRequestCtx.OAuthParameters.State,
			}
			return err
		}

		return nil
	}()

	if authErr != nil {
		if authErr.Code == oauth2const.ErrorServerError {
			as.logger.Error(ctx, "Failed to process authorization callback", log.Error(err))
		}
		return "", authErr
	}
	if err != nil {
		as.logger.Error(ctx, "Failed to process authorization callback", log.Error(err))
		return "", &AuthorizationError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process authorization request",
		}
	}

	return redirectURI, nil
}

// handleFailedCallback constructs the OAuth error response for a verified error assertion. The
// assertion is bound to this authorization request before the request context is loaded, since
// loading consumes it and an assertion minted for another request must not burn a live authID.
func (as *authorizeService) handleFailedCallback(
	ctx context.Context, authID string, claims oauth2utils.FlowErrorAssertionClaims) *AuthorizationError {
	if claims.AuthorizationRequestID != authID {
		as.logger.Debug(ctx, "Error assertion is not bound to the authorization request")
		return &AuthorizationError{
			Code:    oauth2const.ErrorInvalidRequest,
			Message: "Error assertion does not match the authorization request",
		}
	}

	authRequestCtx, err := as.loadAuthRequestContext(ctx, authID)
	if err != nil {
		if errors.Is(err, errAuthRequestNotFound) {
			return &AuthorizationError{
				Code:    oauth2const.ErrorInvalidRequest,
				Message: "Invalid authorization request",
			}
		}
		as.logger.Error(ctx, "Failed to load authorization request context", log.Error(err))
		return &AuthorizationError{
			Code:    oauth2const.ErrorServerError,
			Message: "Failed to process authorization request",
		}
	}

	code, message := mapFlowErrorTypeToOAuthError(claims.ErrorType, claims.Description)
	// Denials are always reported. Server errors are reported unless the deployment opts out, in
	// which case the failure surfaces on the error page and the client is left to time out.
	sendToClient := code != oauth2const.ErrorServerError || as.cfg.OAuth.SendServerErrorsToClientEnabled()
	as.logger.Debug(ctx, "Propagating flow failure",
		log.String("flowErrorType", claims.ErrorType),
		log.String("flowErrorDescription", claims.Description),
		log.Bool("sendToClient", sendToClient))
	return &AuthorizationError{
		Code:              code,
		Message:           message,
		SendErrorToClient: sendToClient,
		ClientRedirectURI: authRequestCtx.OAuthParameters.RedirectURI,
		State:             authRequestCtx.OAuthParameters.State,
	}
}

// loadAuthRequestContext loads the authorization request context from the store using the auth ID.
func (as *authorizeService) loadAuthRequestContext(ctx context.Context, authID string) (*authRequestContext, error) {
	ok, authRequestCtx, err := as.authReqStore.GetRequest(ctx, authID)
	if err != nil {
		as.logger.Error(ctx, "Failed to retrieve authorization request context", log.Error(err))
		return nil, errors.New("failed to retrieve authorization request context")
	}
	if !ok {
		as.logger.Debug(ctx, "Authorization request context not found", log.String("auth_id", authID))
		return nil, errAuthRequestNotFound
	}

	// Remove the authorization request context after retrieval.
	if clearErr := as.authReqStore.ClearRequest(ctx, authID); clearErr != nil {
		as.logger.Error(ctx, "Failed to clear authorization request context", log.Error(clearErr))
	}
	return &authRequestCtx, nil
}

// verifyAssertion verifies the JWT assertion.
func (as *authorizeService) verifyAssertion(ctx context.Context, assertion string) error {
	if err := as.jwtService.VerifyJWT(ctx, assertion, "", ""); err != nil {
		as.logger.Debug(ctx, "Invalid assertion signature", log.String("error", err.Error.DefaultValue))
		return errors.New("invalid assertion signature")
	}
	return nil
}

// decodeAttributesFromAssertion decodes user attributes from the flow assertion JWT using the
// shared base decoder. authorized_permissions is auth-code-specific and extracted separately
// from the raw payload returned alongside the common claims.
func decodeAttributesFromAssertion(assertion string) (assertionClaims, time.Time, error) {
	base, payload, err := oauth2utils.DecodeFlowAssertionClaims(assertion)
	if err != nil {
		return assertionClaims{}, time.Time{}, err
	}

	claims := assertionClaims{
		userID:           base.UserID,
		attributeCacheID: base.AttributeCacheID,
		completedACR:     base.CompletedACR,
	}

	if v, ok := payload[oauth2const.ClaimAuthorizedPermissions].(string); ok {
		claims.authorizedPermissions = v
	}

	if v, ok := payload[oauth2const.ClaimTokenFamilyID].(string); ok {
		claims.tokenFamilyID = v
	}

	if v, ok := payload[flowcm.ClaimFlowErrorType].(string); ok {
		claims.flowErrorType = v
	}

	if v, ok := payload[oauth2const.ClaimAuthorizationRequestID]; ok {
		strValue, ok := v.(string)
		if !ok {
			return assertionClaims{}, time.Time{}, fmt.Errorf(
				"%w: 'authorization_request_id' claim is not a string", errAssertionClaimInvalid)
		}
		claims.authorizationRequestID = strValue
	}

	return claims, base.AuthTime, nil
}

// createAuthorizationCode generates an authorization code based on the provided
// authorization request context and authenticated user.
func createAuthorizationCode(
	cfg oauthconfig.Config,
	authRequestCtx *authRequestContext,
	claims *assertionClaims,
	authTime time.Time,
) (AuthorizationCode, error) {
	clientID := authRequestCtx.OAuthParameters.ClientID
	redirectURI := authRequestCtx.OAuthParameters.RedirectURI

	if clientID == "" || redirectURI == "" {
		return AuthorizationCode{}, errors.New("client_id or redirect_uri is missing")
	}

	if claims.userID == "" {
		return AuthorizationCode{}, errors.New("authenticated user not found")
	}

	// The code's own lifetime runs from now, not from authTime. On the SSO path authTime is the
	// reused session's authentication, which can be hours old; measuring expiry from it would mint
	// codes that are already expired and fail insertion outright.
	now := time.Now()

	// Use provided authTime, or fallback to current time if zero (iat claim was not available).
	if authTime.IsZero() {
		authTime = now
	}

	standardScopes := authRequestCtx.OAuthParameters.StandardScopes
	permissionScopes := authRequestCtx.OAuthParameters.PermissionScopes
	allScopes := append(append([]string{}, standardScopes...), permissionScopes...)
	resources := authRequestCtx.OAuthParameters.Resources

	validityPeriod := cfg.OAuth.AuthorizationCode.ValidityPeriod
	expiryTime := now.Add(time.Duration(validityPeriod) * time.Second)

	codeID, err := utils.GenerateUUIDv7()
	if err != nil {
		return AuthorizationCode{}, errors.New("failed to generate UUID")
	}

	// Fall back to minting the token family id here when the login flow did not (a flow without an SSO
	// SessionExecutor node never mints one). Every authorization code then anchors a revocable family,
	// so grant-scoped revocation works regardless of whether SSO was enabled. SSO flows already carry
	// the tfid minted at the session node, so this preserves their session-participant linkage.
	tokenFamilyID := claims.tokenFamilyID
	if tokenFamilyID == "" {
		tokenFamilyID, err = utils.GenerateUUIDv7()
		if err != nil {
			return AuthorizationCode{}, errors.New("failed to generate token family id")
		}
	}

	code, err := oauth2utils.GenerateAuthorizationCode()
	if err != nil {
		return AuthorizationCode{}, errors.New("failed to generate authorization code")
	}

	return AuthorizationCode{
		CodeID:              codeID,
		Code:                code,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		RedirectURIProvided: authRequestCtx.OAuthParameters.RedirectURIProvided,
		AuthorizedUserID:    claims.userID,
		AttributeCacheID:    claims.attributeCacheID,
		TimeCreated:         now,
		AuthTime:            authTime,
		ExpiryTime:          expiryTime,
		Scopes:              utils.StringifyStringArray(allScopes, " "),
		State:               AuthCodeStateActive,
		CodeChallenge:       authRequestCtx.OAuthParameters.CodeChallenge,
		CodeChallengeMethod: authRequestCtx.OAuthParameters.CodeChallengeMethod,
		Resources:           resources,
		ClaimsRequest:       authRequestCtx.OAuthParameters.ClaimsRequest,
		ClaimsLocales:       authRequestCtx.OAuthParameters.ClaimsLocales,
		Nonce:               authRequestCtx.OAuthParameters.Nonce,
		CompletedACR:        claims.completedACR,
		DPoPJkt:             authRequestCtx.OAuthParameters.DPoPJkt,
		TokenFamilyID:       tokenFamilyID,
	}, nil
}

// getRequiredAttributes determines the essential and optional user attributes required based on OIDC scopes,
// claims parameter, and app configuration.
func getRequiredAttributes(oidcScopes []string, claimsRequest *oauth2model.ClaimsRequest,
	app *providers.OAuthClient) (essentialAttributes, optionalAttributes string) {
	if app == nil {
		return "", ""
	}

	essentialAttributesMap := make(map[string]bool)
	optionalAttributesMap := make(map[string]bool)

	// Add access token attributes from app config
	if app.Token != nil {
		appendAccessTokenAttributes(app, optionalAttributesMap)
	}

	// Process OIDC-related attributes only if openid scope is present
	if slices.Contains(oidcScopes, oauth2const.ScopeOpenID) {
		appendOIDCAttributes(oidcScopes, claimsRequest, app,
			essentialAttributesMap, optionalAttributesMap)
	}

	// Remove any duplicates between essential and optional attributes, giving precedence to essential
	if len(essentialAttributesMap) > 0 && len(optionalAttributesMap) > 0 {
		for attr := range essentialAttributesMap {
			if optionalAttributesMap[attr] {
				delete(optionalAttributesMap, attr)
			}
		}
	}

	// Convert attribute maps to space-separated strings
	essentialAttributes = strings.Join(slices.Collect(maps.Keys(essentialAttributesMap)), " ")
	optionalAttributes = strings.Join(slices.Collect(maps.Keys(optionalAttributesMap)), " ")

	return essentialAttributes, optionalAttributes
}

// appendAccessTokenAttributes appends access token attributes from app configuration.
func appendAccessTokenAttributes(app *providers.OAuthClient, attributesMap map[string]bool) {
	if app.Token.AccessToken == nil || app.Token.AccessToken.UserConfig == nil {
		return
	}
	for _, attr := range app.Token.AccessToken.UserConfig.Attributes {
		attributesMap[attr] = true
	}
}

// appendOIDCAttributes appends OIDC-related attributes from scopes and claims parameters.
func appendOIDCAttributes(oidcScopes []string, claimsRequest *oauth2model.ClaimsRequest,
	app *providers.OAuthClient, essentialAttributes, optionalAttributes map[string]bool) {
	var idTokenAllowedSet map[string]bool
	if app.Token != nil {
		idTokenAllowedSet = buildIDTokenAllowedSet(app.Token.IDToken)
	}
	userInfoAllowedSet := buildUserInfoAllowedSet(app.UserInfo)

	appendAttributesFromClaimsParameter(claimsRequest, idTokenAllowedSet, userInfoAllowedSet,
		essentialAttributes, optionalAttributes)

	appendAttributesFromScopes(oidcScopes, app, idTokenAllowedSet, userInfoAllowedSet,
		optionalAttributes)
}

// buildIDTokenAllowedSet creates a set of allowed attributes for ID token.
func buildIDTokenAllowedSet(idTokenConfig *providers.IDTokenConfig) map[string]bool {
	if idTokenConfig == nil || len(idTokenConfig.UserAttributes) == 0 {
		return nil
	}
	allowedSet := make(map[string]bool, len(idTokenConfig.UserAttributes))
	for _, attr := range idTokenConfig.UserAttributes {
		allowedSet[attr] = true
	}
	return allowedSet
}

// buildUserInfoAllowedSet creates a set of allowed attributes for UserInfo.
func buildUserInfoAllowedSet(userInfoConfig *providers.UserInfoConfig) map[string]bool {
	if userInfoConfig == nil || len(userInfoConfig.UserAttributes) == 0 {
		return nil
	}
	allowedSet := make(map[string]bool, len(userInfoConfig.UserAttributes))
	for _, attr := range userInfoConfig.UserAttributes {
		allowedSet[attr] = true
	}
	return allowedSet
}

// appendAttributesFromClaimsParameter appends user attributes requested via the claims parameter.
func appendAttributesFromClaimsParameter(claimsRequest *oauth2model.ClaimsRequest,
	idTokenAllowedSet, userInfoAllowedSet, essentialAttributes, optionalAttributes map[string]bool) {
	if claimsRequest == nil {
		return
	}

	// Append id token attributes
	if claimsRequest.IDToken != nil && idTokenAllowedSet != nil {
		for name, value := range claimsRequest.IDToken {
			if idTokenAllowedSet[name] {
				if value != nil && value.Essential {
					essentialAttributes[name] = true
				} else {
					optionalAttributes[name] = true
				}
			}
		}
	}

	// Append user info attributes (verified_claims is held separately in VerifiedUserInfo)
	if userInfoAllowedSet != nil {
		for name, value := range claimsRequest.UserInfo {
			if userInfoAllowedSet[name] {
				if value != nil && value.Essential {
					essentialAttributes[name] = true
				} else {
					optionalAttributes[name] = true
				}
			}
		}
	}
}

// appendAttributesFromScopes appends user attributes based on OIDC scopes and app configuration.
func appendAttributesFromScopes(oidcScopes []string, app *providers.OAuthClient,
	idTokenAllowedSet, userInfoAllowedSet map[string]bool, optionalAttributes map[string]bool) {
	for _, scope := range oidcScopes {
		appendAttributesForScope(oauth2utils.ResolveScopeClaims(scope, app.ScopeClaims),
			idTokenAllowedSet, userInfoAllowedSet, optionalAttributes)
	}
}

// appendAttributesForScope appends attributes for a particular scope, allow-listed for either the
// ID token or the UserInfo endpoint.
// When using scopes, all attributes are treated as optional since there is no way to determine
// which attributes are essential vs optional.
func appendAttributesForScope(scopeAttributes []string,
	idTokenAllowedSet, userInfoAllowedSet, optionalAttributes map[string]bool) {
	for _, attribute := range scopeAttributes {
		// A scope claim may be surfaced from the UserInfo endpoint or, when allow-listed for the ID
		// token, embedded in the ID token, so cache the attributes needed by either sink.
		if idTokenAllowedSet != nil && idTokenAllowedSet[attribute] {
			optionalAttributes[attribute] = true
		}
		if userInfoAllowedSet != nil && userInfoAllowedSet[attribute] {
			optionalAttributes[attribute] = true
		}
	}
}

// validateSubClaimConstraint validates the sub claim constraint if specified in the claims parameter.
func validateSubClaimConstraint(claimsRequest *oauth2model.ClaimsRequest, actualSubject string) error {
	if claimsRequest == nil {
		return nil
	}

	// Check id_token sub claim constraint.
	if claimsRequest.IDToken != nil {
		if subReq, exists := claimsRequest.IDToken["sub"]; exists && subReq != nil {
			if !subReq.MatchesValue(actualSubject) {
				return errors.New("sub claim in id_token does not match requested value")
			}
		}
	}

	// Check userinfo sub claim constraint (verified_claims is held separately in VerifiedUserInfo).
	if subReq, exists := claimsRequest.UserInfo["sub"]; exists && subReq != nil {
		if !subReq.MatchesValue(actualSubject) {
			return errors.New("sub claim in userinfo does not match requested value")
		}
	}

	return nil
}

// resolveUserAttributesCacheTTL determines the TTL for caching user attributes based on the
// token validity configuration. The largest of the access and refresh token (if allowed) validity
// periods is taken as the base, then the authorization code validity period is added to cover
// the window between code issuance and token exchange.
// A fixed buffer of attributeCacheTTLBufferSeconds is added to cover the window between
// authentication completion and token issuance.
func (as *authorizeService) resolveUserAttributesCacheTTL(app *providers.OAuthClient) int64 {
	maxTTL := tokenservice.ResolveTokenConfig(
		as.cfg, app, tokenservice.TokenTypeAccess, app.UserAccessTokenConfig().ValidityPeriodOrZero()).ValidityPeriod
	if app.IsAllowedGrantType(providers.GrantTypeRefreshToken) {
		refreshTTL := tokenservice.ResolveTokenConfig(as.cfg, app, tokenservice.TokenTypeRefresh, 0).ValidityPeriod
		if refreshTTL > maxTTL {
			maxTTL = refreshTTL
		}
	}
	authCodeTTL := as.cfg.OAuth.AuthorizationCode.ValidityPeriod
	return maxTTL + authCodeTTL + oauth2const.AttributeCacheTTLBufferSeconds
}

func mapFlowErrorTypeToOAuthError(errorType, description string) (string, string) {
	code := oauth2const.ErrorServerError
	message := "Failed to process authorization request"
	if errorType == flowcm.FlowErrorTypeEndUser {
		code = oauth2const.ErrorAccessDenied
		message = "Access denied"
	}

	if sanitized := oauth2utils.SanitizeErrorDescription(description); sanitized != "" {
		message = sanitized
	}

	return code, message
}
