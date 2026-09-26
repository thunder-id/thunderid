// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// maskedSecretValue is the placeholder used for secret property values on read. A value of
// this on update means "keep the stored secret".
const maskedSecretValue = "******"

// appendProperty appends a property built from the given field, skipping empty values
// (the IdP service rejects properties with empty values). Secret values are encrypted by
// cmodels.NewProperty.
func appendProperty(props []cmodels.Property, name, value string, isSecret bool) ([]cmodels.Property, error) {
	if strings.TrimSpace(value) == "" {
		return props, nil
	}
	property, err := cmodels.NewProperty(name, value, isSecret)
	if err != nil {
		return nil, err
	}
	return append(props, *property), nil
}

// propertyValues returns a name→value map for the given properties. Secret values are
// replaced with the mask placeholder; non-secret values are returned in plain text.
func propertyValues(props []cmodels.Property) (map[string]string, error) {
	values := make(map[string]string, len(props))
	for i := range props {
		property := props[i]
		if property.IsSecret() {
			values[property.GetName()] = maskedSecretValue
			continue
		}
		value, err := property.GetValue()
		if err != nil {
			return nil, err
		}
		values[property.GetName()] = value
	}
	return values, nil
}

// mergeStoredSecrets keeps secret values that the request omits: secret properties are
// optional on update, so any secret present in the stored connection but absent from the
// incoming request is carried over unchanged. A secret that IS present in the request is
// used verbatim (presence-based — the value is not inspected).
//
// An outbound-authentication secret is carried over only while the credential target is
// unchanged: the authentication type, every non-secret authentication property (such as the
// username), and every property named in credentialTargetKeys (such as the server host and
// port). Changing the method, or turning authentication off, must not leave the previous
// method's credential behind for a later switch back to silently reuse. Changing where or as
// whom the credential is presented must not either, or a caller allowed to edit a connection
// but not to read its secrets could redirect the stored credential to a server they control.
func mergeStoredSecrets(incoming, existing []cmodels.Property,
	credentialTargetKeys ...string) []cmodels.Property {
	incomingNames := make(map[string]bool, len(incoming))
	for i := range incoming {
		incomingNames[incoming[i].GetName()] = true
	}

	targetUnchanged := credentialTargetUnchanged(incoming, existing, credentialTargetKeys)

	merged := make([]cmodels.Property, 0, len(incoming)+len(existing))
	merged = append(merged, incoming...)
	for i := range existing {
		if !existing[i].IsSecret() || incomingNames[existing[i].GetName()] {
			continue
		}
		if outboundauth.OwnsPropertyKey(existing[i].GetName()) && !targetUnchanged {
			continue
		}
		merged = append(merged, existing[i])
	}
	return merged
}

// credentialTargetUnchanged reports whether two property bags agree on every property that
// identifies the target of an outbound-authentication credential: the non-secret authentication
// properties, including the type, and the given transport keys. An absent property compares
// equal only to an absent property. A value that cannot be read is treated as a change, so the
// stored credential is dropped rather than risked.
func credentialTargetUnchanged(incoming, existing []cmodels.Property, credentialTargetKeys []string) bool {
	incomingValues, ok := credentialTargetValues(incoming, credentialTargetKeys)
	if !ok {
		return false
	}
	existingValues, ok := credentialTargetValues(existing, credentialTargetKeys)
	if !ok {
		return false
	}
	return maps.Equal(incomingValues, existingValues)
}

// credentialTargetValues collects the values credentialTargetUnchanged compares. None of them is
// secret, so they are readable without decryption.
func credentialTargetValues(props []cmodels.Property, credentialTargetKeys []string) (map[string]string, bool) {
	values := make(map[string]string, len(credentialTargetKeys)+2)
	for i := range props {
		name := props[i].GetName()
		if props[i].IsSecret() ||
			(!outboundauth.OwnsPropertyKey(name) && !slices.Contains(credentialTargetKeys, name)) {
			continue
		}
		value, err := props[i].GetValue()
		if err != nil {
			return nil, false
		}
		values[name] = value
	}
	return values, true
}

// connectionTypeName returns the lowercase connection-type identifier (e.g. "google") that
// matches the /connections/{type} path, derived from the underlying IdP type.
func connectionTypeName(idpType providers.IDPType) string {
	return strings.ToLower(string(idpType))
}

// joinScopes serializes scopes to the comma-separated form stored in the IdP property.
func joinScopes(scopes []string) string {
	return strings.Join(scopes, ",")
}

// splitScopes parses the comma-separated scopes property into a slice, trimming whitespace
// and dropping empty entries (guards against externally-seeded values like "openid, email").
func splitScopes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			scopes = append(scopes, trimmed)
		}
	}
	return scopes
}

// writeServiceError maps a service error from the underlying service to an HTTP response.
func writeServiceError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	status := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case idp.ErrorIDPNotFound.Code, notification.ErrorSenderNotFound.Code, authzenpdp.ErrorNotFound.Code:
			status = http.StatusNotFound
		case idp.ErrorIDPAlreadyExists.Code,
			idp.ErrorIDPHasBlockingDependencies.Code,
			notification.ErrorDuplicateSenderName.Code,
			notification.ErrorSenderHasBlockingDependencies.Code,
			authzenpdp.ErrorHasBlockingDependencies.Code,
			authzenpdp.ErrorAlreadyExists.Code:
			status = http.StatusConflict
		default:
			status = http.StatusBadRequest
		}
	}
	sysutils.WriteErrorResponse(ctx, w, status, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}

// writeInvalidBody writes a 400 response for a malformed request body.
func writeInvalidBody(ctx context.Context, w http.ResponseWriter) {
	sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, apierror.ErrorResponse{
		Code:        idp.ErrorInvalidRequestFormat.Code,
		Message:     idp.ErrorInvalidRequestFormat.Error,
		Description: idp.ErrorInvalidRequestFormat.ErrorDescription,
	})
}
