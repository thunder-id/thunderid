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

package idp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// GetPropertyValue returns the plain-text value for the named property from the slice,
// or an empty string if the property is absent or its value cannot be retrieved.
func GetPropertyValue(properties []cmodels.Property, name string) string {
	for i := range properties {
		if properties[i].GetName() == name {
			val, err := properties[i].GetValue()
			if err != nil {
				return ""
			}
			return val
		}
	}
	return ""
}

// validateIDP validates the identity provider details.
func validateIDP(idp *IDPDTO, logger *log.Logger) *serviceerror.ServiceError {
	if idp == nil {
		return &ErrorIDPNil
	}
	if strings.TrimSpace(idp.Name) == "" {
		return &ErrorInvalidIDPName
	}

	// Validate identity provider type
	if strings.TrimSpace(string(idp.Type)) == "" {
		return &ErrorInvalidIDPType
	}
	isValidType := slices.Contains(supportedIDPTypes, idp.Type)
	if !isValidType {
		return &ErrorInvalidIDPType
	}

	// Validate and apply default properties based on IDP type
	updatedProperties, svcErr := validateIDPProperties(idp.Type, idp.Properties, logger)
	if svcErr != nil {
		return svcErr
	}
	idp.Properties = updatedProperties

	return nil
}

// validateIDPProperties validates the properties of the identity provider based on its type.
func validateIDPProperties(idpType IDPType, properties []cmodels.Property, logger *log.Logger) (
	[]cmodels.Property, *serviceerror.ServiceError) {
	config, exists := idpPropertyConfigs[idpType]
	if !exists {
		logger.Error("No property configuration found for IDP type", log.String("idpType", string(idpType)))
		return nil, &serviceerror.InternalServerError
	}

	allowedProps := make([]string, 0, len(config.Required)+len(config.Optional))
	allowedProps = append(allowedProps, config.Required...)
	allowedProps = append(allowedProps, config.Optional...)

	// Filter and validate provided properties
	filteredPropsMap := make(map[string]cmodels.Property)
	filteredPropKeys := []string{}
	for _, prop := range properties {
		propName := prop.GetName()
		if strings.TrimSpace(propName) == "" {
			return nil, serviceerror.CustomServiceError(ErrorInvalidIDPProperty, core.I18nMessage{
				Key:          "error.idpservice.property_name_empty_description",
				DefaultValue: "property names cannot be empty",
			})
		}
		if !slices.Contains(allowedProps, propName) {
			return nil, serviceerror.CustomServiceError(ErrorUnsupportedIDPProperty, core.I18nMessage{
				Key:          "error.idpservice.property_not_supported_for_type_description",
				DefaultValue: fmt.Sprintf("property '%s' is not supported for IDP type '%s'", propName, idpType),
			})
		}

		propertyValue, err := prop.GetValue()
		if err != nil {
			return nil, serviceerror.CustomServiceError(ErrorInvalidIDPProperty, core.I18nMessage{
				Key:          "error.idpservice.property_value_get_failed_description",
				DefaultValue: fmt.Sprintf("failed to get value for property '%s': %v", propName, err),
			})
		}
		if strings.TrimSpace(propertyValue) == "" {
			return nil, serviceerror.CustomServiceError(ErrorInvalidIDPProperty, core.I18nMessage{
				Key:          "error.idpservice.property_value_empty_description",
				DefaultValue: fmt.Sprintf("value cannot be empty for property '%s'", propName),
			})
		}

		filteredPropsMap[propName] = prop
		filteredPropKeys = append(filteredPropKeys, propName)
	}

	// Check for required properties, using the token-exchange override when applicable.
	requiredProps := config.Required
	if teProps, ok := tokenExchangeRequiredProps[idpType]; ok {
		if prop, exists := filteredPropsMap[PropTokenExchangeEnabled]; exists {
			if val, err := prop.GetValue(); err == nil && val == "true" {
				requiredProps = teProps
			}
		}
	}
	for _, requiredProp := range requiredProps {
		if !slices.Contains(filteredPropKeys, requiredProp) {
			return nil, serviceerror.CustomServiceError(ErrorInvalidIDPProperty, core.I18nMessage{
				Key:          "error.idpservice.required_property_missing_description",
				DefaultValue: fmt.Sprintf("required property '%s' is missing for IDP type '%s'", requiredProp, idpType),
			})
		}
	}

	// Apply default properties
	for propName, defaultValue := range config.Defaults {
		if _, exists := filteredPropsMap[propName]; !exists {
			if err := createAndAppendProperty(filteredPropsMap, propName, defaultValue, false, logger); err != nil {
				return nil, err
			}
		}
	}

	// Ensure openid scope for OIDC and Google IDPs
	if idpType == IDPTypeOIDC || idpType == IDPTypeGoogle {
		if err := ensureOpenIDScope(filteredPropsMap, logger); err != nil {
			return nil, err
		}
	}

	return propertyMapToSlice(filteredPropsMap), nil
}

// ensureOpenIDScope ensures that the openid scope is present in the scopes property.
func ensureOpenIDScope(propertyMap map[string]cmodels.Property, logger *log.Logger) *serviceerror.ServiceError {
	scopesProp, exists := propertyMap[PropScopes]
	if !exists {
		err := createAndAppendProperty(propertyMap, PropScopes, "openid", false, logger)
		if err != nil {
			return err
		}
		return nil
	}

	scopesValue, err := scopesProp.GetValue()
	if err != nil {
		return serviceerror.CustomServiceError(ErrorInvalidIDPProperty, core.I18nMessage{
			Key:          "error.idpservice.scopes_value_get_failed_description",
			DefaultValue: fmt.Sprintf("failed to get scopes value: %v", err),
		})
	}

	scopes := sysutils.ParseStringArray(scopesValue, ",")
	filteredScopes := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope != "" {
			filteredScopes = append(filteredScopes, scope)
		}
	}
	scopes = filteredScopes

	if len(scopes) == 0 {
		err := createAndAppendProperty(propertyMap, PropScopes, "openid", false, logger)
		if err != nil {
			return err
		}
		return nil
	}
	if !slices.Contains(scopes, "openid") {
		scopes = append(scopes, "openid")
		updatedScopes := sysutils.StringifyStringArray(scopes, ",")
		if err := createAndAppendProperty(
			propertyMap, PropScopes, updatedScopes, scopesProp.IsSecret(), logger); err != nil {
			return err
		}
	}

	return nil
}

// createAndAppendProperty creates a new property and appends it to the property map.
func createAndAppendProperty(propertyMap map[string]cmodels.Property,
	name, value string, isSecret bool, logger *log.Logger,
) *serviceerror.ServiceError {
	prop, err := cmodels.NewProperty(name, value, isSecret)
	if err != nil {
		logger.Error("Failed to create property", log.String("propertyName", name), log.Error(err))
		return &serviceerror.InternalServerError
	}
	propertyMap[name] = *prop
	return nil
}

// propertyMapToSlice converts a property map to a slice.
func propertyMapToSlice(propertyMap map[string]cmodels.Property) []cmodels.Property {
	properties := make([]cmodels.Property, 0, len(propertyMap))
	for _, prop := range propertyMap {
		properties = append(properties, prop)
	}
	return properties
}
