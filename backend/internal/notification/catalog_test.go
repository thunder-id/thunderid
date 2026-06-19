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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/integration"
	"github.com/thunder-id/thunderid/internal/notification/common"
)

func findIntegration(descriptors []integration.Descriptor,
	connectorType string) *integration.Descriptor {
	for i := range descriptors {
		if descriptors[i].Type == connectorType {
			return &descriptors[i]
		}
	}
	return nil
}

func fieldByName(d *integration.Descriptor, name string) *integration.Field {
	for i := range d.Fields {
		if d.Fields[i].Name == name {
			return &d.Fields[i]
		}
	}
	return nil
}

func TestIntegrationsCoversAllProviders(t *testing.T) {
	descriptors := Integrations()

	assert.Len(t, descriptors, 3)
	for _, provider := range []common.MessageProviderType{
		common.MessageProviderTypeTwilio,
		common.MessageProviderTypeVonage,
		common.MessageProviderTypeCustom,
	} {
		d := findIntegration(descriptors, string(provider))
		assert.NotNil(t, d, "missing integration for %s", provider)
		assert.Equal(t, integration.CategorySMS, d.Category)
		assert.False(t, d.HostedCredentials)
	}
}

func TestIntegrationsSecretFields(t *testing.T) {
	descriptors := Integrations()

	twilio := findIntegration(descriptors, string(common.MessageProviderTypeTwilio))
	assert.NotNil(t, twilio)
	authToken := fieldByName(twilio, common.TwilioPropKeyAuthToken)
	assert.NotNil(t, authToken)
	assert.True(t, authToken.Required)
	assert.True(t, authToken.Secret)
	accountSID := fieldByName(twilio, common.TwilioPropKeyAccountSID)
	assert.NotNil(t, accountSID)
	assert.False(t, accountSID.Secret)

	vonage := findIntegration(descriptors, string(common.MessageProviderTypeVonage))
	assert.NotNil(t, vonage)
	apiSecret := fieldByName(vonage, common.VonagePropKeyAPISecret)
	assert.NotNil(t, apiSecret)
	assert.True(t, apiSecret.Secret)
}

func TestIntegrationsCustomOptionalFields(t *testing.T) {
	descriptors := Integrations()

	custom := findIntegration(descriptors, string(common.MessageProviderTypeCustom))
	assert.NotNil(t, custom)

	url := fieldByName(custom, common.CustomPropKeyURL)
	assert.NotNil(t, url)
	assert.True(t, url.Required)

	headers := fieldByName(custom, common.CustomPropKeyHTTPHeaders)
	assert.NotNil(t, headers)
	assert.False(t, headers.Required)
}
