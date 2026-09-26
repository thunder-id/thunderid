// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection //nolint:dupl // sms-gateway mirrors twilio/vonage's shape, kept distinct per vendor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauthn"
)

// smsGatewayConnectionRequest is the create/update payload for a generic HTTP SMS gateway
// connection (a custom webhook, unlike the Twilio/Vonage vendor-specific integrations).
type smsGatewayConnectionRequest struct {
	Name          string                       `json:"name"`
	Description   string                       `json:"description,omitempty"`
	URL           string                       `json:"url"`
	HTTPMethod    string                       `json:"httpMethod,omitempty"`
	APIKeyHeaders []outboundauthn.APIKeyHeader `json:"apiKeyHeaders,omitempty"`
	ContentType   string                       `json:"contentType,omitempty"`
}

// smsGatewayConnectionResponse is the detail payload for an SMS gateway connection.
type smsGatewayConnectionResponse struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	Description   string                       `json:"description,omitempty"`
	Type          string                       `json:"type"`
	URL           string                       `json:"url,omitempty"`
	HTTPMethod    string                       `json:"httpMethod,omitempty"`
	APIKeyHeaders []outboundauthn.APIKeyHeader `json:"apiKeyHeaders,omitempty"`
	ContentType   string                       `json:"contentType,omitempty"`
}

func smsGatewayToSenderDTO(req smsGatewayConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	return smsGatewayToSenderDTOWithMaskedValues(req, false)
}

func smsGatewayUpdateToSenderDTO(req smsGatewayConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	return smsGatewayToSenderDTOWithMaskedValues(req, true)
}

func smsGatewayToSenderDTOWithMaskedValues(
	req smsGatewayConnectionRequest, allowMaskedValues bool,
) (*ncommon.NotificationSenderDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, ncommon.CustomPropKeyURL, req.URL, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.CustomPropKeyHTTPMethod, req.HTTPMethod, false); err != nil {
		return nil, err
	}
	if props, err = appendSMSGatewayAPIKeyHeaders(props, req.APIKeyHeaders, allowMaskedValues); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.CustomPropKeyContentType, req.ContentType, false); err != nil {
		return nil, err
	}
	return &ncommon.NotificationSenderDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        ncommon.NotificationSenderTypeMessage,
		Provider:    ncommon.NotificationProviderTypeCustom,
		Properties:  props,
	}, nil
}

func smsGatewayFromSenderDTO(dto ncommon.NotificationSenderDTO) (smsGatewayConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return smsGatewayConnectionResponse{}, err
	}
	headers, err := smsGatewayAPIKeyHeaders(dto.Properties, true)
	if err != nil {
		return smsGatewayConnectionResponse{}, err
	}
	return smsGatewayConnectionResponse{
		ID:            dto.ID,
		Name:          dto.Name,
		Description:   dto.Description,
		Type:          smsGatewayVendorName,
		URL:           values[ncommon.CustomPropKeyURL],
		HTTPMethod:    values[ncommon.CustomPropKeyHTTPMethod],
		APIKeyHeaders: headers,
		ContentType:   values[ncommon.CustomPropKeyContentType],
	}, nil
}

func appendSMSGatewayAPIKeyHeaders(
	properties []cmodels.Property, headers []outboundauthn.APIKeyHeader, allowMaskedValues bool,
) ([]cmodels.Property, error) {
	if headers == nil {
		return properties, nil
	}
	seen := make(map[string]struct{}, len(headers))
	for index := range headers {
		header := &headers[index]
		if header.Value == maskedSecretValue && allowMaskedValues {
			name, err := outboundauthn.ValidateAPIKeyHeader(header.Name, header.Value)
			if err != nil {
				return nil, err
			}
			if _, exists := seen[name]; exists {
				return nil, fmt.Errorf("duplicate API key header %q", name)
			}
			seen[name] = struct{}{}
			header.Name = name
			continue
		}
		if header.Value == maskedSecretValue {
			return nil, fmt.Errorf("masked API key header values are only allowed on update")
		}
		name, err := outboundauthn.ValidateAPIKeyHeader(header.Name, header.Value)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate API key header %q", name)
		}
		seen[name] = struct{}{}
		header.Name = name
	}
	value, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to encode API key headers: %w", err)
	}
	return appendProperty(properties, ncommon.CustomPropKeyAPIKeyHeaders, string(value), true)
}

