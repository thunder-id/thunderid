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

package role

import "github.com/thunder-id/thunderid/internal/system/utils"

// AssigneeType represents the type of assignee principal.
type AssigneeType string

// Public assignee types accepted in requests and returned in responses.
const (
	// AssigneeTypeUser is the public type for user principals.
	AssigneeTypeUser AssigneeType = "user"
	// AssigneeTypeApp is the public type for application principals.
	AssigneeTypeApp AssigneeType = "app"
	// AssigneeTypeAgent is the public type for agent principals.
	AssigneeTypeAgent AssigneeType = "agent"
	// AssigneeTypeGroup is the public type for group principals.
	AssigneeTypeGroup AssigneeType = "group"
)

// Internal assignee types used only for storage.
const (
	assigneeTypeEntity AssigneeType = "entity"
)

// IsEntityType reports whether t is an entity type (user, app, agent) that maps
// to the internal entity storage type.
func (t AssigneeType) IsEntityType() bool {
	switch t {
	case AssigneeTypeUser, AssigneeTypeApp, AssigneeTypeAgent:
		return true
	}
	return false
}

// AssignmentResponse represents an assignment of a role to a user or group.
type AssignmentResponse struct {
	ID      string       `json:"id"`
	Type    AssigneeType `json:"type"`
	Display string       `json:"display,omitempty"`
}

// AssignmentRequest represents an assignment of a role to a user or group.
type AssignmentRequest struct {
	ID   string       `json:"id"`
	Type AssigneeType `json:"type"`
}

// RoleSummaryResponse represents the basic information of a role.
type RoleSummaryResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OUID        string `json:"ouId"`
	OUHandle    string `json:"ouHandle,omitempty"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// RoleResponse represents a complete role with permissions.
type RoleResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	OUID        string                `json:"ouId"`
	OUHandle    string                `json:"ouHandle,omitempty"`
	Permissions []ResourcePermissions `json:"permissions"`
}

// CreateRoleRequest represents the request body for creating a role.
type CreateRoleRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	OUID        string                `json:"ouId"`
	Permissions []ResourcePermissions `json:"permissions"`
	Assignments []AssignmentRequest   `json:"assignments,omitempty"`
}

// CreateRoleResponse represents the response body for creating a role.
type CreateRoleResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	OUID        string                `json:"ouId"`
	OUHandle    string                `json:"ouHandle,omitempty"`
	Permissions []ResourcePermissions `json:"permissions"`
	Assignments []AssignmentResponse  `json:"assignments,omitempty"`
}

// UpdateRoleRequest represents the request body for updating a role.
type UpdateRoleRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	OUID        string                `json:"ouId"`
	Permissions []ResourcePermissions `json:"permissions"`
}

// AssignmentsRequest represents the request body for adding or removing assignments.
type AssignmentsRequest struct {
	Assignments []AssignmentRequest `json:"assignments"`
}

// RoleListResponse represents the response for listing roles with pagination.
type RoleListResponse struct {
	TotalResults int                   `json:"totalResults"`
	StartIndex   int                   `json:"startIndex"`
	Count        int                   `json:"count"`
	Roles        []RoleSummaryResponse `json:"roles"`
	Links        []utils.Link          `json:"links"`
}

// AssignmentListResponse represents the response for listing role assignments with pagination.
type AssignmentListResponse struct {
	TotalResults int                  `json:"totalResults"`
	StartIndex   int                  `json:"startIndex"`
	Count        int                  `json:"count"`
	Assignments  []AssignmentResponse `json:"assignments"`
	Links        []utils.Link         `json:"links"`
}

// Internal service layer structs - used for business logic processing

// ResourcePermissions represents permissions grouped by resource server.
type ResourcePermissions struct {
	ResourceServerID string   `json:"resourceServerId" yaml:"resource_server_id"`
	Permissions      []string `json:"permissions" yaml:"permissions"`
}

// RoleCreationDetail represents the parameters for creating a role.
// ID is optional; if empty, the service generates a new UUID.
type RoleCreationDetail struct {
	ID          string
	Name        string
	Description string
	OUID        string
	Permissions []ResourcePermissions
	Assignments []RoleAssignment
}

// RoleWithPermissionsAndAssignments represents the parameters for creating a role.
type RoleWithPermissionsAndAssignments struct {
	ID          string
	Name        string
	Description string
	OUID        string
	OUHandle    string
	Permissions []ResourcePermissions
	Assignments []RoleAssignment
}

// RoleAssignment represents an assignment used internally by the service layer.
type RoleAssignment struct {
	ID   string       `yaml:"id"`
	Type AssigneeType `yaml:"type"`
}

// RoleAssignmentWithDisplay represents an assignment used internally by the service layer.
type RoleAssignmentWithDisplay struct {
	ID      string
	Type    AssigneeType
	Display string
}

// Role represents basic role information used internally by the service layer.
type Role struct {
	ID          string
	Name        string
	Description string
	OUID        string
	OUHandle    string
	IsReadOnly  bool
}

// RoleWithPermissions represents complete role details used internally by the service layer.
type RoleWithPermissions struct {
	ID          string
	Name        string
	Description string
	OUID        string
	OUHandle    string
	Permissions []ResourcePermissions
}

// RoleUpdateDetail represents the parameters for creating a role.
type RoleUpdateDetail struct {
	Name        string
	Description string
	OUID        string
	Permissions []ResourcePermissions
}

// RoleList represents the result of listing roles.
type RoleList struct {
	TotalResults int
	StartIndex   int
	Count        int
	Roles        []Role
	Links        []utils.Link
}

// AssignmentList represents the result of listing role assignments.
type AssignmentList struct {
	TotalResults int
	StartIndex   int
	Count        int
	Assignments  []RoleAssignmentWithDisplay
	Links        []utils.Link
}
