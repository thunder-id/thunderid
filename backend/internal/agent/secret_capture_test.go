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

package agent

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

func TestCaptureSecret(t *testing.T) {
	t.Run("captures the client secret under the exported field name", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &agentService{secretCapturer: rec}

		s.captureSecret(context.Background(), "Test", "s3cret")

		assert.Equal(t, [][3]string{{"Test", "ClientSecret", "s3cret"}}, rec.calls)
	})

	t.Run("skips a public client that has no secret", func(t *testing.T) {
		rec := &recordingCapturer{}
		s := &agentService{secretCapturer: rec}

		s.captureSecret(context.Background(), "Test", "")

		assert.Empty(t, rec.calls)
	})

	t.Run("no-op when no capturer is configured", func(t *testing.T) {
		s := &agentService{}
		assert.NotPanics(t, func() {
			s.captureSecret(context.Background(), "Test", "s3cret")
		})
	})
}
