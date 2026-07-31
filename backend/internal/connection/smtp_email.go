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

package connection

import (
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// smtpConnectionRequest is the create/update payload for an SMTP email connection.
type smtpConnectionRequest struct {
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	Host                 string `json:"host"`
	Port                 string `json:"port"`
	Username             string `json:"username,omitempty"`
	Password             string `json:"password,omitempty"`
	FromAddress          string `json:"fromAddress"`
	TLS                  string `json:"tls,omitempty"`
	EnableAuthentication string `json:"enableAuthentication,omitempty"`
}

// smtpConnectionResponse is the detail payload for an SMTP email connection (secrets masked).
type smtpConnectionResponse struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	Type                 string `json:"type"`
	Host                 string `json:"host,omitempty"`
	Port                 string `json:"port,omitempty"`
	Username             string `json:"username,omitempty"`
	Password             string `json:"password,omitempty"`
	FromAddress          string `json:"fromAddress,omitempty"`
	TLS                  string `json:"tls,omitempty"`
	EnableAuthentication string `json:"enableAuthentication,omitempty"`
}

// smtpToSenderDTO converts an SMTP email connection request to an SMTP email sender DTO.
func smtpToSenderDTO(req smtpConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, ncommon.SMTPPropKeyHost, req.Host, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyPort, req.Port, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyUsername, req.Username, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyPassword, req.Password, true); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyFromAddress, req.FromAddress, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyTLS, req.TLS, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.SMTPPropKeyEnableAuth, req.EnableAuthentication, false); err != nil {
		return nil, err
	}
	return &ncommon.NotificationSenderDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        ncommon.NotificationSenderTypeEmail,
		Provider:    ncommon.NotificationProviderTypeSMTP,
		Properties:  props,
	}, nil
}

// smtpFromSenderDTO converts an SMTP email sender DTO to an SMTP email connection response.
func smtpFromSenderDTO(dto ncommon.NotificationSenderDTO) (smtpConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return smtpConnectionResponse{}, err
	}
	return smtpConnectionResponse{
		ID:                   dto.ID,
		Name:                 dto.Name,
		Description:          dto.Description,
		Type:                 string(ncommon.NotificationProviderTypeSMTP),
		Host:                 values[ncommon.SMTPPropKeyHost],
		Port:                 values[ncommon.SMTPPropKeyPort],
		Username:             values[ncommon.SMTPPropKeyUsername],
		Password:             values[ncommon.SMTPPropKeyPassword],
		FromAddress:          values[ncommon.SMTPPropKeyFromAddress],
		TLS:                  values[ncommon.SMTPPropKeyTLS],
		EnableAuthentication: values[ncommon.SMTPPropKeyEnableAuth],
	}, nil
}
