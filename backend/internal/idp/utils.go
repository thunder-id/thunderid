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
	"context"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// subClaim is the OIDC subject identifier claim, which is always preserved during attribute mapping.
const subClaim = "sub"

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

// GetMappedUserType returns the resolved local user type for the IDP's attribute mapping, or an
// empty string when no mapping is configured. When claim-driven resolution is configured with a
// value mapping (ExternalAttribute + ValueMapping), the user type is derived by mapping the external
// claim value. When an external attribute is configured without a value mapping, the external claim
// value is used directly as the user type. In either case, when the claim is missing or its value is
// unmapped, the configured default user type is returned.
func GetMappedUserType(idp *providers.IDPDTO, claims map[string]interface{}) string {
	if idp == nil || idp.AttributeConfiguration == nil || idp.AttributeConfiguration.UserTypeResolution == nil {
		return ""
	}
	resolution := idp.AttributeConfiguration.UserTypeResolution
	externalAttribute := strings.TrimSpace(resolution.ExternalAttribute)
	if externalAttribute != "" {
		if value, ok := getNestedValue(claims, externalAttribute); ok {
			key := sysutils.ConvertInterfaceValueToString(value)
			if len(resolution.ValueMapping) > 0 {
				if userType, ok := resolution.ValueMapping[key]; ok {
					return userType
				}
			} else if trimmed := strings.TrimSpace(key); trimmed != "" {
				return trimmed
			}
		}
	}
	return resolution.Default
}

// GetAttributeMappings returns the external→local attribute mappings for the resolved user type's
// attributes entry, or nil when no mapping is configured.
func GetAttributeMappings(idp *providers.IDPDTO, claims map[string]interface{}) []providers.AttributeMapping {
	if idp == nil || idp.AttributeConfiguration == nil {
		return nil
	}
	userType := GetMappedUserType(idp, claims)
	if userType == "" {
		return nil
	}
	for i := range idp.AttributeConfiguration.UserTypeAttributeMappings {
		if idp.AttributeConfiguration.UserTypeAttributeMappings[i].UserType == userType {
			return idp.AttributeConfiguration.UserTypeAttributeMappings[i].Attributes
		}
	}
	return nil
}

// ApplyAttributeMappings applies external→local attribute mappings. Unmapped attributes pass through;
// mapped values take precedence on collision. Returns attrs unchanged when no mappings are configured.
func ApplyAttributeMappings(
	attrs map[string]interface{},
	mappings []providers.AttributeMapping,
) map[string]interface{} {
	if len(mappings) == 0 {
		return attrs
	}

	mappedSources := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		// sub is always preserved as-is; it must not be consumed by a mapping.
		if m.ExternalAttribute == subClaim {
			continue
		}
		mappedSources[m.ExternalAttribute] = true
	}

	result := make(map[string]interface{}, len(attrs))
	for key, value := range attrs {
		if !mappedSources[key] {
			result[key] = value
		}
	}
	for _, m := range mappings {
		if value, ok := getNestedValue(attrs, m.ExternalAttribute); ok {
			result[m.LocalAttribute] = value
		}
	}

	return result
}

// getNestedValue resolves a value by exact key first, then by dot-notation path through nested maps.
func getNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	if value, ok := data[path]; ok {
		return value, true
	}
	if !strings.Contains(path, ".") {
		return nil, false
	}

	current := interface{}(data)
	for _, segment := range strings.Split(path, ".") {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		value, exists := obj[segment]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

// validateAttributeMappingShape validates the external→local mappings independently of any user type
// schema: non-empty source/target names and no duplicate targets. A single external attribute may map
// to multiple local attributes, but two external attributes mapping to the same local attribute is a
// conflict and is rejected.
func validateAttributeMappingShape(mappings []providers.AttributeMapping) *tidcommon.ServiceError {
	seenTargets := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		external := strings.TrimSpace(m.ExternalAttribute)
		local := strings.TrimSpace(m.LocalAttribute)
		if external == "" || local == "" {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key:          "error.idpservice.attribute_configuration_empty_claim_description",
				DefaultValue: "attribute mapping must not contain empty attribute names",
			})
		}
		if seenTargets[local] {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key: "error.idpservice.attribute_configuration_duplicate_target_description",
				DefaultValue: "local attribute name '{{param(attribute)}}' appears " +
					"as a mapping target more than once",
				Params: map[string]string{"attribute": local},
			})
		}
		seenTargets[local] = true
	}
	return nil
}

// validateIDP validates the identity provider details.
func validateIDP(ctx context.Context, idp *providers.IDPDTO, logger *log.Logger) *tidcommon.ServiceError {
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
	isValidType := slices.Contains(providers.SupportedIDPTypes, idp.Type)
	if !isValidType {
		return &ErrorInvalidIDPType
	}

	// Validate and apply default properties based on IDP type
	updatedProperties, svcErr := validateIDPProperties(ctx, idp.Type, idp.Properties, logger)
	if svcErr != nil {
		return svcErr
	}
	idp.Properties = updatedProperties

	return nil
}

