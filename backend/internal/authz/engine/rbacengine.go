/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package engine provides authorization engine implementations.
// It includes various authorization engines such as RBAC (Role-Based Access Control)
// that delegate authorization decisions to the appropriate services.
package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// rbacEngine implements Role-Based Access Control (RBAC) authorization.
// It delegates authorization decisions to the role service.
type rbacEngine struct {
	roleService providers.RoleProvider
}

// evaluationGroup groups access evaluations that can be checked in one role service call.
type evaluationGroup struct {
	subject     Subject
	permissions []string
	indexes     []int
}

// NewRBACEngine creates a new RBAC authorization engine.
func NewRBACEngine(roleService providers.RoleProvider) AuthorizationEngine {
	return &rbacEngine{
		roleService: roleService,
	}
}

// EvaluateAccess evaluates a single fine-grained access request.
func (e *rbacEngine) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := e.EvaluateAccessBatch(ctx, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{request},
	})
	if err != nil {
		return nil, err
	}
	if len(response.Evaluations) == 0 {
		return &AccessEvaluationResponse{}, nil
	}
	return &response.Evaluations[0], nil
}

// EvaluateAccessBatch evaluates multiple fine-grained access requests based on role assignments.
func (e *rbacEngine) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	if len(request.Evaluations) == 0 {
		return &AccessEvaluationsResponse{Evaluations: []AccessEvaluationResponse{}}, nil
	}

	evaluations := make([]AccessEvaluationResponse, len(request.Evaluations))
	for _, group := range groupEvaluations(request.Evaluations) {
		authorizedPerms, svcErr := e.roleService.GetAuthorizedPermissions(
			ctx, group.subject.ID, group.subject.GroupIDs, group.permissions)
		if svcErr != nil {
			return nil, fmt.Errorf("role service error: %s", svcErr.Error)
		}

		for _, index := range group.indexes {
			permission := request.Evaluations[index].Permission.Name
			evaluations[index] = AccessEvaluationResponse{
				Decision: slices.Contains(authorizedPerms, permission),
			}
		}
	}
	return &AccessEvaluationsResponse{Evaluations: evaluations}, nil
}

// groupEvaluations groups access evaluations by subject and collects unique permissions.
func groupEvaluations(evaluations []AccessEvaluationRequest) []evaluationGroup {
	groups := make([]evaluationGroup, 0, len(evaluations))
	for index, evaluation := range evaluations {
		groupIndex := findEvaluationGroup(groups, evaluation.Subject)
		if groupIndex == -1 {
			groups = append(groups, evaluationGroup{subject: evaluation.Subject})
			groupIndex = len(groups) - 1
		}

		permission := evaluation.Permission.Name
		if !slices.Contains(groups[groupIndex].permissions, permission) {
			groups[groupIndex].permissions = append(groups[groupIndex].permissions, permission)
		}
		groups[groupIndex].indexes = append(groups[groupIndex].indexes, index)
	}
	return groups
}

// findEvaluationGroup returns the index of the group matching the subject.
func findEvaluationGroup(groups []evaluationGroup, subject Subject) int {
	for i, group := range groups {
		if group.subject.Type == subject.Type &&
			group.subject.ID == subject.ID &&
			slices.Equal(group.subject.GroupIDs, subject.GroupIDs) {
			return i
		}
	}
	return -1
}
