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

import "context"

// SecretCapturer stores a resource credential into the Control Plane secret store at creation time,
// keyed by the declarative placeholder the credential resolves. It is optional: the Data Plane wires
// no capturer, so creation there behaves exactly as before.
type SecretCapturer interface {
	CaptureSecret(ctx context.Context, resourceType, resourceName, field, value string)
}

// captureSecret forwards an agent's client secret to the configured capturer. It is a no-op when no
// capturer is set or the value is empty, and it never affects the outcome of the calling operation.
func (s *agentService) captureSecret(ctx context.Context, resourceName, value string) {
	if s.secretCapturer == nil || value == "" {
		return
	}
	s.secretCapturer.CaptureSecret(ctx, resourceTypeAgent, resourceName, "ClientSecret", value)
}
