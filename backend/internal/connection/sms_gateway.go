// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection //nolint:dupl // SMS gateway follows other sender connection shapes

import (
	"fmt"
	"net/http"
	"net/url"
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
	OUID           string                       `json:"ouId,omitempty"`
	URL            string                       `json:"url"`
	HTTPMethod     string                       `json:"httpMethod,omitempty"`
	HTTPHeaders    *string                      `json:"httpHeaders,omitempty"`
	Authentication *outboundauth.Authentication `json:"authentication,omitempty"`
	ContentType    string                       `json:"contentType,omitempty"`
}

// smsGatewayConnectionResponse is the detail payload for a custom HTTP SMS gateway.
type smsGatewayConnectionResponse struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description,omitempty"`
	OUID           string                      `json:"ouId"`
	Type           string                      `json:"type"`
	URL            string                      `json:"url,omitempty"`
	HTTPMethod     string                      `json:"httpMethod,omitempty"`
	HTTPHeaders    string                      `json:"httpHeaders,omitempty"`
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
	if req.HTTPHeaders != nil {
		property, propertyErr := cmodels.NewProperty(ncommon.CustomPropKeyHTTPHeaders, *req.HTTPHeaders, false)
		if propertyErr != nil {
			return nil, propertyErr
		}
		props = append(props, *property)
	}
	if req.Authentication != nil {
		config := req.Authentication.Config()
		if allowMasked {
			config, err = httpauth.CanonicalizeForUpdate(config)
		} else {
			config, err = httpauth.Canonicalize(config)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidSMSGatewayAuthentication, err)
		}
		if err := validateSMSGatewayAuthenticationURL(req.URL, config); err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidSMSGatewayAuthentication, err)
		}
		if req.HTTPHeaders != nil {
			names := smsGatewayHeaderNames(*req.HTTPHeaders)
			for name := range smsGatewayAuthenticationHeaderNames(config) {
				if names[strings.ToLower(name)] {
					return nil, fmt.Errorf("%w: HTTP header %q is also used for authentication",
						errInvalidSMSGatewayAuthentication, name)
				}
			}
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
		Name: req.Name, Description: req.Description, OUID: req.OUID, Type: ncommon.NotificationSenderTypeMessage,
		Provider: ncommon.NotificationProviderTypeCustom, Properties: props,
	}, nil
}

func smsGatewayFromSenderDTO(dto ncommon.NotificationSenderDTO) (smsGatewayConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return smsGatewayConnectionResponse{}, err
	}
	auth, err := outboundauth.AuthenticationFromProperties(dto.Properties)
	if err != nil {
		return smsGatewayConnectionResponse{}, err
	}
	headers, err := smsGatewayHeaderValue(dto.Properties)
	if err != nil {
		return smsGatewayConnectionResponse{}, err
	}
	return smsGatewayConnectionResponse{
		ID: dto.ID, Name: dto.Name, Description: dto.Description, OUID: dto.OUID, Type: smsGatewayVendorName,
		URL: values[ncommon.CustomPropKeyURL], HTTPMethod: values[ncommon.CustomPropKeyHTTPMethod],
		HTTPHeaders: headers, Authentication: auth, ContentType: values[ncommon.CustomPropKeyContentType],
	}, nil
}

func smsGatewayHeaderValue(props []cmodels.Property) (string, error) {
	for _, prop := range props {
		if prop.GetName() != ncommon.CustomPropKeyHTTPHeaders {
			continue
		}
		return prop.GetValue()
	}
	return "", nil
}

func smsGatewayHeaderNames(value string) map[string]bool {
	names := make(map[string]bool)
	for _, pair := range strings.Split(value, ",") {
		name, _, found := strings.Cut(pair, ":")
		if !found {
			continue
		}
		names[strings.ToLower(strings.TrimSpace(name))] = true
	}
	return names
}

func smsGatewayAuthenticationHeaderNames(config outboundauth.Config) map[string]bool {
	if config.Type == outboundauth.TypeBearer || config.Type == outboundauth.TypeBasic {
		return map[string]bool{"authorization": true}
	}
	if config.Type != outboundauth.TypeAPIKey {
		return nil
	}
	names := make(map[string]bool, len(config.Properties))
	for name := range config.Properties {
		names[strings.ToLower(name)] = true
	}
	return names
}

func validateSMSGatewayAuthenticationURL(rawURL string, config outboundauth.Config) error {
	if !config.Enabled() {
		return nil
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	return httpauth.ValidateCredentialTransport(&http.Request{URL: parsedURL})
}

func mergeSMSGatewayProperties(incoming, existing []cmodels.Property) ([]cmodels.Property, error) {
	incomingValues, err := propertyValues(incoming)
	if err != nil {
		return nil, err
	}
	existingValues, err := propertyValues(existing)
	if err != nil {
		return nil, err
	}
	storedAuth := make([]cmodels.Property, 0)
	for _, property := range existing {
		if outboundauth.OwnsPropertyKey(property.GetName()) {
			storedAuth = append(storedAuth, property)
		}
	}
	var request *outboundauth.Authentication
	if _, present := incomingValues[outboundauth.PropertyKeyType]; present {
		config, err := outboundauth.FromProperties(incoming)
		if err != nil {
			return nil, err
		}
		request = &outboundauth.Authentication{Type: string(config.Type), Properties: config.Properties}
	}
	authProps, err := outboundauth.UpdateProperties(request, storedAuth,
		[]string{existingValues[ncommon.CustomPropKeyURL]}, []string{incomingValues[ncommon.CustomPropKeyURL]},
		httpauth.SupportedTypes(),
		httpauth.CanonicalizeForUpdate, httpauth.Canonicalize)
	if err != nil {
		return nil, err
	}
	mergedConfig, err := outboundauth.FromProperties(authProps)
	if err != nil {
		return nil, err
	}
	if err := validateSMSGatewayAuthenticationURL(incomingValues[ncommon.CustomPropKeyURL], mergedConfig); err != nil {
		return nil, err
	}
	result := make([]cmodels.Property, 0, len(incoming)+len(authProps))
	for _, property := range incoming {
		if !outboundauth.OwnsPropertyKey(property.GetName()) &&
			property.GetName() != ncommon.CustomPropKeyHTTPHeaders {
			result = append(result, property)
		}
	}
	result = append(result, authProps...)
	headerProperty, err := mergeSMSGatewayHeaders(incoming, existing, authProps)
	if err != nil {
		return nil, err
	}
	if headerProperty != nil {
		result = append(result, *headerProperty)
	}
	return result, nil
}

func mergeSMSGatewayHeaders(incoming, existing, authProps []cmodels.Property) (*cmodels.Property, error) {
	previousHeaders, err := smsGatewayHeaderValue(existing)
	if err != nil {
		return nil, err
	}
	incomingHeaders := previousHeaders
	for _, property := range incoming {
		if property.GetName() == ncommon.CustomPropKeyHTTPHeaders {
			incomingHeaders, err = property.GetValue()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if incomingHeaders != "" {
		mergedAuth, err := outboundauth.FromProperties(authProps)
		if err != nil {
			return nil, err
		}
		names := smsGatewayHeaderNames(incomingHeaders)
		for name := range smsGatewayAuthenticationHeaderNames(mergedAuth) {
			if names[name] {
				return nil, fmt.Errorf("HTTP header %q is also used for authentication", name)
			}
		}
		property, err := cmodels.NewProperty(ncommon.CustomPropKeyHTTPHeaders, incomingHeaders, false)
		if err != nil {
			return nil, err
		}
		return property, nil
	}
	return nil, nil
}
