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

package openid4vci

import (
	"context"
	"encoding/json"
	"fmt"
)

// resolveClaims loads the authenticated subject's profile attributes and selects
// the configured selectively disclosable claims for the credential. Attributes
// absent on the user are simply omitted from the issued credential.
func (s *service) resolveClaims(
	ctx context.Context, userID string, claimNames []string,
) (map[string]interface{}, error) {
	u, svcErr := s.userService.GetUser(ctx, userID, false)
	if svcErr != nil || u == nil {
		return nil, fmt.Errorf("%w: %s", ErrUserNotFound, userID)
	}

	var attrs map[string]interface{}
	if len(u.Attributes) > 0 {
		if err := json.Unmarshal(u.Attributes, &attrs); err != nil {
			return nil, fmt.Errorf("%w: failed to decode user attributes: %w", ErrIssuance, err)
		}
	}

	claims := make(map[string]interface{}, len(claimNames))
	for _, name := range claimNames {
		if v, ok := attrs[name]; ok {
			claims[name] = v
		}
	}
	return claims, nil
}
