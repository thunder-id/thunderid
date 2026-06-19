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

package idp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/integration"
)

func findIntegration(descriptors []integration.Descriptor,
	idpType IDPType) *integration.Descriptor {
	for i := range descriptors {
		if descriptors[i].Type == string(idpType) {
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

func TestIntegrationsCoversAllSupportedTypes(t *testing.T) {
	descriptors := Integrations()

	assert.Len(t, descriptors, len(supportedIDPTypes))
	for _, idpType := range supportedIDPTypes {
		assert.NotNil(t, findIntegration(descriptors, idpType), "missing integration for %s", idpType)
	}
}

func TestIntegrationsPresentation(t *testing.T) {
	descriptors := Integrations()

	google := findIntegration(descriptors, IDPTypeGoogle)
	assert.NotNil(t, google)
	assert.Equal(t, "Google", google.DisplayName)
	assert.Equal(t, integration.CategorySocialLogin, google.Category)
	assert.False(t, google.HostedCredentials)

	oidc := findIntegration(descriptors, IDPTypeOIDC)
	assert.NotNil(t, oidc)
	assert.Equal(t, integration.CategoryEnterprise, oidc.Category)
}

func TestIntegrationsFieldMetadata(t *testing.T) {
	descriptors := Integrations()
	google := findIntegration(descriptors, IDPTypeGoogle)
	assert.NotNil(t, google)

	clientID := fieldByName(google, PropClientID)
	assert.NotNil(t, clientID)
	assert.True(t, clientID.Required)
	assert.False(t, clientID.Secret)

	clientSecret := fieldByName(google, PropClientSecret)
	assert.NotNil(t, clientSecret)
	assert.True(t, clientSecret.Required)
	assert.True(t, clientSecret.Secret)

	// Optional properties carry their configured defaults.
	authEndpoint := fieldByName(google, PropAuthorizationEndpoint)
	assert.NotNil(t, authEndpoint)
	assert.False(t, authEndpoint.Required)
	assert.Equal(t, googleAuthorizationEndpoint, authEndpoint.Default)
}
