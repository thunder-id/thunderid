// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// OrganizationUnitRequest represents the request body for creating an organization unit.
type OrganizationUnitRequest struct {
	Handle                    string  `json:"handle" native:"required,min=1,max=100"`
	Name                      string  `json:"name" native:"required,min=1,max=100"`
	Description               string  `json:"description,omitempty"`
	Parent                    *string `json:"parent" native:"omitempty,max=255"`
	ThemeID                   string  `json:"themeId,omitempty"`
	LayoutID                  string  `json:"layoutId,omitempty"`
	AuthFlowID                string  `json:"authFlowId,omitempty"`
	RegistrationFlowID        string  `json:"registrationFlowId,omitempty"`
	IsRegistrationFlowEnabled bool    `json:"isRegistrationFlowEnabled"`
	RecoveryFlowID            string  `json:"recoveryFlowId,omitempty"`
	IsRecoveryFlowEnabled     bool    `json:"isRecoveryFlowEnabled"`
	SignOutFlowID             string  `json:"signOutFlowId,omitempty"`
	UserOnboardingFlowID      string  `json:"userOnboardingFlowId,omitempty"`
	LogoURL                   string  `json:"logoUrl,omitempty" native:"omitempty,url,max=2048"`
	TosURI                    string  `json:"tosUri,omitempty" native:"omitempty,url,max=2048"`
	PolicyURI                 string  `json:"policyUri,omitempty" native:"omitempty,url,max=2048"`
	CookiePolicyURI           string  `json:"cookiePolicyUri,omitempty" native:"omitempty,url,max=2048"`
}

// OrganizationUnit is the management representation of an organization unit. It backs the REST
// API, the declarative YAML resource and the persistence layer. The runtime sees only the narrower
// OrganizationUnit.
type OrganizationUnit struct {
	ID                        string    `json:"id" yaml:"id"`
	Handle                    string    `json:"handle" yaml:"handle"`
	Name                      string    `json:"name" yaml:"name"`
	Description               string    `json:"description,omitempty" yaml:"description,omitempty"`
	Parent                    *string   `json:"parent" yaml:"parent"`
	ThemeID                   string    `json:"themeId,omitempty" yaml:"themeId,omitempty"`
	LayoutID                  string    `json:"layoutId,omitempty" yaml:"layoutId,omitempty"`
	AuthFlowID                string    `json:"authFlowId,omitempty" yaml:"authFlowId,omitempty"`
	RegistrationFlowID        string    `json:"registrationFlowId,omitempty" yaml:"registrationFlowId,omitempty"`
	IsRegistrationFlowEnabled bool      `json:"isRegistrationFlowEnabled" yaml:"isRegistrationFlowEnabled"`
	RecoveryFlowID            string    `json:"recoveryFlowId,omitempty" yaml:"recoveryFlowId,omitempty"`
	IsRecoveryFlowEnabled     bool      `json:"isRecoveryFlowEnabled" yaml:"isRecoveryFlowEnabled"`
	SignOutFlowID             string    `json:"signOutFlowId,omitempty" yaml:"signOutFlowId,omitempty"`
	UserOnboardingFlowID      string    `json:"userOnboardingFlowId,omitempty" yaml:"userOnboardingFlowId,omitempty"`
	LogoURL                   string    `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
	TosURI                    string    `json:"tosUri,omitempty" yaml:"tosUri,omitempty"`
	PolicyURI                 string    `json:"policyUri,omitempty" yaml:"policyUri,omitempty"`
	CookiePolicyURI           string    `json:"cookiePolicyUri,omitempty" yaml:"cookiePolicyUri,omitempty"`
	CreatedAt                 time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// OrganizationUnitRequestWithID represents the request body for creating an organization unit
// in import/declarative paths where preserving IDs is required.
type OrganizationUnitRequestWithID struct {
	ID                        string  `json:"id" yaml:"id" native:"required"`
	Handle                    string  `json:"handle" yaml:"handle" native:"required,min=1,max=100"`
	Name                      string  `json:"name" yaml:"name" native:"required,min=1,max=100"`
	Description               string  `json:"description,omitempty" yaml:"description,omitempty"`
	Parent                    *string `json:"parent" yaml:"parent"`
	ThemeID                   string  `json:"themeId,omitempty" yaml:"themeId,omitempty"`
	LayoutID                  string  `json:"layoutId,omitempty" yaml:"layoutId,omitempty"`
	AuthFlowID                string  `json:"authFlowId,omitempty" yaml:"authFlowId,omitempty"`
	RegistrationFlowID        string  `json:"registrationFlowId,omitempty" yaml:"registrationFlowId,omitempty"`
	IsRegistrationFlowEnabled bool    `json:"isRegistrationFlowEnabled" yaml:"isRegistrationFlowEnabled"`
	RecoveryFlowID            string  `json:"recoveryFlowId,omitempty" yaml:"recoveryFlowId,omitempty"`
	IsRecoveryFlowEnabled     bool    `json:"isRecoveryFlowEnabled" yaml:"isRecoveryFlowEnabled"`
	SignOutFlowID             string  `json:"signOutFlowId,omitempty" yaml:"signOutFlowId,omitempty"`
	UserOnboardingFlowID      string  `json:"userOnboardingFlowId,omitempty" yaml:"userOnboardingFlowId,omitempty"`

	// Branding URIs, validated as absolute URLs on the management API.
	LogoURL         string `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty" native:"omitempty,url,max=2048"`
	TosURI          string `json:"tosUri,omitempty" yaml:"tosUri,omitempty" native:"omitempty,url,max=2048"`
	PolicyURI       string `json:"policyUri,omitempty" yaml:"policyUri,omitempty" native:"omitempty,url,max=2048"`
	CookiePolicyURI string `json:"cookiePolicyUri,omitempty" yaml:"cookiePolicyUri,omitempty" native:"url,max=2048"`
}

