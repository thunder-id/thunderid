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

package executor

import (
	"encoding/json"
	"errors"
	"fmt"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// getAuthnServiceName returns the authn service name for an executor.
// Returns empty string if executor doesn't map to an authn service.
func getAuthnServiceName(executorName string) string {
	executorToAuthnServiceMap := map[string]string{
		ExecutorNameBasicAuth:  authncm.AuthenticatorCredentials,
		ExecutorNameSMSAuth:    authncm.AuthenticatorSMSOTP,
		ExecutorNameOAuth:      authncm.AuthenticatorOAuth,
		ExecutorNameOIDCAuth:   authncm.AuthenticatorOIDC,
		ExecutorNameGitHubAuth: authncm.AuthenticatorGithub,
		ExecutorNameGoogleAuth: authncm.AuthenticatorGoogle,
	}
	return executorToAuthnServiceMap[executorName]
}

// GetUserAttribute extracts a specific attribute value from a user entity's JSON attributes.
func GetUserAttribute(user *entityprovider.Entity, attributeKey string) (string, error) {
	if user == nil || len(user.Attributes) == 0 {
		return "", errors.New("user entity or attributes are empty")
	}

	var attrs map[string]interface{}
	if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
		return "", errors.New("failed to parse user attributes")
	}

	if val, ok := attrs[attributeKey]; ok {
		if strVal, isString := val.(string); isString && strVal != "" {
			return strVal, nil
		}
	}

	return "", fmt.Errorf("attribute '%s' not found, empty, or not a string", attributeKey)
}

// resolveInputIdentifierByType returns the identifier of the first input in ctx.NodeInputs matching inputType,
// or fallback if none is found.
func resolveInputIdentifierByType(ctx *core.NodeContext, inputType string, fallback string) string {
	if input, ok := findInputByType(ctx.NodeInputs, inputType); ok {
		return input.Identifier
	}
	return fallback
}

// findInputByType returns the first input in the given slice whose Type matches inputType.
func findInputByType(inputs []common.Input, inputType string) (common.Input, bool) {
	for _, input := range inputs {
		if input.Type == inputType {
			return input, true
		}
	}
	return common.Input{}, false
}

// isAuthenticationWithoutLocalUserAllowed returns the value of the AllowAuthenticationWithoutLocalUser
// node property, defaulting to false if absent or not a bool.
// This is used to determine if authentication flow can proceed without a local user account.
// Idea is to use this in authentication flows which has a ProvisioningExecutor attached at the end
// to provision the user account and auto login without throwing an error for user not found.
func isAuthenticationWithoutLocalUserAllowed(ctx *core.NodeContext) bool {
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// isRegistrationWithExistingUserAllowed returns the value of the AllowRegistrationWithExistingUser
// node property, defaulting to false if absent or not a bool.
// This is used to determine if registration flow can proceed when an existing user account is found.
// Idea is to use this in registration flows which can continue with the existing user account
// instead of throwing an error for user already exists and allow the flow to complete successfully.
func isRegistrationWithExistingUserAllowed(ctx *core.NodeContext) bool {
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowRegistrationWithExistingUser]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// isCrossOUProvisioningAllowed returns the value of the AllowCrossOUProvisioning node property,
// defaulting to false if absent or not a bool.
// This is used to determine if provisioning can proceed across organizational units (OUs).
// Idea is to use this in registration flows which can continue even if an existing user account
// is found, but the provisioning executor is trying to provision the user to a different OU than
// the one in the existing account.
func isCrossOUProvisioningAllowed(ctx *core.NodeContext) bool {
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowCrossOUProvisioning]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

func isAllowRegistrationWithExistingUserRuntimeFlagSet(ctx *core.NodeContext) bool {
	val, ok := ctx.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser]
	return ok && val == dataValueTrue
}

// validateFederatedIdentifierConsistency checks if the federated identifiers from the authentication result
// are consistent with any existing identifiers in the context (runtime data, user inputs, authenticated
// user attributes).
func validateFederatedIdentifierConsistency(ctx *core.NodeContext,
	federatedIdentifiers, existingIdentifiers map[string]interface{}) bool {
	if len(federatedIdentifiers) == 0 {
		return true
	}

	// TODO: Refine this well-known-key comparison when IDP-to-local attribute mapping is supported
	fedIdfConsistencyKeys := []string{userAttributeEmail, userAttributeSub}
	for _, key := range fedIdfConsistencyKeys {
		federatedValue := ""
		if value, ok := federatedIdentifiers[key]; ok {
			federatedValue = systemutils.ConvertInterfaceValueToString(value)
		}

		if federatedValue == "" {
			continue
		}

		if value, ok := ctx.RuntimeData[key]; ok && value != "" && value != federatedValue {
			return false
		}
		if value, ok := ctx.UserInputs[key]; ok && value != "" && value != federatedValue {
			return false
		}
		if value := existingIdentifiers[key]; value != nil &&
			systemutils.ConvertInterfaceValueToString(value) != "" &&
			systemutils.ConvertInterfaceValueToString(value) != federatedValue {
			return false
		}
	}

	return true
}
