/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package authz

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// AuthorizeHandlerInterface defines the interface for handling OAuth2 authorization requests.
type AuthorizeHandlerInterface interface {
	HandleAuthorizeGetRequest(w http.ResponseWriter, r *http.Request)
	HandleAuthCallbackPostRequest(w http.ResponseWriter, r *http.Request)
}

// authorizeHandler implements the AuthorizeHandlerInterface for handling OAuth2 authorization requests.
type authorizeHandler struct {
	authZService AuthorizeServiceInterface
	logger       *log.Logger
}

// newAuthorizeHandler creates a new instance of authorizeHandler with injected dependencies.
func newAuthorizeHandler(authZService AuthorizeServiceInterface) AuthorizeHandlerInterface {
	return &authorizeHandler{
		authZService: authZService,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizeHandler")),
	}
}

// HandleAuthorizeGetRequest handles the GET request for OAuth2 authorization.
func (ah *authorizeHandler) HandleAuthorizeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	oAuthMessage := ah.getOAuthMessage(r, w)
	if oAuthMessage == nil {
		return
	}

	result, authErr := ah.authZService.HandleInitialAuthorizationRequest(ctx, oAuthMessage)
	if authErr != nil {
		if authErr.SendErrorToClient {
			queryParams := map[string]string{
				oauth2const.RequestParamError:            authErr.Code,
				oauth2const.RequestParamErrorDescription: authErr.Message,
				oauth2const.RequestParamIss:              config.GetServerRuntime().Config.JWT.Issuer,
			}
			if authErr.State != "" {
				queryParams[oauth2const.RequestParamState] = authErr.State
			}
			redirectURI, err := oauth2utils.GetURIWithQueryParams(authErr.ClientRedirectURI, queryParams)
			if err != nil {
				ah.logger.Error("Failed to construct client redirect URI", log.Error(err))
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

// HandleAuthCallbackPostRequest handles the POST request for OAuth2 auth callback.
// This endpoint receives the assertion from the flow engine after successful authentication.
func (ah *authorizeHandler) HandleAuthCallbackPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	oAuthMessage := ah.getOAuthMessage(r, w)
	if oAuthMessage == nil {
		return
	}

	switch oAuthMessage.RequestType {
	case oauth2const.TypeAuthorizationResponseFromEngine:
		authID := oAuthMessage.AuthID
		assertion := oAuthMessage.RequestBodyParams[oauth2const.Assertion]

		redirectURI, authErr := ah.authZService.HandleAuthorizationCallback(ctx, authID, assertion)
		if authErr != nil {
			if authErr.SendErrorToClient {
				ah.writeAuthZResponseToClientRedirect(w, authErr)
				return
			}
			ah.writeAuthZResponseToErrorPage(w, authErr.Code, authErr.Message, authErr.State)
			return
		}
		ah.writeAuthZResponse(w, redirectURI)

	case oauth2const.TypeConsentResponseFromUser:
		// TODO: Handle the consent response from the user.
		//  Verify whether we need separate session data key for consent flow.
		//  Alternatively could add consent info also to the same session object.
	default:
		utils.WriteJSONError(w, oauth2const.ErrorInvalidRequest, "Invalid authorization request",
			http.StatusBadRequest, nil)
	}
}

// getOAuthMessage extracts the OAuth message from the request and response writer.
func (ah *authorizeHandler) getOAuthMessage(r *http.Request, w http.ResponseWriter) *OAuthMessage {
	logger := ah.logger

	if r == nil || w == nil {
		logger.Error("Request or response writer is nil")
		return nil
	}

	var msg *OAuthMessage
	var err error

	switch r.Method {
	case http.MethodGet:
		msg, err = ah.getOAuthMessageForGetRequest(r)
	case http.MethodPost:
		msg, err = ah.getOAuthMessageForPostRequest(r)
	default:
		err = errors.New("unsupported request method: " + r.Method)
	}

	if err != nil {
		ah.logger.Debug("Invalid authorize request", log.Error(err))
		utils.WriteJSONError(w, oauth2const.ErrorInvalidRequest, "Invalid authorization request",
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

	queryParams := make(map[string]string)
	var resources []string
	for key, values := range r.URL.Query() {
		if len(values) == 0 {
			continue
		}
		if key == oauth2const.RequestParamResource {
			resources = values
			queryParams[key] = values[0]
			continue
		}
		if len(values) > 1 {
			return nil, fmt.Errorf("query parameter %q must not be repeated", key)
		}
		queryParams[key] = values[0]
	}

	return &OAuthMessage{
		RequestType:        oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: queryParams,
		Resources:          resources,
	}, nil
}

// getOAuthMessageForPostRequest extracts the OAuth message from an authorization POST request.
func (ah *authorizeHandler) getOAuthMessageForPostRequest(r *http.Request) (*OAuthMessage, error) {
	authZReq, err := utils.DecodeJSONBody[AuthZPostRequest](r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON body: %w", err)
	}

	if authZReq.AuthID == "" || authZReq.Assertion == "" {
		return nil, errors.New("authId or assertion is missing")
	}

	// TODO: Require to handle other types such as user consent, etc.
	bodyParams := map[string]string{
		oauth2const.Assertion: authZReq.Assertion,
	}

	return &OAuthMessage{
		RequestType:       oauth2const.TypeAuthorizationResponseFromEngine,
		AuthID:            authZReq.AuthID,
		RequestBodyParams: bodyParams,
	}, nil
}

// getLoginPageRedirectURI constructs the login page URL with the provided query parameters.
func getLoginPageRedirectURI(queryParams map[string]string) (string, error) {
	gateClientConfig := config.GetServerRuntime().Config.GateClient
	loginPageURL := (&url.URL{
		Scheme: gateClientConfig.Scheme,
		Host:   fmt.Sprintf("%s:%d", gateClientConfig.Hostname, gateClientConfig.Port),
		Path:   gateClientConfig.LoginPath,
	}).String()

	return oauth2utils.GetURIWithQueryParams(loginPageURL, queryParams)
}

// redirectToLoginPage constructs the login page URL and redirects the user to it.
func (ah *authorizeHandler) redirectToLoginPage(w http.ResponseWriter, r *http.Request,
	queryParams map[string]string) {
	logger := ah.logger

	if w == nil || r == nil {
		logger.Error("Response writer or request is nil. Cannot redirect to login page.")
		return
	}

	redirectURI, err := getLoginPageRedirectURI(queryParams)
	if err != nil {
		logger.Error("Failed to construct login page URL", log.Error(err))
		ah.redirectToErrorPage(w, r, oauth2const.ErrorServerError, "Failed to process authorization request")
		return
	}
	logger.Debug("Redirecting to login page")

	http.Redirect(w, r, redirectURI, http.StatusFound)
}

// getErrorPageRedirectURL constructs the error page URL with the provided error code and message.
func getErrorPageRedirectURL(code, msg string) (string, error) {
	gateClientConfig := config.GetServerRuntime().Config.GateClient
	errorPageURL := (&url.URL{
		Scheme: gateClientConfig.Scheme,
		Host:   fmt.Sprintf("%s:%d", gateClientConfig.Hostname, gateClientConfig.Port),
		Path:   gateClientConfig.ErrorPath,
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
		logger.Error("Response writer or request is nil. Cannot redirect to error page.")
		return
	}

	redirectURL, err := getErrorPageRedirectURL(code, msg)
	if err != nil {
		logger.Error("Failed to construct error page URL", log.Error(err))
		http.Error(w, "Failed to redirect to error page", http.StatusInternalServerError)
		return
	}
	logger.Debug("Redirecting to error page")

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// writeAuthZResponse writes the authorization response to the HTTP response writer.
func (ah *authorizeHandler) writeAuthZResponse(w http.ResponseWriter, redirectURI string) {
	authZResp := AuthZPostResponse{
		RedirectURI: redirectURI,
	}
	utils.WriteSuccessResponse(w, http.StatusOK, authZResp)
}

// writeAuthZResponseToErrorPage writes the authorization response redirecting to the error page.
// The state parameter is included in the redirect if non-empty.
func (ah *authorizeHandler) writeAuthZResponseToErrorPage(w http.ResponseWriter, code, msg, state string) {
	redirectURI, err := getErrorPageRedirectURL(code, msg)
	if err != nil {
		http.Error(w, "Failed to redirect to error page", http.StatusInternalServerError)
		return
	}

	if state != "" {
		queryParams := map[string]string{
			oauth2const.RequestParamState: state,
		}
		redirectURI, err = oauth2utils.GetURIWithQueryParams(redirectURI, queryParams)
		if err != nil {
			http.Error(w, "Failed to redirect to error page", http.StatusInternalServerError)
			return
		}
	}

	ah.writeAuthZResponse(w, redirectURI)
}

// writeAuthZResponseToClientRedirect writes the authorization error response redirecting to the
// client's registered redirect URI.
func (ah *authorizeHandler) writeAuthZResponseToClientRedirect(w http.ResponseWriter, authErr *AuthorizationError) {
	queryParams := map[string]string{
		oauth2const.RequestParamError:            authErr.Code,
		oauth2const.RequestParamErrorDescription: authErr.Message,
		oauth2const.RequestParamIss:              config.GetServerRuntime().Config.JWT.Issuer,
	}
	if authErr.State != "" {
		queryParams[oauth2const.RequestParamState] = authErr.State
	}

	redirectURI, err := oauth2utils.GetURIWithQueryParams(authErr.ClientRedirectURI, queryParams)
	if err != nil {
		ah.logger.Error("Failed to construct client redirect URI", log.Error(err))
		ah.writeAuthZResponseToErrorPage(w, oauth2const.ErrorServerError,
			"Failed to process authorization request", authErr.State)
		return
	}

	ah.writeAuthZResponse(w, redirectURI)
}