// OrganizationUnitListResponse represents the response for listing organization units with pagination.
type OrganizationUnitListResponse struct {
	TotalResults      int                     `json:"totalResults"`
	StartIndex        int                     `json:"startIndex"`
	Count             int                     `json:"count"`
	OrganizationUnits []OrganizationUnitBasic `json:"organizationUnits"`
	Links             []utils.Link            `json:"links"`
}

// OrganizationUnitBasic represents the basic information of an organization unit.
type OrganizationUnitBasic struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	LogoURL     string    `json:"logoUrl,omitempty"`
	IsReadOnly  bool      `json:"isReadOnly"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// User represents a user with basic information for OU endpoints.
type User struct {
	ID      string `json:"id"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

// Group represents a group with basic information for OU endpoints.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UserListResponse represents the response for listing users in an organization unit.
type UserListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Users        []User       `json:"users"`
	Links        []utils.Link `json:"links"`
}

// OUUserResolver provides access to user data for an organization unit
// without requiring direct import of the user package.
type OUUserResolver interface {
	GetUserCountByOUID(ctx context.Context, ouID string) (int, error)
	GetUserListByOUID(ctx context.Context, ouID string, limit, offset int, includeDisplay bool) ([]User, error)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
}

// OUGroupResolver provides access to group data for an organization unit
// without requiring direct import of the group package.
type OUGroupResolver interface {
	GetGroupCountByOUID(ctx context.Context, ouID string) (int, error)
	GetGroupListByOUID(ctx context.Context, ouID string, limit, offset int) ([]Group, error)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
}

// GroupListResponse represents the response for listing groups in an organization unit.
type GroupListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Groups       []Group      `json:"groups"`
	Links        []utils.Link `json:"links"`
}

// Role represents a role with basic information for OU endpoints.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// RoleListResponse represents the response for listing roles in an organization unit.
type RoleListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Roles        []Role       `json:"roles"`
	Links        []utils.Link `json:"links"`
}

// OURoleResolver provides access to role data for an organization unit
// without requiring direct import of the role package.
type OURoleResolver interface {
	GetRoleCountByOUID(ctx context.Context, ouID string) (int, error)
	GetRoleListByOUID(ctx context.Context, ouID string, limit, offset int) ([]Role, error)
}
