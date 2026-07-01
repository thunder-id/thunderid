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

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// ----- Consent element data models -----

// ConsentElementInput represents the input struct for creating a consent element.
// A consent element is the most granular unit — a specific data point (e.g. email).
type ConsentElementInput struct {
	// Name is the unique name of the consent element within the ou
	Name string
	// Description is a human-readable description of the element
	Description string
	// Namespace is the consent namespace to which this element belongs (e.g. "attribute")
	Namespace providers.Namespace
	// Properties is an optional map of additional element properties
	Properties map[string]string
}

// ConsentElement represents a consent element managed in the system.
// A consent element is the most granular unit — a specific data point (e.g. email).
type ConsentElement struct {
	// ID is the unique identifier of the consent element
	ID string
	// Name is the unique name of the consent element within the organization
	Name string
	// Description is a human-readable description of the element
	Description string
	// Namespace is the consent namespace to which this element belongs (e.g. "attribute")
	Namespace providers.Namespace
	// Properties is an optional map of additional element properties
	Properties map[string]string
}

// ----- Consent purpose data models -----

// PurposeElement represents an element reference within a consent purpose.
type PurposeElement struct {
	// Name is the consent element name
	Name string
	// Namespace is the consent namespace to which this element belongs (e.g. "attribute")
	Namespace providers.Namespace
	// IsMandatory indicates whether user approval for this element is mandatory
	IsMandatory bool
}

// ConsentPurposeInput represents the input struct for creating or updating a consent purpose.
// A consent purpose groups consent elements under a single objective for a specific resource.
type ConsentPurposeInput struct {
	// Name is the unique name of the purpose
	Name string
	// Description is a human-readable description of the purpose
	Description string
	// GroupID is the group ID that owns this purpose (e.g. app id)
	GroupID string
	// Namespace is the consent namespace to which this purpose belongs (e.g. "attribute")
	Namespace providers.Namespace
	// Elements is the list of consent elements belonging to this purpose
	Elements []PurposeElement
}

// ConsentPurpose represents a consent purpose managed in the system.
// A consent purpose groups consent elements under a single objective for a specific resource.
type ConsentPurpose struct {
	// ID is the unique identifier of the consent purpose
	ID string
	// Name is the unique name of the purpose
	Name string
	// Description is a human-readable description of the purpose
	Description string
	// GroupID is the group ID that owns this purpose (e.g. app id)
	GroupID string
	// Namespace is the consent namespace to which this purpose belongs (e.g. "attribute")
	Namespace providers.Namespace
	// Elements is the list of consent elements belonging to this purpose
	Elements []PurposeElement
	// CreatedTime is the Unix timestamp when the purpose was created
	CreatedTime int64
	// UpdatedTime is the Unix timestamp when the purpose was last updated
	UpdatedTime int64
}

// ----- Consent record data models -----

// ConsentAuthorizationRequest represents the authorization payload within a consent creation request.
type ConsentAuthorizationRequest struct {
	// UserID is the identifier of the user who performed the authorization
	UserID string
	// Type is the authorization type (e.g. "authorization")
	Type providers.ConsentAuthorizationType
	// Status is the authorization status (e.g. "APPROVED")
	Status providers.ConsentAuthorizationStatus
}

// ConsentRequest represents the payload for creating a new consent record.
type ConsentRequest struct {
	// Type is the consent type (e.g. "authentication")
	Type providers.ConsentType
	// GroupID is the group ID that this consent is associated with (e.g. app id)
	GroupID string
	// ValidityTime is the Unix timestamp until which the consent is valid
	ValidityTime int64
	// Purposes is the list of purposes with element approval decisions
	Purposes []providers.ConsentPurposeItem
	// Authorizations is the list of authorization records to attach
	Authorizations []ConsentAuthorizationRequest
}

// ConsentSearchFilter defines the search criteria for querying consent records.
type ConsentSearchFilter struct {
	// ConsentTypes is an optional list of consent types to filter by
	ConsentTypes []providers.ConsentType
	// ConsentStatuses is an optional list of consent statuses to filter by
	ConsentStatuses []providers.ConsentStatus
	// GroupIDs is an optional list of group IDs to filter by
	GroupIDs []string
	// UserIDs is an optional list of user IDs to filter by
	UserIDs []string
	// PurposeName is an optional purpose name to filter by
	PurposeName string
	// Limit is the maximum number of results to return
	Limit int
	// Offset is the number of results to skip
	Offset int
}

// ConsentValidationResult represents the result of a consent validation check.
type ConsentValidationResult struct {
	// IsValid indicates whether the consent is valid
	IsValid bool
	// ConsentInformation contains the full consent details if valid
	ConsentInformation *providers.Consent
}

// ConsentRevokeRequest represents the request for revoking a consent.
type ConsentRevokeRequest struct {
	// Reason is an optional human-readable reason for the revocation
	Reason string
}
