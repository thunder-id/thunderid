// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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

// idJagEnabledFromProperties returns a pointer to the parsed id_jag_enabled property value, or nil
// when the property is absent (or its value cannot be parsed as a boolean). This lets basic IDP
// listings distinguish trusted-issuer OIDC connections from plain federation ones without
// exposing full property details.
func idJagEnabledFromProperties(properties []cmodels.Property) *bool {
	for i := range properties {
		if properties[i].GetName() != PropIDJagEnabled {
			continue
		}
		val, err := properties[i].GetValue()
		if err != nil {
			return nil
		}
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return nil
		}
		return &enabled
	}
	return nil
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

// ApplyAttributeMappings applies external→local attribute mappings. Mappings copy rather than rename:
// every incoming attribute is preserved and the mapped value is published under the local name as
// well. Mapped values take precedence on collision. Returns attrs unchanged when no mappings are
// configured.
//
// Copying matters because one external claim can legitimately feed two local attributes, and because
// consuming the source silently drops it. Mapping email onto a required username, for example, would
// otherwise leave the identity with no email at all.
func ApplyAttributeMappings(
	attrs map[string]interface{},
	mappings []providers.AttributeMapping,
) map[string]interface{} {
	if len(mappings) == 0 {
		return attrs
	}

	result := make(map[string]interface{}, len(attrs)+len(mappings))
	for key, value := range attrs {
		result[key] = value
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

// ValidateIDP validates and normalizes dto in place (required properties, type defaults, openid
// scope), mirroring the checks the live /connections create and update APIs run. For use by the
// declarative connection loader, which otherwise writes straight to the file store without
// running this validation.
func ValidateIDP(dto *providers.IDPDTO) error {
	if svcErr := validateIDP(context.Background(), dto, log.GetLogger()); svcErr != nil {
		return errors.New(svcErr.Error.DefaultValue)
	}
	return nil
}

// endpointBaseURLOverride returns the configured mock base URL for the given IDP type
// (google/github only), or "" when no override is configured or the runtime is uninitialized.
// Production config leaves this empty, so the real Google/GitHub endpoints are used unchanged.
func endpointBaseURLOverride(idpType providers.IDPType) string {
	if !sysconfig.IsServerRuntimeInitialized() {
		return ""
	}
	runtime := sysconfig.GetServerRuntime()
	switch idpType {
	case providers.IDPTypeGoogle:
		return runtime.Config.IdentityProvider.GoogleBaseURL
	case providers.IDPTypeGitHub:
		return runtime.Config.IdentityProvider.GitHubBaseURL
	default:
		return ""
	}
}

// resolveEndpointDefaults returns a new map in which each endpoint value's scheme and host are
// replaced by those of base (path and query preserved). When base is empty, isn't a valid http(s)
// URL with a host, or a value fails to parse as a URL, that value is kept unchanged. The input map
// is never mutated.
func resolveEndpointDefaults(defaults map[string]string, base string) map[string]string {
	if base == "" {
		return defaults
	}
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return defaults
	}
	resolved := make(map[string]string, len(defaults))
	for propName, value := range defaults {
		valueURL, err := url.Parse(value)
		if err != nil {
			resolved[propName] = value
			continue
		}
		valueURL.Scheme = baseURL.Scheme
		valueURL.Host = baseURL.Host
		resolved[propName] = valueURL.String()
	}
	return resolved
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

		// The stored value is enough to validate with, and it is all a control plane can offer: it holds
		// a reference to the credential rather than the credential, and no provider to resolve it with.
		propertyValue, err := prop.UnresolvedValue()
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
		if propName == PropIDJagEnabled || propName == PropTokenExchangeEnabled {
			if propertyValue != "true" && propertyValue != "false" {
				return nil, tidcommon.CustomServiceError(ErrorInvalidIDPProperty, tidcommon.I18nMessage{
					Key: "error.idpservice.property_value_not_boolean_description",
					DefaultValue: "value for property '{{param(property)}}' must be either " +
						"'true' or 'false'",
					Params: map[string]string{"property": propName},
				})
			}
		}

		filteredPropsMap[propName] = prop
		filteredPropKeys = append(filteredPropKeys, propName)
	}

	// Check for required properties, using the token-exchange override when applicable.
	// The reduced required set also applies when id_jag_enabled is present, since that property
	// is only ever sent for trust-only connections which carry no OAuth client credentials.
	requiredProps := config.Required
	if teProps, ok := tokenExchangeRequiredProps[idpType]; ok {
		_, idJagEnabledPresent := filteredPropsMap[PropIDJagEnabled]
		tokenExchangeEnabled := false
		if prop, exists := filteredPropsMap[PropTokenExchangeEnabled]; exists {
			if val, err := prop.GetValue(); err == nil && val == "true" {
				tokenExchangeEnabled = true
			}
		}
		if tokenExchangeEnabled || idJagEnabledPresent {
			requiredProps = teProps
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
	effectiveDefaults := resolveEndpointDefaults(config.Defaults, endpointBaseURLOverride(idpType))
	for propName, defaultValue := range effectiveDefaults {
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

	// Seed the email scope for GitHub IDPs
	if idpType == providers.IDPTypeGitHub {
		if err := ensureDefaultGitHubScopes(ctx, filteredPropsMap, logger); err != nil {
			return nil, err
		}
	}

	return propertyMapToSlice(filteredPropsMap), nil
}

// readScopesValue reads the raw value of a scopes property. Shared by the per-provider scope defaults
// so the failure is reported under a single i18n key rather than one per caller.
func readScopesValue(scopesProp cmodels.Property) (string, *tidcommon.ServiceError) {
	scopesValue, err := scopesProp.GetValue()
	if err != nil {
		return "", tidcommon.CustomServiceError(ErrorInvalidIDPProperty, tidcommon.I18nMessage{
			Key:          "error.idpservice.scopes_value_get_failed_description",
			DefaultValue: "failed to get scopes value: {{param(error)}}",
			Params:       map[string]string{"error": err.Error()},
		})
	}
	return scopesValue, nil
}

// ensureOpenIDScope ensures that the openid scope is present in the scopes property,
// defaulting to the standard OIDC scopes (openid, email, profile) when none are supplied.
func ensureOpenIDScope(ctx context.Context, propertyMap map[string]cmodels.Property,
	logger *log.Logger) *tidcommon.ServiceError {
	scopesProp, exists := propertyMap[PropScopes]
	if !exists {
		err := createAndAppendProperty(ctx, propertyMap, PropScopes, defaultOIDCScopes, false, logger)
		if err != nil {
			return err
		}
		return nil
	}

	scopesValue, svcErr := readScopesValue(scopesProp)
	if svcErr != nil {
		return svcErr
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
		err := createAndAppendProperty(ctx, propertyMap, PropScopes, defaultOIDCScopes, false, logger)
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

// ensureDefaultGitHubScopes seeds the GitHub email scope when no scopes are supplied, leaving explicitly
// configured scopes untouched. Applied here rather than through the type's default property map,
// because those values are rewritten as URLs by resolveEndpointDefaults.
func ensureDefaultGitHubScopes(ctx context.Context, propertyMap map[string]cmodels.Property,
	logger *log.Logger) *tidcommon.ServiceError {
	scopesProp, exists := propertyMap[PropScopes]
	if !exists {
		return createAndAppendProperty(ctx, propertyMap, PropScopes, defaultGitHubScopes, false, logger)
	}

	scopesValue, svcErr := readScopesValue(scopesProp)
	if svcErr != nil {
		return svcErr
	}

	for _, scope := range sysutils.ParseStringArray(scopesValue, ",") {
		if scope != "" {
			return nil
		}
	}

	return createAndAppendProperty(ctx, propertyMap, PropScopes, defaultGitHubScopes, false, logger)
}

// scopesGrantEmail reports whether the effective scopes let the connection read an email address.
// Generic OAuth is absent by design as we cannot infer an arbitrary provider's scope semantics.
func scopesGrantEmail(idpType providers.IDPType, scopes []string) bool {
	switch idpType {
	case providers.IDPTypeGoogle, providers.IDPTypeOIDC:
		return slices.Contains(scopes, emailScope)
	case providers.IDPTypeGitHub:
		return slices.Contains(scopes, gitHubUserEmailScope) || slices.Contains(scopes, gitHubUserScope)
	default:
		return false
	}
}

// defaultUsernameSourceAttribute returns the external claim used for the default username mapping,
// or "" when the provider has none. Google and OIDC use email, while GitHub uses its login claim.
func defaultUsernameSourceAttribute(idpType providers.IDPType) string {
	switch idpType {
	case providers.IDPTypeGoogle, providers.IDPTypeOIDC:
		return emailClaim
	case providers.IDPTypeGitHub:
		return gitHubLoginClaim
	default:
		return ""
	}
}

// ensureAttributeConfiguration returns the attribute configuration, creating it when absent.
func ensureAttributeConfiguration(idp *providers.IDPDTO) *providers.AttributeConfiguration {
	if idp.AttributeConfiguration == nil {
		idp.AttributeConfiguration = &providers.AttributeConfiguration{}
	}
	return idp.AttributeConfiguration
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
