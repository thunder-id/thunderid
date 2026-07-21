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

package connection //nolint:dupl // sms-gateway mirrors twilio/vonage's shape, kept distinct per vendor

import (
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// smsGatewayConnectionRequest is the create/update payload for a generic HTTP SMS gateway
// connection (a custom webhook, unlike the Twilio/Vonage vendor-specific integrations).
type smsGatewayConnectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
	HTTPMethod  string `json:"httpMethod,omitempty"`
	HTTPHeaders string `json:"httpHeaders,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

// smsGatewayConnectionResponse is the detail payload for an SMS gateway connection.
type smsGatewayConnectionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	URL         string `json:"url,omitempty"`
	HTTPMethod  string `json:"httpMethod,omitempty"`
	HTTPHeaders string `json:"httpHeaders,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

func smsGatewayToSenderDTO(req smsGatewayConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, ncommon.CustomPropKeyURL, req.URL, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.CustomPropKeyHTTPMethod, req.HTTPMethod, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.CustomPropKeyHTTPHeaders, req.HTTPHeaders, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, ncommon.CustomPropKeyContentType, req.ContentType, false); err != nil {
		return nil, err
	}
	return &ncommon.NotificationSenderDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        ncommon.NotificationSenderTypeMessage,
		Provider:    ncommon.MessageProviderTypeCustom,
		Properties:  props,
	}, nil
}

func smsGatewayFromSenderDTO(dto ncommon.NotificationSenderDTO) (smsGatewayConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return smsGatewayConnectionResponse{}, err
	}
	return smsGatewayConnectionResponse{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		Type:        smsGatewayVendorName,
		URL:         values[ncommon.CustomPropKeyURL],
		HTTPMethod:  values[ncommon.CustomPropKeyHTTPMethod],
		HTTPHeaders: values[ncommon.CustomPropKeyHTTPHeaders],
		ContentType: values[ncommon.CustomPropKeyContentType],
	}, nil
}
