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
	"context"

	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// SecretCapturer stores a resource credential into the Control Plane secret store, keyed by the
// declarative placeholder the credential resolves. It is optional: the Data Plane wires no capturer,
// so creation and update there behave exactly as before.
//
// Every connection credential is captured as replayable, because the Data Plane presents it to the
// third party the connection points at. A hash would authenticate nobody.
type SecretCapturer interface {
	CaptureReplayableSecret(ctx context.Context, resourceType, resourceName, field, value string)
}

// captureIDPSecret forwards an identity-provider-backed connection's client secret to the configured
// capturer, under the same field name the exporter parameterizes ("ClientSecret"), so the captured
// secret and the generated template variable agree.
func (s *service) captureIDPSecret(ctx context.Context, dto *providers.IDPDTO) {
	if s.secretCapturer == nil || dto == nil {
		return
	}
	s.captureProperty(ctx, dto.Name, "ClientSecret", dto.Properties, idp.PropClientSecret)
}

// captureSenderSecret forwards a notification-sender-backed connection's secret to the configured
// capturer, under the field name the exporter parameterizes for that vendor.
func (s *service) captureSenderSecret(ctx context.Context, dto *ncommon.NotificationSenderDTO) {
	if s.secretCapturer == nil || dto == nil {
		return
	}
	switch dto.Provider {
	case ncommon.MessageProviderTypeTwilio:
		s.captureProperty(ctx, dto.Name, "AuthToken", dto.Properties, ncommon.TwilioPropKeyAuthToken)
	case ncommon.MessageProviderTypeVonage:
		s.captureProperty(ctx, dto.Name, "APISecret", dto.Properties, ncommon.VonagePropKeyAPISecret)
	}
}

// captureProperty reads one property's usable value and hands it to the capturer. A property that is
// absent, empty, or already a reference to a secret held elsewhere is skipped: there is nothing new to
// store, and storing a reference would overwrite the value it points at. The read mask is skipped for
// the same reason: a client that echoes back what a read gave it is not setting a new secret.
func (s *service) captureProperty(ctx context.Context, resourceName, field string,
	props []cmodels.Property, propertyName string) {
	if resourceName == "" {
		return
	}
	for i := range props {
		if props[i].GetName() != propertyName {
			continue
		}
		// The stored value is read unresolved, so a reference is still recognizable as one. Resolving
		// first would hand back the credential it points at and capture it all over again.
		value, err := props[i].UnresolvedValue()
		if err != nil || value == "" || value == maskedSecretValue || secretresolver.IsReference(value) {
			return
		}
		s.secretCapturer.CaptureReplayableSecret(ctx, resourceTypeConnection, resourceName, field, value)
		return
	}
}
