// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/internal/flow/session"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// AuthorizeHandlerInterface defines the interface for handling OAuth2 authorization requests.
type AuthorizeHandlerInterface interface {
	HandleAuthorizeGetRequest(w http.ResponseWriter, r *http.Request)
}

// authorizeHandler implements the AuthorizeHandlerInterface for handling OAuth2 authorization requests.
type authorizeHandler struct {
	cfg          oauthconfig.Config
	authZService AuthorizeServiceInterface
	// ssoTransport reads the inbound SSO handle cookies; the authorize endpoint only reads them,
	// the flow endpoint remains the sole writer.
	ssoTransport session.HandleTransport
	logger       *log.Logger
}

// newAuthorizeHandler creates a new instance of authorizeHandler with injected dependencies.
func newAuthorizeHandler(authZService AuthorizeServiceInterface, cfg oauthconfig.Config) AuthorizeHandlerInterface {
	return &authorizeHandler{
		cfg:          cfg,
		authZService: authZService,
		ssoTransport: session.NewCookieTransport(true),
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizeHandler")),
	}
}

// HandleAuthorizeGetRequest handles the GET request for OAuth2 authorization.
func (ah *authorizeHandler) HandleAuthorizeGetRequest(w http.ResponseWriter, r *http.Request) {
	// Validate the request before touching it: getOAuthMessage handles a nil request or response
	// writer, and both the context and the cookie read below dereference it.
	oAuthMessage := ah.getOAuthMessage(r, w)
	if oAuthMessage == nil {
		return
	}

	// Carry the inbound SSO handle cookies so prompt=none can be answered from an existing
	// session. The per-flow cookie is selected later, once the client's flow is known.
	ctx := session.WithInbound(r.Context(), ah.ssoTransport.Read(r))

	result, authErr := ah.authZService.HandleInitialAuthorizationRequest(ctx, oAuthMessage)
	if authErr != nil {
		if authErr.SendErrorToClient {
			queryParams := map[string]string{
				oauth2const.RequestParamError:            authErr.Code,
				oauth2const.RequestParamErrorDescription: authErr.Message,
				oauth2const.RequestParamIss:              ah.cfg.JWT.Issuer,
			}
			if authErr.State != "" {
				queryParams[oauth2const.RequestParamState] = authErr.State
			}
			redirectURI, err := oauth2utils.GetURIWithQueryParams(authErr.ClientRedirectURI, queryParams)
			if err != nil {
				ah.logger.Error(ctx, "Failed to construct client redirect URI", log.Error(err))
				ah.redirectToErrorPage(w, r, oauth2const.ErrorServerError, "Failed to process authorization request")
				return
			}
			http.Redirect(w, r, redirectURI, http.StatusFound)
			return
		}
		ah.redirectToErrorPage(w, r, authErr.Code, authErr.Message)
		return
	}

	ah.redirectToLoginPage(w, r, result.QueryParams)
}

// getOAuthMessage extracts the OAuth message from the request and response writer.
func (ah *authorizeHandler) getOAuthMessage(r *http.Request, w http.ResponseWriter) *OAuthMessage {
	logger := ah.logger

	if r == nil || w == nil {
		// The request may be nil here, so there is no request context to propagate.
		logger.Error(context.Background(), "Request or response writer is nil")
		return nil
	}

	var msg *OAuthMessage
	var err error

	switch r.Method {
	case http.MethodGet:
		msg, err = ah.getOAuthMessageForGetRequest(r)
	default:
		err = errors.New("unsupported request method: " + r.Method)
	}

	if err != nil {
		ah.logger.Debug(r.Context(), "Invalid authorize request", log.Error(err))
		sysutils.WriteJSONError(r.Context(), w, oauth2const.ErrorInvalidRequest, "Invalid authorization request",
			http.StatusBadRequest, nil)
	}

	return msg
}

