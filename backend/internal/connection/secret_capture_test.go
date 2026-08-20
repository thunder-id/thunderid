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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/idp"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type recordingCapturer struct {
	calls [][3]string
}

func (r *recordingCapturer) CaptureReplayableSecret(_ context.Context, _, resourceName, field, value string) {
	r.calls = append(r.calls, [3]string{resourceName, field, value})
}

// The field name each capture uses has to match the variable the exporter parameterizes for that
// vendor, otherwise the promoted document references a variable the secret store does not hold.
func TestCaptureConnectionSecrets(t *testing.T) {
	initConfigWithTestCryptoKey(t)
	t.Cleanup(config.ResetServerRuntime)

	t.Run("captures an identity provider client secret", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &service{secretCapturer: rec}

		s.captureIDPSecret(context.Background(), &providers.IDPDTO{
			Name: "my-google",
			Properties: []cmodels.Property{
				mustProperty(t, idp.PropClientID, "client-id", false),
				mustProperty(t, idp.PropClientSecret, "s3cret", true),
			},
		})

		assert.Equal(t, [][3]string{{"my-google", "ClientSecret", "s3cret"}}, rec.calls)
	})

	t.Run("captures a Vonage API secret", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &service{secretCapturer: rec}

		s.captureSenderSecret(context.Background(), &ncommon.NotificationSenderDTO{
			Name:     "my-vonage",
			Provider: ncommon.MessageProviderTypeVonage,
			Properties: []cmodels.Property{
				mustProperty(t, ncommon.VonagePropKeyAPIKey, "key", false),
				mustProperty(t, ncommon.VonagePropKeyAPISecret, "api-s3cret", true),
			},
		})

		assert.Equal(t, [][3]string{{"my-vonage", "APISecret", "api-s3cret"}}, rec.calls)
	})

	t.Run("captures a Twilio auth token", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &service{secretCapturer: rec}

		s.captureSenderSecret(context.Background(), &ncommon.NotificationSenderDTO{
			Name:     "my-twilio",
			Provider: ncommon.MessageProviderTypeTwilio,
			Properties: []cmodels.Property{
				mustProperty(t, ncommon.TwilioPropKeyAuthToken, "tok3n", true),
			},
		})

		assert.Equal(t, [][3]string{{"my-twilio", "AuthToken", "tok3n"}}, rec.calls)
	})

	t.Run("skips a value that only references a secret held elsewhere", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &service{secretCapturer: rec}

		s.captureIDPSecret(context.Background(), &providers.IDPDTO{
			Name: "imported",
			Properties: []cmodels.Property{
				mustProperty(t, idp.PropClientSecret, "kv:IMPORTED_CLIENT_SECRET", false),
			},
		})

		assert.Empty(t, rec.calls)
	})

	t.Run("skips the read mask echoed back by a client", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &service{secretCapturer: rec}

		s.captureIDPSecret(context.Background(), &providers.IDPDTO{
			Name: "echoed",
			Properties: []cmodels.Property{
				mustProperty(t, idp.PropClientSecret, maskedSecretValue, true),
			},
		})

		assert.Empty(t, rec.calls)
	})

	t.Run("skips a vendor that holds no secret", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &service{secretCapturer: rec}

		s.captureSenderSecret(context.Background(), &ncommon.NotificationSenderDTO{
			Name:     "my-gateway",
			Provider: ncommon.MessageProviderTypeCustom,
			Properties: []cmodels.Property{
				mustProperty(t, ncommon.CustomPropKeyURL, "https://example.com", false),
			},
		})

		assert.Empty(t, rec.calls)
	})

	t.Run("no-op when no capturer is configured", func(t *testing.T) {
		s := &service{}
		assert.NotPanics(t, func() {
			s.captureIDPSecret(context.Background(), &providers.IDPDTO{Name: "x"})
			s.captureSenderSecret(context.Background(), &ncommon.NotificationSenderDTO{Name: "x"})
		})
	})
}
