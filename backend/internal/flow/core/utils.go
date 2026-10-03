// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ExternalIdentity is what an external party (a federated identity provider or a credential issuer)
// asserted, as published under common.RuntimeKeyExternalIdentity. IdpID and Sub are set only for a
// federated connection.
type ExternalIdentity struct {
	IdpID  string                 `json:"idpId,omitempty"`
	Sub    string                 `json:"sub,omitempty"`
	Claims map[string]interface{} `json:"claims,omitempty"`
}

// GetExternalIdentity decodes the external identity from runtime data. It returns nil when none is
// published or the entry does not decode.
func GetExternalIdentity(runtimeData map[string]string) *ExternalIdentity {
	raw := runtimeData[common.RuntimeKeyExternalIdentity]
	if raw == "" {
		return nil
	}
	var identity ExternalIdentity
	if err := json.Unmarshal([]byte(raw), &identity); err != nil {
		return nil
	}
	return &identity
}

// GetExternalClaim returns one claim of the external identity as a string, and whether it is present.
// Readers of attribute values consult it after their own sources. Reads of flow control state never
// do, which is what keeps an external party from steering the flow.
func GetExternalClaim(runtimeData map[string]string, name string) (string, bool) {
	identity := GetExternalIdentity(runtimeData)
	if identity == nil {
		return "", false
	}
	value, ok := identity.Claims[name]
	if !ok {
		return "", false
	}
	return systemutils.ConvertInterfaceValueToString(value), true
}

// placeholderPattern matches {{ctx(key)}} with optional whitespace.
// TODO: Extend to support {{user(key)}}, {{env(key)}}, etc.
var placeholderPattern = regexp.MustCompile(`{{\s*ctx\(\s*(\w+)\s*\)\s*}}`)

// ResolvePlaceholder resolves a single placeholder string using the "{{ctx(key)}}" syntax.
// If no placeholder is found, the original value is returned.
// If a placeholder is found but the key doesn't exist in any data source, the placeholder is kept as-is.
func ResolvePlaceholder(ctx *providers.NodeContext, value string, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager, logger *log.Logger) string {
	return resolvePlaceholder(ctx, value, execResp, authnProvider, logger, true)
}

// resolvePlaceholder resolves placeholders as ResolvePlaceholder does, falling back to the external
// identity's claims only when allowExternalClaims is set. Flow control reads leave it unset.
func resolvePlaceholder(ctx *providers.NodeContext, value string, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager, logger *log.Logger, allowExternalClaims bool) string {
	if ctx == nil {
		return value
	}

	var contextUserRef *providers.EntityReference

	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		submatches := placeholderPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		key := submatches[1]

		// Special handling for userId - only resolve from runtime data or authenticated user
		if key == "userId" {
			if runtimeValue, ok := ctx.RuntimeData["userId"]; ok && runtimeValue != "" {
				return runtimeValue
			}
			if authnProvider != nil && execResp != nil && ctx.AuthUser.IsAuthenticated() && contextUserRef == nil {
				contextUserRef = fetchContextUserRef(authnProvider, ctx, execResp, logger, contextUserRef)
			}
			if contextUserRef != nil && contextUserRef.EntityID != "" {
				return contextUserRef.EntityID
			}

			return match // Keep placeholder if not found
		}

		// Special handling for ouId - only resolve from runtime data or authenticated user
		if key == "ouId" {
			if runtimeValue, ok := ctx.RuntimeData["ouId"]; ok && runtimeValue != "" {
				return runtimeValue
			}
			if authnProvider != nil && execResp != nil && ctx.AuthUser.IsAuthenticated() && contextUserRef == nil {
				contextUserRef = fetchContextUserRef(authnProvider, ctx, execResp, logger, contextUserRef)
			}
			if contextUserRef != nil && contextUserRef.OUID != "" {
				return contextUserRef.OUID
			}
			return match // Keep placeholder if not found
		}

		// Check runtime data first
		if runtimeValue, ok := ctx.RuntimeData[key]; ok && runtimeValue != "" {
			return runtimeValue
		}
		if allowExternalClaims {
			if claimValue, ok := GetExternalClaim(ctx.RuntimeData, key); ok && claimValue != "" {
				return claimValue
			}
		}

		// Check user inputs next
		if userInputValue, ok := ctx.UserInputs[key]; ok && userInputValue != "" {
			return userInputValue
		}

		// If not found, keep the placeholder as-is
		return match
	})
}

// fetchContextUserRef attempts to resolve the authenticated user's entity reference using the authn provider.
func fetchContextUserRef(
	authnProvider providers.AuthnProviderManager,
	ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse,
	logger *log.Logger,
	contextUserRef *providers.EntityReference,
) *providers.EntityReference {
	authUser, userRef, err := authnProvider.GetEntityReference(ctx.Context, ctx.AuthUser)
	execResp.AuthUser = authUser
	if err != nil {
		logger.Warn(ctx.Context, "Failed to resolve authenticated user reference for userId placeholder")
	} else {
		contextUserRef = userRef
	}
	return contextUserRef
}

// ParsePresentedOptionalInputIdentifiers converts a space-separated identifier list into a set.
func ParsePresentedOptionalInputIdentifiers(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	if raw == "" {
		return result
	}

	for _, identifier := range strings.Fields(raw) {
		if identifier != "" {
			result[identifier] = struct{}{}
		}
	}

	return result
}

