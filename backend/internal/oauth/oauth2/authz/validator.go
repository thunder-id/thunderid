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
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz/requestvalidator"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// AuthorizationValidatorInterface defines the interface for validating OAuth2 authorization requests.
type AuthorizationValidatorInterface interface {
	validateInitialAuthorizationRequest(msg *OAuthMessage, oauthApp *inboundmodel.OAuthClient) (
		bool, string, string)
}

// authorizationValidator implements the AuthorizationValidatorInterface for validating OAuth2 authorization requests.
type authorizationValidator struct{}

// newAuthorizationValidator creates a new instance of authorizationValidator.
func newAuthorizationValidator() AuthorizationValidatorInterface {
	return &authorizationValidator{}
}

// validateInitialAuthorizationRequest validates the initial authorization request parameters.
func (av *authorizationValidator) validateInitialAuthorizationRequest(msg *OAuthMessage,
	oauthApp *inboundmodel.OAuthClient) (bool, string, string) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizationValidator"))

	clientID := msg.RequestQueryParams[constants.RequestParamClientID]
	redirectURI := msg.RequestQueryParams[constants.RequestParamRedirectURI]

	if clientID == "" {
		return false, constants.ErrorInvalidRequest, "Missing client_id parameter"
	}

	if err := oauthApp.ValidateRedirectURI(redirectURI); err != nil {
		logger.Debug("Validation failed for redirect URI", log.Error(err))
		return false, constants.ErrorInvalidRequest, "Invalid redirect URI"
	}

	// All subsequent validation errors can be sent to the client application via redirect.
	errCode, errMsg := requestvalidator.ValidateAuthorizationRequestParams(msg.RequestQueryParams, oauthApp)
	if errCode != "" {
		return true, errCode, errMsg
	}

	if errResp := resourceindicators.ValidateResourceURIs(msg.Resources); errResp != nil {
		return true, errResp.Error, errResp.ErrorDescription
	}

	return false, "", ""
}
