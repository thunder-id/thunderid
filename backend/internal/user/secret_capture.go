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

package user

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// SecretCapturer stores a resource credential into the Control Plane secret store at creation time,
// keyed by the declarative placeholder the credential resolves. It is optional: the Data Plane wires
// no capturer, so user creation there behaves exactly as before (no extra schema lookup either).
type SecretCapturer interface {
	CaptureSecret(ctx context.Context, resourceType, resourceName, field, value string)
}

// captureUserCredentials stores each schema-defined credential of a newly created user into the
// Control Plane secret store, keyed by the placeholder the exporter emits (<USERNAME>_<CREDENTIAL>).
// It is a no-op when no capturer is configured, and it is best-effort: any failure is logged and
// never affects the outcome of user creation.
func (us *userService) captureUserCredentials(ctx context.Context, user *User) {
	if us.secretCapturer == nil || user == nil || us.entityTypeService == nil {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	var attrs map[string]any
	if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
		return
	}
	username, _ := attrs["username"].(string)
	if username == "" {
		return
	}

	credInfos, svcErr := us.entityTypeService.GetAttributes(
		ctx, entitytype.TypeCategoryUser, user.Type,
		entitytype.AttributeFilter{AllowCredential: true})
	if svcErr != nil {
		logger.Warn(ctx, "Secret auto-capture: failed to read credential attributes for user",
			log.String("userType", user.Type))
		return
	}

	for _, ci := range credInfos {
		if value, ok := attrs[ci.Attribute].(string); ok && value != "" {
			us.secretCapturer.CaptureSecret(ctx, resourceTypeUser, username, ci.Attribute, value)
		}
	}
}

// captureUpdatedCredentials stores credentials changed after creation, keyed by the same placeholder
// the exporter emits.
//
// Creation is not the only moment a credential is set: rotating a password produces a new value that
// the data plane must receive, and without this the store keeps serving the one from creation, so the
// rotated credential silently never takes effect.
func (us *userService) captureUpdatedCredentials(ctx context.Context, userID string,
	plaintextCreds map[string]string) {
	if us.secretCapturer == nil || len(plaintextCreds) == 0 {
		return
	}

	user, svcErr := us.GetUser(ctx, userID, false)
	if svcErr != nil || user == nil {
		return
	}
	var attrs map[string]any
	if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
		return
	}
	username, _ := attrs["username"].(string)
	if username == "" {
		return
	}

	for field, value := range plaintextCreds {
		if value != "" {
			us.secretCapturer.CaptureSecret(ctx, resourceTypeUser, username, field, value)
		}
	}
}
