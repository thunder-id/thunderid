// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection //nolint:dupl // sms-gateway mirrors twilio/vonage's shape, kept distinct per vendor

import (
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
	Name           string                        `json:"name"`
	Description    string                        `json:"description,omitempty"`
	URL            string                        `json:"url"`
	HTTPMethod     string                        `json:"httpMethod,omitempty"`
	Authentication *outboundauthn.Authentication `json:"authentication,omitempty"`
	ContentType    string                        `json:"contentType,omitempty"`
}

// smsGatewayConnectionResponse is the detail payload for an SMS gateway connection.
type smsGatewayConnectionResponse struct {
	ID             string                       `json:"id"`
	Name           string                       `json:"name"`
	Description    string                       `json:"description,omitempty"`
	Type           string                       `json:"type"`
	URL            string                       `json:"url,omitempty"`
	HTTPMethod     string                       `json:"httpMethod,omitempty"`
	Authentication outboundauthn.Authentication `json:"authentication"`
	ContentType    string                       `json:"contentType,omitempty"`
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
	headers, err := smsGatewayAPIKeyHeadersFromAuthentication(req.Authentication, allowMaskedValues)
	if err != nil {
		return nil, err
	}
	if props, err = appendSMSGatewayAPIKeyHeaders(props, headers, allowMaskedValues); err != nil {
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
		ID:             dto.ID,
		Name:           dto.Name,
		Description:    dto.Description,
		Type:           smsGatewayVendorName,
		URL:            values[ncommon.CustomPropKeyURL],
		HTTPMethod:     values[ncommon.CustomPropKeyHTTPMethod],
		Authentication: smsGatewayAuthenticationResponse(headers),
		ContentType:    values[ncommon.CustomPropKeyContentType],
	}, nil
}

func smsGatewayAPIKeyHeadersFromAuthentication(
	authentication *outboundauthn.Authentication, allowMaskedValues bool,
) ([]outboundauthn.APIKeyHeader, error) {
	if authentication == nil {
		return nil, nil
	}
	if allowMaskedValues && authentication.Scheme == outboundauthn.SchemeAPIKey && authentication.Bearer == nil &&
		authentication.Basic == nil && authentication.APIKey != nil {
		return authentication.APIKey.Headers, nil
	}
	config, err := outboundauthn.ConfigFromAuthentication(authentication)
	if err != nil {
		return nil, err
	}
	if config.Scheme == outboundauthn.SchemeNone {
		return []outboundauthn.APIKeyHeader{}, nil
	}
	if config.Scheme != outboundauthn.SchemeAPIKey {
		return nil, fmt.Errorf("SMS gateway only supports API key authentication")
	}
	return authentication.APIKey.Headers, nil
}

func smsGatewayAuthenticationResponse(headers []outboundauthn.APIKeyHeader) outboundauthn.Authentication {
	if len(headers) == 0 {
		return outboundauthn.Authentication{Scheme: outboundauthn.SchemeNone}
	}
	return outboundauthn.Authentication{
		Scheme: outboundauthn.SchemeAPIKey,
		APIKey: &outboundauthn.APIKeyCredentials{Headers: headers},
	}
}

func appendSMSGatewayAPIKeyHeaders(
	properties []cmodels.Property, headers []outboundauthn.APIKeyHeader, allowMaskedValues bool,
) ([]cmodels.Property, error) {
	if headers == nil {
		return properties, nil
	}
	if allowMaskedValues {
		property, err := outboundauthn.NewAPIKeyHeadersPropertyForUpdate(ncommon.CustomPropKeyAPIKeyHeaders, headers)
		if err != nil {
			return nil, err
		}
		return append(properties, *property), nil
	}
	property, err := outboundauthn.NewAPIKeyHeadersProperty(ncommon.CustomPropKeyAPIKeyHeaders, headers)
	if err != nil {
		return nil, err
	}
	return append(properties, *property), nil
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
	mergedHeaders, err := outboundauthn.MergeAPIKeyHeaders(incomingHeaders, existingHeaders)
	if err != nil {
		return nil, err
	}
	properties := removeProperty(incoming, ncommon.CustomPropKeyAPIKeyHeaders)
	return appendSMSGatewayAPIKeyHeaders(properties, mergedHeaders, false)
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
		return outboundauthn.APIKeyHeadersFromProperty(property, maskValues)
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
