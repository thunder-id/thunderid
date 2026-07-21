/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package connection //nolint:dupl // vonage mirrors twilio's shape, kept distinct per vendor

import (
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// vonageConnectionRequest is the create/update payload for a Vonage SMS connection.
type vonageConnectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	APIKey      string `json:"apiKey"`
	APISecret   string `json:"apiSecret"`
	SenderID    string `json:"senderId"`
}

// vonageConnectionResponse is the detail payload for a Vonage connection (secret masked).
type vonageConnectionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	APIKey      string `json:"apiKey,omitempty"`
	APISecret   string `json:"apiSecret,omitempty"`
	SenderID    string `json:"senderId,omitempty"`
}

func vonageToSenderDTO(req vonageConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, ncommon.VonagePropKeyAPIKey, req.APIKey, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.VonagePropKeyAPISecret, req.APISecret, true); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.VonagePropKeySenderID, req.SenderID, false); err != nil {
		return nil, err
	}
	return &ncommon.NotificationSenderDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        ncommon.NotificationSenderTypeMessage,
		Provider:    ncommon.MessageProviderTypeVonage,
		Properties:  props,
	}, nil
}

func vonageFromSenderDTO(dto ncommon.NotificationSenderDTO) (vonageConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return vonageConnectionResponse{}, err
	}
	return vonageConnectionResponse{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		Type:        string(ncommon.MessageProviderTypeVonage),
		APIKey:      values[ncommon.VonagePropKeyAPIKey],
		APISecret:   values[ncommon.VonagePropKeyAPISecret],
		SenderID:    values[ncommon.VonagePropKeySenderID],
	}, nil
}
