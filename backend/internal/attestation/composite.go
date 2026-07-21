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

package attestation

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// compositeVerifier routes attestation verification to the platform provider that matches the
// application's configuration. An application configures exactly one platform, so the platform is
// determined by which sub-config is present.
type compositeVerifier struct {
	android providers.AttestationProvider
	apple   providers.AttestationProvider
	logger  *log.Logger
}

// newCompositeVerifier creates a platform-dispatching attestation provider.
func newCompositeVerifier(android, apple providers.AttestationProvider) providers.AttestationProvider {
	return &compositeVerifier{
		android: android,
		apple:   apple,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AttestationVerifier")),
	}
}

// Verify dispatches to the Android (Play Integrity) or Apple (App Attest) verifier based on the
// configured platform. A configuration with no platform set is an operational error.
func (c *compositeVerifier) Verify(ctx context.Context, cfg *providers.AttestationConfig, token string) (
	bool, *tidcommon.ServiceError) {
	switch {
	case cfg != nil && cfg.Android != nil:
		return c.android.Verify(ctx, cfg, token)
	case cfg != nil && cfg.Apple != nil:
		return c.apple.Verify(ctx, cfg, token)
	default:
		c.logger.Error(ctx, "Attestation requested without a platform configuration")
		return false, &tidcommon.InternalServerError
	}
}
