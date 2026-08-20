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

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
)

// CaptureSecretForTenant relays a secret the control plane captured to the secret provider of the one
// environment the control plane administers directly.
//
// A credential is created once, in the organization's single workspace, but no control plane holds
// one: they live in each data plane's own store. It goes to that environment alone, because creating
// an application while developing must not reach into production and set the credential running
// there. The others receive theirs when one is set against them deliberately.
//
// It returns how many providers received the secret. Zero with no error means the organization has no
// environment registered yet, which the caller treats as "nothing to do" rather than a failure:
// secrets created before an environment exists are recreated on promote.
func (s *Service) CaptureSecretForTenant(ctx context.Context, deploymentID, name string,
	body map[string]interface{}) (int, error) {
	if strings.TrimSpace(deploymentID) == "" || strings.TrimSpace(name) == "" {
		return 0, fmt.Errorf("%w: a deployment id and a secret name are required", ErrValidation)
	}

	envs, err := s.store.ListEnvironments(ctx)
	if err != nil {
		return 0, err
	}
	target, ok := managedEnvironment(envs)
	if !ok {
		return 0, nil
	}

	if _, err := s.queueSecret(ctx, target, name, body); err != nil {
		return 0, fmt.Errorf("failed to store the secret for %s: %w", target.Name, err)
	}
	return 1, nil
}

// managedEnvironment is the environment the control plane administers directly, which is where a
// credential created in the workspace is issued.
//
// It is the one marked. With none marked, the lowest rank stands in: that is the bottom of the
// promotion chain, so it is where work starts and where a newly created resource belongs.
func managedEnvironment(envs []model.Environment) (model.Environment, bool) {
	var chosen model.Environment
	found := false
	for _, env := range envs {
		if env.ManagedByControlPlane {
			return env, true
		}
		if !found || env.Rank < chosen.Rank {
			chosen, found = env, true
		}
	}
	return chosen, found
}