func mergeStoredSMSGatewayAPIKeyHeaders(
	incoming, existing []cmodels.Property,
) ([]cmodels.Property, error) {
	if !hasProperty(incoming, ncommon.CustomPropKeyAPIKeyHeaders) {
		existingHeaders, err := smsGatewayAPIKeyHeaders(existing, false)
		if err != nil || len(existingHeaders) == 0 {
			return incoming, err
		}
		return appendSMSGatewayAPIKeyHeaders(incoming, existingHeaders, false)
	}
	incomingHeaders, err := smsGatewayAPIKeyHeaders(incoming, false)
	if err != nil {
		return nil, err
	}
	existingHeaders, err := smsGatewayAPIKeyHeaders(existing, false)
	if err != nil {
		return nil, err
	}
	existingValues := make(map[string]string, len(existingHeaders))
	for _, header := range existingHeaders {
		existingValues[http.CanonicalHeaderKey(header.Name)] = header.Value
	}
	for index := range incomingHeaders {
		if incomingHeaders[index].Value != maskedSecretValue {
			continue
		}
		value, found := existingValues[http.CanonicalHeaderKey(incomingHeaders[index].Name)]
		if !found {
			return nil, fmt.Errorf("stored API key header %q was not found", incomingHeaders[index].Name)
		}
		incomingHeaders[index].Value = value
	}
	properties := removeProperty(incoming, ncommon.CustomPropKeyAPIKeyHeaders)
	return appendSMSGatewayAPIKeyHeaders(properties, incomingHeaders, false)
}

func hasProperty(properties []cmodels.Property, name string) bool {
	for _, property := range properties {
		if property.GetName() == name {
			return true
		}
	}
	return false
}

func removeProperty(properties []cmodels.Property, name string) []cmodels.Property {
	result := make([]cmodels.Property, 0, len(properties))
	for _, property := range properties {
		if property.GetName() != name {
			result = append(result, property)
		}
	}
	return result
}

func smsGatewayAPIKeyHeaders(
	properties []cmodels.Property, maskValues bool,
) ([]outboundauthn.APIKeyHeader, error) {
	for _, property := range properties {
		if property.GetName() != ncommon.CustomPropKeyAPIKeyHeaders {
			continue
		}
		value, err := property.GetValue()
		if err != nil {
			return nil, err
		}
		if !property.IsSecret() {
			headers, parseErr := parseLegacySMSGatewayAPIKeyHeaders(value)
			if parseErr != nil {
				return nil, parseErr
			}
			if maskValues {
				for index := range headers {
					headers[index].Value = maskedSecretValue
				}
			}
			return headers, nil
		}
		var headers []outboundauthn.APIKeyHeader
		if err = json.Unmarshal([]byte(value), &headers); err != nil {
			return nil, fmt.Errorf("failed to decode API key headers: %w", err)
		}
		for i := range headers {
			headers[i].Name = http.CanonicalHeaderKey(headers[i].Name)
		}
		if maskValues {
			for i := range headers {
				headers[i].Value = maskedSecretValue
			}
		}
		return headers, nil
	}
	return nil, nil
}

func parseLegacySMSGatewayAPIKeyHeaders(value string) ([]outboundauthn.APIKeyHeader, error) {
	segments := strings.Split(value, ",")
	headers := make([]outboundauthn.APIKeyHeader, 0, len(segments))
	for _, segment := range segments {
		parts := strings.SplitN(segment, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid legacy HTTP header format")
		}
		name := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if name == "" || value == "" {
			return nil, fmt.Errorf("invalid legacy HTTP header format")
		}
		headers = append(headers, outboundauthn.APIKeyHeader{Name: name, Value: value})
	}
	return headers, nil
}
