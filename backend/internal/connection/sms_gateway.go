// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection //nolint:dupl // SMS gateway follows other sender connection shapes

import (
	"encoding/json"
	"fmt"
	"strings"

	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
)

// smsGatewayConnectionRequest is the create/update payload for a custom HTTP SMS gateway.
type smsGatewayConnectionRequest struct {
	Name           string                       `json:"name"`
	Description    string                       `json:"description,omitempty"`
	URL            string                       `json:"url"`
	HTTPMethod     string                       `json:"httpMethod,omitempty"`
	Authentication *outboundauth.Authentication `json:"authentication,omitempty"`
	ContentType    string                       `json:"contentType,omitempty"`
}

// smsGatewayConnectionResponse is the detail payload for a custom HTTP SMS gateway.
type smsGatewayConnectionResponse struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description,omitempty"`
	Type           string                      `json:"type"`
	URL            string                      `json:"url,omitempty"`
	HTTPMethod     string                      `json:"httpMethod,omitempty"`
	Authentication outboundauth.Authentication `json:"authentication"`
	ContentType    string                      `json:"contentType,omitempty"`
}

func smsGatewayToSenderDTO(req smsGatewayConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	return smsGatewayToSenderDTOWithMasked(req, false)
}

func smsGatewayUpdateToSenderDTO(req smsGatewayConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	return smsGatewayToSenderDTOWithMasked(req, true)
}

func smsGatewayToSenderDTOWithMasked(
	req smsGatewayConnectionRequest, allowMasked bool,
) (*ncommon.NotificationSenderDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, ncommon.CustomPropKeyURL, req.URL, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.CustomPropKeyHTTPMethod, req.HTTPMethod, false); err != nil {
		return nil, err
	}
	if req.Authentication != nil {
		config := req.Authentication.Config()
		if config.Type != outboundauth.TypeNone && config.Type != outboundauth.TypeAPIKey {
			return nil, fmt.Errorf("SMS gateway supports only API-key authentication")
		}
		if allowMasked {
			config, err = httpauth.CanonicalizeForUpdate(config)
		} else {
			config, err = httpauth.Canonicalize(config)
		}
		if err != nil {
			return nil, err
		}
		authProps, err := outboundauth.ToProperties(config)
		if err != nil {
			return nil, err
		}
		props = append(props, authProps...)
	}
	if props, err = appendProperty(props, ncommon.CustomPropKeyContentType, req.ContentType, false); err != nil {
		return nil, err
	}
	return &ncommon.NotificationSenderDTO{
		Name: req.Name, Description: req.Description, Type: ncommon.NotificationSenderTypeMessage,
		Provider: ncommon.NotificationProviderTypeCustom, Properties: props,
	}, nil
}

func smsGatewayFromSenderDTO(dto ncommon.NotificationSenderDTO) (smsGatewayConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return smsGatewayConnectionResponse{}, err
	}
	auth := outboundauth.AuthenticationFromValues(values)
	if auth.Type == string(outboundauth.TypeNone) && values[ncommon.CustomPropKeyAPIKeyHeaders] != "" {
		legacy, err := legacySMSGatewayHeaders(dto.Properties)
		if err != nil {
			return smsGatewayConnectionResponse{}, err
		}
		if len(legacy) > 0 {
			auth = outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey), Properties: legacy}
		}
	}
	return smsGatewayConnectionResponse{
		ID: dto.ID, Name: dto.Name, Description: dto.Description, Type: smsGatewayVendorName,
		URL: values[ncommon.CustomPropKeyURL], HTTPMethod: values[ncommon.CustomPropKeyHTTPMethod],
		Authentication: auth, ContentType: values[ncommon.CustomPropKeyContentType],
	}, nil
}

func legacySMSGatewayHeaders(props []cmodels.Property) (map[string]string, error) {
	values, err := legacySMSGatewayHeaderValues(props)
	if err != nil {
		return nil, err
	}
	for name := range values {
		values[name] = maskedSecretValue
	}
	return values, nil
}

func legacySMSGatewayHeaderValues(props []cmodels.Property) (map[string]string, error) {
	for _, prop := range props {
		if prop.GetName() != ncommon.CustomPropKeyAPIKeyHeaders {
			continue
		}
		value, err := prop.GetValue()
		if err != nil {
			return nil, err
		}
		result := make(map[string]string)
		if prop.IsSecret() {
			var headers []struct{ Name, Value string }
			if err := json.Unmarshal([]byte(value), &headers); err != nil {
				return nil, fmt.Errorf("invalid legacy HTTP header bundle: %w", err)
			}
			for _, header := range headers {
				canonical, err := httpauth.ValidateAPIKeyHeader(header.Name, header.Value)
				if err != nil {
					return nil, err
				}
				result[canonical] = header.Value
			}
		} else {
			for _, pair := range strings.Split(value, ",") {
				name, headerValue, found := strings.Cut(pair, ":")
				if !found {
					return nil, fmt.Errorf("invalid legacy HTTP header format")
				}
				canonical, err := httpauth.ValidateAPIKeyHeader(name, headerValue)
				if err != nil {
					return nil, err
				}
				result[canonical] = strings.TrimSpace(headerValue)
			}
		}
		return result, nil
	}
	return nil, nil
}

func mergeSMSGatewayAuthentication(incoming, existing []cmodels.Property) ([]cmodels.Property, error) {
	incomingValues, err := propertyValues(incoming)
	if err != nil {
		return nil, err
	}
	existingValues, err := propertyValues(existing)
	if err != nil {
		return nil, err
	}
	sameURL := incomingValues[ncommon.CustomPropKeyURL] == existingValues[ncommon.CustomPropKeyURL]
	requestedType, hasRequestedType := incomingValues["authentication_type"]
	if !hasRequestedType {
		if sameURL {
			for _, property := range existing {
				if outboundauth.OwnsPropertyKey(property.GetName()) ||
					property.GetName() == ncommon.CustomPropKeyAPIKeyHeaders {
					incoming = append(incoming, property)
				}
			}
		}
		return incoming, nil
	}
	if requestedType != string(outboundauth.TypeAPIKey) {
		return incoming, nil
	}

	storedType := existingValues["authentication_type"]
	legacyHeaders, err := legacySMSGatewayHeaderValues(existing)
	if err != nil {
		return nil, err
	}
	canRetain := sameURL && (storedType == requestedType || storedType == "" && len(legacyHeaders) > 0)
	storedProperties := make(map[string]cmodels.Property, len(existing))
	for _, property := range existing {
		storedProperties[property.GetName()] = property
	}
	for i, property := range incoming {
		if !property.IsSecret() || !outboundauth.OwnsPropertyKey(property.GetName()) {
			continue
		}
		value, err := property.GetValue()
		if err != nil {
			return nil, err
		}
		if value != maskedSecretValue {
			continue
		}
		if !canRetain {
			return nil, fmt.Errorf("cannot retain an API-key header for a changed gateway or authentication type")
		}
		if stored, found := storedProperties[property.GetName()]; found {
			incoming[i] = stored
			continue
		}
		name := strings.TrimPrefix(property.GetName(), "authentication_")
		storedValue, found := legacyHeaders[name]
		if !found {
			return nil, fmt.Errorf("cannot retain unknown API-key header %q", name)
		}
		replacement, err := cmodels.NewProperty(property.GetName(), storedValue, true)
		if err != nil {
			return nil, err
		}
		incoming[i] = *replacement
	}
	return incoming, nil
}
