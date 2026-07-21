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

package consent

// PurposeElement represents an element reference within a consent purpose.
type PurposeElement struct {
	Name        string
	Namespace   Namespace
	IsMandatory bool
}

// ConsentPurpose represents a consent purpose managed in the system.
// A consent purpose groups consent elements under a single objective for a specific resource.
type ConsentPurpose struct {
	ID          string
	Name        string
	Description string
	GroupID     string // e.g. app id
	Elements    []PurposeElement
}

// Consent represents a consent record in the system, containing all relevant details and status.
type Consent struct {
	ID      string
	GroupID string // e.g. app id
	Status  ConsentStatus
	// ValidityTime is the Unix timestamp until which the consent is valid
	ValidityTime   int64
	Purposes       []ConsentPurposeItem
	Authorizations []ConsentAuthorization
}

// ConsentStatus defines the possible statuses for a consent record.
type ConsentStatus string

const (
	// ConsentStatusActive indicates that the consent is active and valid.
	ConsentStatusActive ConsentStatus = "ACTIVE"
	// ConsentStatusExpired indicates that the consent has expired after its validity time.
	ConsentStatusExpired ConsentStatus = "EXPIRED"
)

// IsValid reports whether the status is one of the known consent statuses.
func (s ConsentStatus) IsValid() bool {
	switch s {
	case ConsentStatusActive, ConsentStatusExpired:
		return true
	default:
		return false
	}
}

// ConsentPurposeItem represents an element approval record within a consent.
type ConsentPurposeItem struct {
	Name     string                   `json:"name"`
	Elements []ConsentElementApproval `json:"elements"`
}

// ConsentElementApproval represents a user's approval decision for a specific element.
type ConsentElementApproval struct {
	Name           string    `json:"name"`
	Namespace      Namespace `json:"namespace"`
	IsUserApproved bool      `json:"isUserApproved"`
}

// Namespace represents the consent namespace that classifies a consent element.
type Namespace string

const (
	// NamespaceAttribute represents the attribute consent namespace.
	// Used for managing consent over user attributes (e.g. email, mobile).
	NamespaceAttribute Namespace = "attribute"
	// NamespacePermission represents the permission consent namespace.
	// Used for managing consent over resource action permissions (e.g. booking:reservations:read).
	NamespacePermission Namespace = "permission"
)

// IsValid reports whether the namespace is one of the known consent namespaces.
func (n Namespace) IsValid() bool {
	switch n {
	case NamespaceAttribute, NamespacePermission:
		return true
	default:
		return false
	}
}

// ConsentAuthorization represents the authorization record within a consent.
type ConsentAuthorization struct {
	ID     string
	UserID string
	Type   ConsentAuthorizationType
	Status ConsentAuthorizationStatus
	// UpdatedTime is the Unix timestamp of the last status change
	UpdatedTime int64
}

// ConsentAuthorizationType defines the possible types for a consent authorization record.
type ConsentAuthorizationType string

const (
	// AuthorizationTypeAuthorization represents a standard user authorization action for a consent.
	AuthorizationTypeAuthorization ConsentAuthorizationType = "AUTHORIZATION"
	// AuthorizationTypeReAuthorization represents a re-authorization action for a consent.
	AuthorizationTypeReAuthorization ConsentAuthorizationType = "RE_AUTHORIZATION"
)

// IsValid reports whether the type is one of the known consent authorization types.
func (t ConsentAuthorizationType) IsValid() bool {
	switch t {
	case AuthorizationTypeAuthorization, AuthorizationTypeReAuthorization:
		return true
	default:
		return false
	}
}

// ConsentAuthorizationStatus defines the possible statuses for a consent authorization record.
type ConsentAuthorizationStatus string

const (
	// AuthorizationStatusCreated indicates that the authorization record has been created,
	// but not yet approved or rejected.
	AuthorizationStatusCreated ConsentAuthorizationStatus = "CREATED"
	// AuthorizationStatusApproved indicates that the authorization record has been approved by the user.
	AuthorizationStatusApproved ConsentAuthorizationStatus = "APPROVED"
	// AuthorizationStatusRejected indicates that the authorization record has been rejected by the user.
	AuthorizationStatusRejected ConsentAuthorizationStatus = "REJECTED"
)

// IsValid reports whether the status is one of the known consent authorization statuses.
func (s ConsentAuthorizationStatus) IsValid() bool {
	switch s {
	case AuthorizationStatusCreated, AuthorizationStatusApproved, AuthorizationStatusRejected:
		return true
	default:
		return false
	}
}

// PurposeFilter defines the search criteria for querying consent purposes.
type PurposeFilter struct {
	GroupID string // e.g. app id
}

// ConsentFilter defines the search criteria for querying consent records.
type ConsentFilter struct {
	ConsentStatus ConsentStatus
	GroupID       string // e.g. app id
	UserID        string
}

// ConsentRequest represents the payload for creating a new consent record.
type ConsentRequest struct {
	GroupID string // e.g. app id
	// ValidityTime is the Unix timestamp until which the consent is valid
	ValidityTime   int64
	Purposes       []ConsentPurposeItem
	Authorizations []ConsentAuthorizationRequest
}

// ConsentAuthorizationRequest represents the authorization payload within a consent creation request.
type ConsentAuthorizationRequest struct {
	UserID string
	Type   ConsentAuthorizationType
	Status ConsentAuthorizationStatus
}
