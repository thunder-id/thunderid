// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"fmt"
	"slices"
	"strconv"

	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/smtpauth"
)

// emailSMTPConnectionRequest is the create/update payload for an SMTP email connection. Port and TLS
// mode are typed here even though the sender store holds every property as a string: the
// conversion belongs at this boundary, not in the public contract. Credentials are the
// exception, and deliberately so: they live in a discriminated authentication object whose
// properties vary per method, which is what lets a new method ship without a schema change.
type emailSMTPConnectionRequest struct {
	Name           string                       `json:"name"`
	Description    string                       `json:"description,omitempty"`
	OUID           string                       `json:"ouId,omitempty"`
	Host           string                       `json:"host"`
	Port           int                          `json:"port"`
	FromAddress    string                       `json:"fromAddress"`
	FromName       string                       `json:"fromName,omitempty"`
	TLS            string                       `json:"tls,omitempty"`
	Authentication *outboundauth.Authentication `json:"authentication,omitempty"`
}

// emailSMTPConnectionResponse is the detail payload for an SMTP email connection (secret masked).
// Authentication is always present so a console select always has a concrete value to bind to.
type emailSMTPConnectionResponse struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description,omitempty"`
	OUID           string                      `json:"ouId"`
	Type           string                      `json:"type"`
	Host           string                      `json:"host,omitempty"`
	Port           int                         `json:"port,omitempty"`
	FromAddress    string                      `json:"fromAddress,omitempty"`
	FromName       string                      `json:"fromName,omitempty"`
	TLS            string                      `json:"tls,omitempty"`
	Authentication outboundauth.Authentication `json:"authentication"`
}

func emailSMTPToSenderDTO(req emailSMTPConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	return emailSMTPToSenderDTOWithAuthentication(req, true)
}

// emailSMTPUpdateToSenderDTO omits authentication properties when the update does not specify them.
func emailSMTPUpdateToSenderDTO(req emailSMTPConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	return emailSMTPToSenderDTOWithAuthentication(req, req.Authentication != nil)
}

// emailSMTPToSenderDTOWithAuthentication converts an SMTP request into sender properties,
// including authentication only when requested by the create or update path.
func emailSMTPToSenderDTOWithAuthentication(req emailSMTPConnectionRequest,
	includeAuthentication bool) (*ncommon.NotificationSenderDTO, error) {
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
	if includeAuthentication {
		authConfig := req.Authentication.Config()
		if _, ok := outboundauth.ParseType(string(authConfig.Type)); !ok ||
			!slices.Contains(smtpauth.SupportedTypes(), authConfig.Type) {
			return nil, errUnsupportedAuthenticationType
		}
		authProps, err := outboundauth.ToProperties(authConfig)
		if err != nil {
			return nil, err
		}
		props = append(props, authProps...)
	}

	return &ncommon.NotificationSenderDTO{
		Name:        req.Name,
		Description: req.Description,
		OUID:        req.OUID,
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
	port, err := strconv.Atoi(values[ncommon.SMTPPropKeyPort])
	if err != nil {
		return emailSMTPConnectionResponse{}, fmt.Errorf("invalid stored port: %q", values[ncommon.SMTPPropKeyPort])
	}
	authentication, err := outboundauth.AuthenticationFromProperties(dto.Properties)
	if err != nil {
		return emailSMTPConnectionResponse{}, err
	}
	return emailSMTPConnectionResponse{
		ID:             dto.ID,
		Name:           dto.Name,
		Description:    dto.Description,
		OUID:           dto.OUID,
		Type:           emailSMTPVendorName,
		Host:           values[ncommon.SMTPPropKeyHost],
		Port:           port,
		FromAddress:    values[ncommon.SMTPPropKeyFromAddress],
		FromName:       values[ncommon.SMTPPropKeyFromName],
		TLS:            values[ncommon.SMTPPropKeyTLS],
		Authentication: authentication,
	}, nil
}

// mergeSMTPAuthentication applies the shared credential update rules, retaining stored
// credentials only when the SMTP host and port are unchanged.
func mergeSMTPAuthentication(incoming, existing []cmodels.Property) ([]cmodels.Property, error) {
	incomingValues, err := propertyValues(incoming)
	if err != nil {
		return nil, err
	}
	existingValues, err := propertyValues(existing)
	if err != nil {
		return nil, err
	}
	var request *outboundauth.Authentication
	if _, present := incomingValues[outboundauth.PropertyKeyType]; present {
		config, err := outboundauth.FromProperties(incoming)
		if err != nil {
			return nil, err
		}
		request = &outboundauth.Authentication{Type: string(config.Type), Properties: config.Properties}
	}
	targetKeys := []string{ncommon.SMTPPropKeyHost, ncommon.SMTPPropKeyPort}
	previousTarget := make([]string, len(targetKeys))
	nextTarget := make([]string, len(targetKeys))
	for i, key := range targetKeys {
		previousTarget[i] = existingValues[key]
		nextTarget[i] = incomingValues[key]
	}
	storedAuth := make([]cmodels.Property, 0)
	for _, property := range existing {
		if outboundauth.OwnsPropertyKey(property.GetName()) {
			storedAuth = append(storedAuth, property)
		}
	}
	properties, err := outboundauth.UpdateProperties(request, storedAuth, previousTarget, nextTarget,
		smtpauth.SupportedTypes(), nil, nil)
	if err != nil {
		return nil, err
	}
	result := make([]cmodels.Property, 0, len(incoming)+len(properties))
	for _, property := range incoming {
		if !outboundauth.OwnsPropertyKey(property.GetName()) {
			result = append(result, property)
		}
	}
	return append(result, properties...), nil
}

// portValue renders a port for storage. Zero means "not supplied", which appendProperty skips
// so the sender service reports the property as missing rather than as port 0.
func portValue(port int) string {
	if port == 0 {
		return ""
	}
	return strconv.Itoa(port)
}
