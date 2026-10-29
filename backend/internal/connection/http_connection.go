// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
)

// httpConnectionRequest is the create/update payload for an HTTP endpoint connection. Credentials
// live in the same discriminated authentication object SMTP connections use.
type httpConnectionRequest struct {
	Name           string                       `json:"name"`
	Description    string                       `json:"description,omitempty"`
	OUID           string                       `json:"ouId,omitempty"`
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
	OUID           string                      `json:"ouId"`
	Type           string                      `json:"type"`
	URL            string                      `json:"url"`
	TimeoutMS      int                         `json:"timeoutMs"`
	Headers        []httpconnection.Header     `json:"headers"`
	Authentication outboundauth.Authentication `json:"authentication"`
}

// httpToDTO converts a create request into an HTTP connection with its authentication in property
// form. Every secret must be given: there is no stored one to keep.
func httpToDTO(req httpConnectionRequest) (*httpconnection.HTTPConnection, error) {
	return httpRequestToDTO(req, false)
}

// httpUpdateToDTO converts an update request. A secret may be omitted, and an API-key header value
// may be sent as the mask, for the update to resolve against the stored connection.
func httpUpdateToDTO(req httpConnectionRequest) (*httpconnection.HTTPConnection, error) {
	return httpRequestToDTO(req, true)
}

func httpRequestToDTO(req httpConnectionRequest, update bool) (*httpconnection.HTTPConnection, error) {
	authConfig := req.Authentication.Config()
	if _, ok := outboundauth.ParseType(string(authConfig.Type)); !ok ||
		!slices.Contains(httpauth.SupportedTypes(), authConfig.Type) {
		return nil, errUnsupportedAuthenticationType
	}
	var err error
	if update {
		err = refuseMaskedCredentialField(authConfig)
	} else {
		authConfig, err = httpauth.Canonicalize(authConfig)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidHTTPAuthentication, err)
	}
	authProps, err := outboundauth.ToProperties(authConfig)
	if err != nil {
		return nil, err
	}
	return &httpconnection.HTTPConnection{
		Name:           req.Name,
		Description:    req.Description,
		OUID:           req.OUID,
		URL:            req.URL,
		TimeoutMS:      req.TimeoutMS,
		Headers:        req.Headers,
		AuthProperties: authProps,
	}, nil
}

// refuseMaskedCredentialField refuses the mask as the value of a method's credential field, the basic
// password or the bearer token. The shared update rule would read it as "keep the stored secret",
// but an HTTP connection keeps a secret only when it is omitted, so a read sent back unchanged is
// refused rather than taken as consent to reuse. API-key header values are not fields and follow
// the shared rule.
func refuseMaskedCredentialField(authConfig outboundauth.Config) error {
	method, _ := outboundauth.GetMethod(authConfig.Type)
	for _, field := range method.Fields {
		if field.Credential && authConfig.Get(field.Key) == maskedSecretValue {
			return fmt.Errorf("masked %s cannot be stored", field.Key)
		}
	}
	return nil
}

// httpFromDTO converts a stored HTTP connection into its response, with secrets masked.
func httpFromDTO(dto httpconnection.HTTPConnection) (httpConnectionResponse, error) {
	authentication, err := outboundauth.AuthenticationFromProperties(dto.AuthProperties)
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
		OUID:           dto.OUID,
		Type:           httpVendorName,
		URL:            dto.URL,
		TimeoutMS:      dto.TimeoutMS,
		Headers:        headers,
		Authentication: authentication,
	}, nil
}
