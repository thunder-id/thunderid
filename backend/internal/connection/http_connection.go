// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"slices"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

// httpConnectionRequest is the create/update payload for an HTTP endpoint connection. Credentials
// live in the same discriminated authentication object SMTP connections use.
type httpConnectionRequest struct {
	Name           string                       `json:"name"`
	Description    string                       `json:"description,omitempty"`
	URL            string                       `json:"url"`
	TimeoutMS      int                          `json:"timeoutMs,omitempty"`
	Headers        []httpconnection.Header      `json:"headers,omitempty"`
	Authentication *outboundauth.Authentication `json:"authentication,omitempty"`
}

// httpConnectionResponse is the detail payload for an HTTP endpoint connection (secret masked).
type httpConnectionResponse struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description,omitempty"`
	Type           string                      `json:"type"`
	URL            string                      `json:"url"`
	TimeoutMS      int                         `json:"timeoutMs"`
	Headers        []httpconnection.Header     `json:"headers"`
	Authentication outboundauth.Authentication `json:"authentication"`
}

// httpToDTO converts a request into an HTTP connection with its authentication in property form.
func httpToDTO(req httpConnectionRequest) (*httpconnection.HTTPConnection, error) {
	authConfig := req.Authentication.Config()
	if _, ok := outboundauth.ParseType(string(authConfig.Type)); !ok ||
		!slices.Contains(httpconnection.SupportedAuthTypes(), authConfig.Type) {
		return nil, errUnsupportedAuthenticationType
	}
	authProps, err := outboundauth.ToProperties(authConfig)
	if err != nil {
		return nil, err
	}
	return &httpconnection.HTTPConnection{
		Name:           req.Name,
		Description:    req.Description,
		URL:            req.URL,
		TimeoutMS:      req.TimeoutMS,
		Headers:        req.Headers,
		AuthProperties: authProps,
	}, nil
}

// httpFromDTO converts a stored HTTP connection into its response, with secrets masked.
func httpFromDTO(dto httpconnection.HTTPConnection) (httpConnectionResponse, error) {
	values, err := propertyValues(dto.AuthProperties)
	if err != nil {
		return httpConnectionResponse{}, err
	}
	headers := dto.Headers
	if headers == nil {
		headers = []httpconnection.Header{}
	}
	return httpConnectionResponse{
		ID:             dto.ID,
		Name:           dto.Name,
		Description:    dto.Description,
		Type:           httpVendorName,
		URL:            dto.URL,
		TimeoutMS:      dto.TimeoutMS,
		Headers:        headers,
		Authentication: outboundauth.AuthenticationFromValues(values),
	}, nil
}
