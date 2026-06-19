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

package notification

import (
	"github.com/thunder-id/thunderid/internal/integration"
	"github.com/thunder-id/thunderid/internal/notification/common"
)

// Integrations returns the integration catalog descriptors for all supported
// message notification providers.
func Integrations() []integration.Descriptor {
	return []integration.Descriptor{
		{
			Type:              string(common.MessageProviderTypeTwilio),
			DisplayName:       "Twilio",
			Category:          integration.CategorySMS,
			HostedCredentials: false,
			Fields: []integration.Field{
				{Name: common.TwilioPropKeyAccountSID, Required: true},
				{Name: common.TwilioPropKeyAuthToken, Required: true, Secret: true},
				{Name: common.TwilioPropKeySenderID, Required: true},
			},
		},
		{
			Type:              string(common.MessageProviderTypeVonage),
			DisplayName:       "Vonage",
			Category:          integration.CategorySMS,
			HostedCredentials: false,
			Fields: []integration.Field{
				{Name: common.VonagePropKeyAPIKey, Required: true},
				{Name: common.VonagePropKeyAPISecret, Required: true, Secret: true},
				{Name: common.VonagePropKeySenderID, Required: true},
			},
		},
		{
			Type:              string(common.MessageProviderTypeCustom),
			DisplayName:       "Custom Gateway",
			Category:          integration.CategorySMS,
			HostedCredentials: false,
			Fields: []integration.Field{
				{Name: common.CustomPropKeyURL, Required: true},
				{Name: common.CustomPropKeyHTTPMethod, Required: true},
				{Name: common.CustomPropKeyHTTPHeaders, Required: false},
				{Name: common.CustomPropKeyContentType, Required: false},
			},
		},
	}
}