// validateIDPProperties validates the properties of the identity provider based on its type.
func validateIDPProperties(ctx context.Context, idpType providers.IDPType, properties []cmodels.Property,
	logger *log.Logger) ([]cmodels.Property, *tidcommon.ServiceError) {
	config, exists := idpPropertyConfigs[idpType]
	if !exists {
		logger.Error(ctx, "No property configuration found for IDP type",
			log.String("idpType", string(idpType)))
		return nil, &tidcommon.InternalServerError
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
			return nil, tidcommon.CustomServiceError(ErrorInvalidIDPProperty, tidcommon.I18nMessage{
				Key:          "error.idpservice.property_name_empty_description",
				DefaultValue: "property names cannot be empty",
			})
		}
		if !slices.Contains(allowedProps, propName) {
			return nil, tidcommon.CustomServiceError(ErrorUnsupportedIDPProperty, tidcommon.I18nMessage{
				Key:          "error.idpservice.property_not_supported_for_type_description",
				DefaultValue: "property '{{param(property)}}' is not supported for IDP type '{{param(idpType)}}'",
				Params:       map[string]string{"property": propName, "idpType": string(idpType)},
			})
		}

		propertyValue, err := prop.GetValue()
		if err != nil {
			return nil, tidcommon.CustomServiceError(ErrorInvalidIDPProperty, tidcommon.I18nMessage{
				Key:          "error.idpservice.property_value_get_failed_description",
				DefaultValue: "failed to get value for property '{{param(property)}}': {{param(error)}}",
				Params:       map[string]string{"property": propName, "error": err.Error()},
			})
		}
		if strings.TrimSpace(propertyValue) == "" {
			return nil, tidcommon.CustomServiceError(ErrorInvalidIDPProperty, tidcommon.I18nMessage{
				Key:          "error.idpservice.property_value_empty_description",
				DefaultValue: "value cannot be empty for property '{{param(property)}}'",
				Params:       map[string]string{"property": propName},
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
			return nil, tidcommon.CustomServiceError(ErrorInvalidIDPProperty, tidcommon.I18nMessage{
				Key: "error.idpservice.required_property_missing_description",
				DefaultValue: "required property '{{param(property)}}' is missing " +
					"for IDP type '{{param(idpType)}}'",
				Params: map[string]string{"property": requiredProp, "idpType": string(idpType)},
			})
		}
	}

	// Apply default properties
	for propName, defaultValue := range config.Defaults {
		if _, exists := filteredPropsMap[propName]; !exists {
			err := createAndAppendProperty(ctx, filteredPropsMap, propName, defaultValue, false, logger)
			if err != nil {
				return nil, err
			}
		}
	}

	// Ensure openid scope for OIDC and Google IDPs
	if idpType == providers.IDPTypeOIDC || idpType == providers.IDPTypeGoogle {
		if err := ensureOpenIDScope(ctx, filteredPropsMap, logger); err != nil {
			return nil, err
		}
	}

	return propertyMapToSlice(filteredPropsMap), nil
}

// ensureOpenIDScope ensures that the openid scope is present in the scopes property.
func ensureOpenIDScope(ctx context.Context, propertyMap map[string]cmodels.Property,
	logger *log.Logger) *tidcommon.ServiceError {
	scopesProp, exists := propertyMap[PropScopes]
	if !exists {
		err := createAndAppendProperty(ctx, propertyMap, PropScopes, "openid", false, logger)
		if err != nil {
			return err
		}
		return nil
	}

	scopesValue, err := scopesProp.GetValue()
	if err != nil {
		return tidcommon.CustomServiceError(ErrorInvalidIDPProperty, tidcommon.I18nMessage{
			Key:          "error.idpservice.scopes_value_get_failed_description",
			DefaultValue: "failed to get scopes value: {{param(error)}}",
			Params:       map[string]string{"error": err.Error()},
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
		err := createAndAppendProperty(ctx, propertyMap, PropScopes, "openid", false, logger)
		if err != nil {
			return err
		}
		return nil
	}
	if !slices.Contains(scopes, "openid") {
		scopes = append(scopes, "openid")
		updatedScopes := sysutils.StringifyStringArray(scopes, ",")
		if err := createAndAppendProperty(
			ctx, propertyMap, PropScopes, updatedScopes, scopesProp.IsSecret(), logger); err != nil {
			return err
		}
	}

	return nil
}

// createAndAppendProperty creates a new property and appends it to the property map.
func createAndAppendProperty(ctx context.Context, propertyMap map[string]cmodels.Property,
	name, value string, isSecret bool, logger *log.Logger,
) *tidcommon.ServiceError {
	prop, err := cmodels.NewProperty(name, value, isSecret)
	if err != nil {
		logger.Error(ctx, "Failed to create property", log.String("propertyName", name), log.Error(err))
		return &tidcommon.InternalServerError
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
