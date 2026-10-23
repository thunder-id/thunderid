// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailexec

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/thunder-id/thunderid/internal/authzdetail/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// processorRequest is the batched body posted to a processor endpoint, named after AuthZEN.
type processorRequest struct {
	Subject  processorEntity   `json:"subject"`
	Client   processorEntity   `json:"client"`
	Resource string            `json:"resource"`
	Details  []processorDetail `json:"details"`
}

// processorEntity identifies the subject or the client of a processor request.
type processorEntity struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// requesterEntity identifies a client as an application or an agent.
func requesterEntity(category providers.EntityCategory, clientID string) processorEntity {
	entityType := oauth2const.SubTypeApp
	if category == providers.EntityCategoryAgent {
		entityType = oauth2const.SubTypeAgent
	}
	return processorEntity{Type: entityType, ID: clientID}
}

// subjectEntity identifies the subject of a request as a user, an agent, or an application.
func subjectEntity(subject providers.EntityReference) processorEntity {
	entityType := string(providers.EntityCategoryUser)
	switch subject.EntityCategory {
	case string(providers.EntityCategoryAgent):
		entityType = oauth2const.SubTypeAgent
	case string(providers.EntityCategoryApp):
		entityType = oauth2const.SubTypeApp
	}
	return processorEntity{Type: entityType, ID: subject.EntityID}
}

// processorDetail is one detail of a processor request, with the identifier its result refers back to.
type processorDetail struct {
	DetailID    string          `json:"detailId"`
	Type        string          `json:"type"`
	TypeVersion int             `json:"typeVersion"`
	Detail      json.RawMessage `json:"detail"`
}

// processorResponse is a processor's answer, one result per detail of the request.
type processorResponse struct {
	Results []processorResult `json:"results"`
}

// processorResult is a processor's decision on one detail; patch keys are JSON Pointers.
type processorResult struct {
	DetailID string                     `json:"detailId"`
	Decision *bool                      `json:"decision"`
	Patch    map[string]json.RawMessage `json:"patch,omitempty"`
	Context  processorResultContext     `json:"context,omitempty"`
}

// processorResultContext carries the reasons for a decision; reason_admin is only logged.
type processorResultContext struct {
	ReasonUser  string `json:"reason_user,omitempty"`
	ReasonAdmin string `json:"reason_admin,omitempty"`
}

// resolvedTypes holds a request's registered types with their compiled schemas and policies.
type resolvedTypes struct {
	byValue  map[string]providers.AuthorizationDetailType
	schemas  map[string]*jsonschema.Resolved
	policies map[string]common.FieldPolicies
}
