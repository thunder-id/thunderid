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

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type recordingCapturer struct {
	calls [][3]string
}

func (r *recordingCapturer) CaptureSecret(_ context.Context, _, resourceName, field, value string) {
	r.calls = append(r.calls, [3]string{resourceName, field, value})
}

func TestApplicationCaptureSecret(t *testing.T) {
	t.Run("forwards non-empty value to the capturer", func(t *testing.T) {
		rec := &recordingCapturer{}
		as := &applicationService{secretCapturer: rec}

		as.captureSecret(context.Background(), "My App", "s3cret")

		assert.Equal(t, [][3]string{{"My App", "ClientSecret", "s3cret"}}, rec.calls)
	})

	t.Run("skips empty values", func(t *testing.T) {
		rec := &recordingCapturer{}
		as := &applicationService{secretCapturer: rec}

		as.captureSecret(context.Background(), "My App", "")

		assert.Empty(t, rec.calls)
	})

	t.Run("no-op when no capturer is configured", func(t *testing.T) {
		as := &applicationService{}
		assert.NotPanics(t, func() {
			as.captureSecret(context.Background(), "My App", "s3cret")
		})
	})
}
