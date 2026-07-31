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

// httpEmailConnectionRequest is the create/update payload for an HTTP webhook email connection.
type httpEmailConnectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
	HTTPMethod  string `json:"httpMethod,omitempty"`
	HTTPHeaders string `json:"httpHeaders,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

// httpEmailConnectionResponse is the detail payload for an HTTP webhook email connection.
type httpEmailConnectionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	URL         string `json:"url,omitempty"`
	HTTPMethod  string `json:"httpMethod,omitempty"`
	HTTPHeaders string `json:"httpHeaders,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

// httpEmailToSenderDTO converts an HTTP webhook email connection request to an HTTP webhook email sender DTO.
func httpEmailToSenderDTO(req httpEmailConnectionRequest) (*ncommon.NotificationSenderDTO, error) {
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
		Type:        ncommon.NotificationSenderTypeEmail,
		Provider:    ncommon.NotificationProviderTypeHTTP,
		Properties:  props,
	}, nil
}

// httpEmailFromSenderDTO converts an HTTP webhook email sender DTO to an HTTP webhook email connection response.
func httpEmailFromSenderDTO(dto ncommon.NotificationSenderDTO) (httpEmailConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return httpEmailConnectionResponse{}, err
	}
	return httpEmailConnectionResponse{
		ID:          dto.ID,
		Name:        dto.Name,
		Description: dto.Description,
		Type:        string(ncommon.NotificationProviderTypeHTTP),
		URL:         values[ncommon.CustomPropKeyURL],
		HTTPMethod:  values[ncommon.CustomPropKeyHTTPMethod],
		HTTPHeaders: values[ncommon.CustomPropKeyHTTPHeaders],
		ContentType: values[ncommon.CustomPropKeyContentType],
	}, nil
}
