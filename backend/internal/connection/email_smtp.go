// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"strconv"

	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// emailSMTPConnectionRequest is the create/update payload for an SMTP email connection. Port and TLS
// mode are typed here even though the sender store holds every property as a string: the
// conversion belongs at this boundary, not in the public contract. Credentials are the
// exception, and deliberately so: they live in a discriminated authentication object whose
// properties vary per method, which is what lets a new method ship without a schema change.
type emailSMTPConnectionRequest struct {
	Name           string                    `json:"name"`
	Description    string                    `json:"description,omitempty"`
	Host           string                    `json:"host"`
	Port           int                       `json:"port"`
	FromAddress    string                    `json:"fromAddress"`
	FromName       string                    `json:"fromName,omitempty"`
	TLS            string                    `json:"tls,omitempty"`
	Authentication *connectionAuthentication `json:"authentication,omitempty"`
}

// emailSMTPConnectionResponse is the detail payload for an SMTP email connection (secret masked).
// Authentication is always present so a console select always has a concrete value to bind to.
type emailSMTPConnectionResponse struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description,omitempty"`
	Type           string                   `json:"type"`
	Host           string                   `json:"host,omitempty"`
	Port           int                      `json:"port,omitempty"`
	FromAddress    string                   `json:"fromAddress,omitempty"`
	FromName       string                   `json:"fromName,omitempty"`
	TLS            string                   `json:"tls,omitempty"`
	Authentication connectionAuthentication `json:"authentication"`
}

func emailSMTPToSenderDTO(req emailSMTPConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	// An omitted tls value resolves to the secure default rather than plaintext. An invalid
	// one is stored as given so the sender service rejects it with a descriptive error.
	tlsMode := req.TLS
	if mode, ok := ncommon.ParseTLSMode(req.TLS); ok {
		tlsMode = string(mode)
	}

	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, ncommon.SMTPPropKeyHost, req.Host, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyPort, portValue(req.Port), false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyFromAddress, req.FromAddress, false); err != nil {
		return nil, err
	}
	// Optional: appendProperty skips a blank value, so clearing the name on update drops the
	// property rather than storing an empty display name.
	if props, err = appendProperty(props, ncommon.SMTPPropKeyFromName, req.FromName, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyTLS, tlsMode, false); err != nil {
		return nil, err
	}
	authProps, err := authenticationProperties(req.Authentication)
	if err != nil {
		return nil, err
	}
	props = append(props, authProps...)

	return &ncommon.NotificationSenderDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        ncommon.NotificationSenderTypeEmail,
		Provider:    ncommon.NotificationProviderTypeSMTP,
		Properties:  props,
	}, nil
}

func emailSMTPFromSenderDTO(dto ncommon.NotificationSenderDTO) (emailSMTPConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return emailSMTPConnectionResponse{}, err
	}
	port, _ := strconv.Atoi(values[ncommon.SMTPPropKeyPort])
	return emailSMTPConnectionResponse{
		ID:             dto.ID,
		Name:           dto.Name,
		Description:    dto.Description,
		Type:           emailSMTPVendorName,
		Host:           values[ncommon.SMTPPropKeyHost],
		Port:           port,
		FromAddress:    values[ncommon.SMTPPropKeyFromAddress],
		FromName:       values[ncommon.SMTPPropKeyFromName],
		TLS:            values[ncommon.SMTPPropKeyTLS],
		Authentication: authenticationFromValues(values),
	}, nil
}

// portValue renders a port for storage. Zero means "not supplied", which appendProperty skips
// so the sender service reports the property as missing rather than as port 0.
func portValue(port int) string {
	if port == 0 {
		return ""
	}
	return strconv.Itoa(port)
}