// getOAuthMessageForGetRequest extracts the OAuth message from an authorization GET request.
// Only the resource parameter is permitted to be repeated (RFC 8707 §2). Any other parameter
// appearing more than once is rejected with invalid_request per RFC 6749 §3.1.
func (ah *authorizeHandler) getOAuthMessageForGetRequest(r *http.Request) (*OAuthMessage, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("failed to parse form data: %w", err)
	}

	queryParams := r.URL.Query()
	var resources []string
	for key, values := range queryParams {
		if key == oauth2const.RequestParamResource {
			resources = values
			continue
		}
		if len(values) > 1 {
			return nil, fmt.Errorf("query parameter %q must not be repeated", key)
		}
	}

	return &OAuthMessage{
		RequestType:        oauth2const.TypeInitialAuthorizationRequest,
		Resources:          resources,
		RequestHeaders:     sysutils.SanitizeRawMultiValueStringMap(r.Header),
		RequestQueryParams: sysutils.SanitizeRawMultiValueStringMap(queryParams),
	}, nil
}

// getLoginPageRedirectURI constructs the login page URL with the provided query parameters.
func getLoginPageRedirectURI(gateClientConfig oauthconfig.Config, queryParams map[string]string) (string, error) {
	loginPageURL := (&url.URL{
		Scheme: gateClientConfig.GateClient.Scheme,
		Host:   fmt.Sprintf("%s:%d", gateClientConfig.GateClient.Hostname, gateClientConfig.GateClient.Port),
		Path:   gateClientConfig.GateClient.LoginPath,
	}).String()

	return oauth2utils.GetURIWithQueryParams(loginPageURL, queryParams)
}

// redirectToLoginPage constructs the login page URL and redirects the user to it.
func (ah *authorizeHandler) redirectToLoginPage(w http.ResponseWriter, r *http.Request,
	queryParams map[string]string) {
	logger := ah.logger

	if w == nil || r == nil {
		// The request may be nil here, so there is no request context to propagate.
		logger.Error(context.Background(),
			"Response writer or request is nil. Cannot redirect to login page.")
		return
	}
	ctx := r.Context()

	redirectURI, err := getLoginPageRedirectURI(ah.cfg, queryParams)
	if err != nil {
		logger.Error(ctx, "Failed to construct login page URL", log.Error(err))
		ah.redirectToErrorPage(w, r, oauth2const.ErrorServerError, "Failed to process authorization request")
		return
	}
	logger.Debug(ctx, "Redirecting to login page")

	http.Redirect(w, r, redirectURI, http.StatusFound)
}

// getErrorPageRedirectURL constructs the error page URL with the provided error code and message.
func getErrorPageRedirectURL(cfg oauthconfig.Config, code, msg string) (string, error) {
	errorPageURL := (&url.URL{
		Scheme: cfg.GateClient.Scheme,
		Host:   fmt.Sprintf("%s:%d", cfg.GateClient.Hostname, cfg.GateClient.Port),
		Path:   cfg.GateClient.ErrorPath,
	}).String()

	queryParams := map[string]string{
		"errorCode":    code,
		"errorMessage": msg,
	}

	return oauth2utils.GetURIWithQueryParams(errorPageURL, queryParams)
}

// redirectToErrorPage constructs the error page URL and redirects the user to it.
func (ah *authorizeHandler) redirectToErrorPage(w http.ResponseWriter, r *http.Request, code, msg string) {
	logger := ah.logger

	if w == nil || r == nil {
		// The request may be nil here, so there is no request context to propagate.
		logger.Error(context.Background(),
			"Response writer or request is nil. Cannot redirect to error page.")
		return
	}
	ctx := r.Context()

	redirectURL, err := getErrorPageRedirectURL(ah.cfg, code, msg)
	if err != nil {
		logger.Error(ctx, "Failed to construct error page URL", log.Error(err))
		http.Error(w, "Failed to redirect to error page", http.StatusInternalServerError)
		return
	}
	logger.Debug(ctx, "Redirecting to error page")

	http.Redirect(w, r, redirectURL, http.StatusFound)
}
