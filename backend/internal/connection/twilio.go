// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection //nolint:dupl // twilio mirrors vonage's shape, kept distinct per vendor

import (
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// twilioConnectionRequest is the create/update payload for a Twilio SMS connection.
type twilioConnectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AccountSID  string `json:"accountSid"`
	AuthToken   string `json:"authToken"`
	SenderID    string `json:"senderId"`
}

// twilioConnectionResponse is the detail payload for a Twilio connection (secret masked).
type twilioConnectionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	AccountSID  string `json:"accountSid,omitempty"`
	AuthToken   string `json:"authToken,omitempty"`
	SenderID    string `json:"senderId,omitempty"`
}

func twilioToSenderDTO(req twilioConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, ncommon.TwilioPropKeyAccountSID, req.AccountSID, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.TwilioPropKeyAuthToken, req.AuthToken, true); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.TwilioPropKeySenderID, req.SenderID, false); err != nil {
		return nil, err
	}
	return &ncommon.NotificationSenderDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        ncommon.NotificationSenderTypeMessage,
		Provider:    ncommon.NotificationProviderTypeTwilio,
		Properties:  props,
	}, nil
}

func twilioFromSenderDTO(dto ncommon.NotificationSenderDTO) (twilioConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return twilioConnectionResponse{}, err
	}
	return twilioConnectionResponse{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		Type:        string(ncommon.NotificationProviderTypeTwilio),
		AccountSID:  values[ncommon.TwilioPropKeyAccountSID],
		AuthToken:   values[ncommon.TwilioPropKeyAuthToken],
		SenderID:    values[ncommon.TwilioPropKeySenderID],
	}, nil
}