// GetPresentedOptionalInputs extracts and parses the presented optional input identifiers from
// runtime data into a set. Call this once before a loop to avoid repeated string parsing.
func GetPresentedOptionalInputs(runtimeData map[string]string) map[string]struct{} {
	return ParsePresentedOptionalInputIdentifiers(runtimeData[common.RuntimeKeyPresentedOptionalInputs])
}

// hasPresentedOptionalInput returns true when the given identifier appears in the presented-input set.
func hasPresentedOptionalInput(presentedInputs map[string]struct{}, identifier string) bool {
	if identifier == "" || len(presentedInputs) == 0 {
		return false
	}

	_, ok := presentedInputs[identifier]
	return ok
}

// IsOptionalInputPrompted returns true when an optional input identifier has already been shown
// to the user in a prior prompt step.
func IsOptionalInputPrompted(presentedOptionalInputs map[string]struct{}, identifier string) bool {
	return hasPresentedOptionalInput(presentedOptionalInputs, identifier)
}

// collectMissingInputs returns inputs from requiredInputs that are not satisfied by user inputs,
// runtime data, forwarded data, or already-presented optional inputs.
func collectMissingInputs(ctx *providers.NodeContext, presentedOptionalInputs map[string]struct{},
	requiredInputs []providers.Input, logger *log.Logger) []providers.Input {
	missing := make([]providers.Input, 0, len(requiredInputs))
	for _, input := range requiredInputs {
		if _, ok := ctx.UserInputs[input.Identifier]; ok {
			continue
		}
		if _, ok := ctx.RuntimeData[input.Identifier]; ok {
			logger.Debug(ctx.Context, "Input available in runtime data, skipping",
				log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
			continue
		}
		if value, ok := ctx.ForwardedData[input.Identifier]; ok {
			if _, isString := value.(string); isString {
				logger.Debug(ctx.Context, "Input available in forwarded data, skipping",
					log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
				continue
			}
		}
		if _, ok := GetExternalClaim(ctx.RuntimeData, input.Identifier); ok {
			logger.Debug(ctx.Context, "Input available in external claims, skipping",
				log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
			continue
		}
		if !input.Required && IsOptionalInputPrompted(presentedOptionalInputs, input.Identifier) {
			logger.Debug(ctx.Context, "Optional input already prompted, skipping",
				log.String("identifier", input.Identifier))
			continue
		}
		logger.Debug(ctx.Context, "Input not available in the context",
			log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
		missing = append(missing, input)
	}
	return missing
}

// MergePresentedOptionalInputIdentifiers appends identifiers to an existing serialized identifier string.
// Duplicates are acceptable since ParsePresentedOptionalInputIdentifiers deduplicates on read.
func MergePresentedOptionalInputIdentifiers(raw string, identifiers []string) string {
	parts := make([]string, 0, len(identifiers)+1)
	if raw != "" {
		parts = append(parts, raw)
	}
	for _, identifier := range identifiers {
		if identifier != "" {
			parts = append(parts, identifier)
		}
	}
	return strings.Join(parts, " ")
}

// BuildProviderMetadata constructs the metadata for providers. It includes
// provider_ext_* runtime keys and the initiator request data (headers and query params).
func BuildProviderMetadata(ctx *providers.NodeContext) *providers.AuthnMetadata {
	return &providers.AuthnMetadata{
		RuntimeMetadata: buildRuntimeMetadata(ctx),
	}
}

// BuildGetAttributesMetadata constructs the metadata for fetching user attributes. It includes
// provider_ext_* runtime keys and the initiator request data (headers and query params).
func BuildGetAttributesMetadata(ctx *providers.NodeContext) *providers.GetAttributesMetadata {
	metadata := &providers.GetAttributesMetadata{
		RuntimeMetadata: buildRuntimeMetadata(ctx),
	}

	if locale, exists := ctx.RuntimeData[common.RuntimeKeyRequiredLocales]; exists && locale != "" {
		metadata.Locale = locale
	}

	return metadata
}

// buildRuntimeMetadata collects provider_ext_* runtime keys and flattens initiator request
// headers/query params into the metadata map.
func buildRuntimeMetadata(ctx *providers.NodeContext) map[string][]string {
	runtimeMetadata := make(map[string][]string)

	if ctx.RuntimeData != nil {
		for key, value := range ctx.RuntimeData {
			if strings.HasPrefix(key, "provider_ext_") {
				runtimeMetadata[key] = []string{value}
			}
		}
	}

	if req := ctx.GetInitiatorRequest(); req != nil {
		// Header names are case-insensitive per RFC 7230, so lowercase the key to give consumers
		// a stable form. Since distinct-casing entries (e.g. "Foo" and "foo") normalize to the
		// same key, merge the value slices instead of overwriting so nothing is silently dropped.
		for name, values := range req.Headers {
			key := "initiator_header_" + strings.ToLower(name)
			runtimeMetadata[key] = append(runtimeMetadata[key], values...)
		}
		// Query parameter names are case-sensitive; preserve original casing verbatim.
		for name, values := range req.QueryParams {
			runtimeMetadata["initiator_query_"+name] = values
		}
	}

	return runtimeMetadata
}
