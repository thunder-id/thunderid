// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"github.com/thunder-id/thunderid/internal/sharing"
)

// SharingPolicyRequest is the body of a sharing-policy create or update.
type SharingPolicyRequest struct {
	// InitiatingOuID is the organization unit making the decision; omit for the server's owner.
	InitiatingOuID string `json:"initiatingOuId,omitempty"`
	// TargetOuScope selects which organization units the policy reaches.
	TargetOuScope sharing.TargetOUScope `json:"targetOuScope"`
	// OverlayRules are the terms, keyed by field. A resource server exposes one field, resources.
	OverlayRules map[string]sharing.OverlayRule `json:"overlayRules,omitempty"`
	// Version is the policy version the caller believes it is editing, for clients that cannot set
	// If-Match. Ignored on create.
	Version int `json:"version,omitempty"`
}

// toServiceRequest converts the API body into the framework's request shape.
func (r SharingPolicyRequest) toServiceRequest() sharing.PolicyRequest {
	return sharing.PolicyRequest{
		InitiatingOUID: r.InitiatingOuID,
		TargetOUScope:  r.TargetOuScope,
		OverlayRules:   r.OverlayRules,
		Version:        r.Version,
	}
}

// SharingPolicyListResponse is the body of a sharing-policy listing.
type SharingPolicyListResponse struct {
	// TotalResults is how many policies the resource server has.
	TotalResults int `json:"totalResults"`
	// Policies are the policies themselves, declared ones included.
	Policies []SharingPolicyResponse `json:"policies"`
}

// SharingPolicyResponse is one sharing policy as the API reports it.
type SharingPolicyResponse struct {
	// ID identifies the policy.
	ID string `json:"id"`
	// ResourceType is always resource_server here, and is reported so a client handling several
	// shareable types can treat the bodies uniformly.
	ResourceType string `json:"resourceType"`
	// ResourceID is the resource server the policy governs.
	ResourceID string `json:"resourceId"`
	// OwningOuID owns the resource server.
	OwningOuID string `json:"owningOuId"`
	// InitiatingOuID made this decision.
	InitiatingOuID string `json:"initiatingOuId"`
	// Stage records whether the owner or a sharee issued the policy.
	Stage string `json:"stage"`
	// TargetOuScope is the selection the policy reaches, in the shape a create takes.
	TargetOuScope sharing.TargetOUScope `json:"targetOuScope"`
	// OverlayRules are the resolved rules, keyed by field.
	OverlayRules map[string]sharing.OverlayRule `json:"overlayRules,omitempty"`
	// Declared marks a policy a resource file defines and that has not been edited since. Editing
	// one stores it and clears this; deleting the stored row reverts to what the file declares.
	Declared bool `json:"declared"`
	// Version is the value to send back in If-Match when editing.
	Version int `json:"version"`
}

// SharingOverlayResponse is the resolved rule set for one organization unit.
type SharingOverlayResponse struct {
	// OuID is the organization unit the rules were resolved for.
	OuID string `json:"ouId"`
	// Origin distinguishes owning the resource server from being shared it. An organization unit
	// that is neither gets no overlay at all: the endpoint reports the server as not found.
	Origin string `json:"origin"`
	// PolicyIDs names every policy that contributed, so an intersection is traceable to its inputs.
	PolicyIDs []string `json:"policyIds"`
	// Rules is the effective rule per field.
	Rules map[string]SharingOverlayRuleResponse `json:"rules"`
}

// Origins reported for one organization unit's hold on a resource server.
const (
	// SharingOriginOwned marks an organization unit that owns the resource server.
	SharingOriginOwned = "owned"
	// SharingOriginShared marks one a sharing policy reaches.
	SharingOriginShared = "shared"
)

// SharingOverlayRuleResponse is one resolved rule, carrying where it came from.
type SharingOverlayRuleResponse struct {
	// Editable reports whether the organization unit may write the field.
	Editable bool `json:"editable"`
	// Value is what it holds, or is pinned to. Absent means the owner's own value.
	Value *[]string `json:"value,omitempty"`
	// AllowedValues bounds what it may choose from.
	AllowedValues *[]string `json:"allowedValues,omitempty"`
	// ExcludedValues is subtracted from both.
	ExcludedValues *[]string `json:"excludedValues,omitempty"`
	// Source separates a rule some policy named from the resource type's declared default, which is
	// the difference between "the owner decided this" and "nobody said anything".
	Source string `json:"source"`
}

// toSharingPolicyResponse reshapes a stored policy into the form a create would take, so a response
// can be edited and sent straight back as an update body.
func toSharingPolicyResponse(p sharing.Policy) SharingPolicyResponse {
	req := p.AsRequest()

	return SharingPolicyResponse{
		ID:             p.ID,
		ResourceType:   string(p.ResourceType),
		ResourceID:     p.ResourceID,
		OwningOuID:     p.OwningOUID,
		InitiatingOuID: p.InitiatingOUID,
		Stage:          string(p.Stage),
		TargetOuScope:  req.TargetOUScope,
		OverlayRules:   req.OverlayRules,
		Declared:       p.Declared,
		Version:        p.Version,
	}
}

// toSharingOverlayResponse reshapes a resolved overlay for the API.
func toSharingOverlayResponse(r sharing.ResolvedOverlay) SharingOverlayResponse {
	// Only a reached organization unit gets this far: one the resource never reached is reported as
	// not found rather than described.
	origin := SharingOriginShared
	if r.Owned {
		origin = SharingOriginOwned
	}

	rules := make(map[string]SharingOverlayRuleResponse, len(r.Rules))
	for key, rule := range r.Rules {
		rules[key] = SharingOverlayRuleResponse{
			Editable:       rule.Editable,
			Value:          rule.Value,
			AllowedValues:  rule.AllowedValues,
			ExcludedValues: rule.ExcludedValues,
			Source:         r.Sources[key],
		}
	}

	policyIDs := r.PolicyIDs
	if policyIDs == nil {
		policyIDs = []string{}
	}

	return SharingOverlayResponse{
		OuID:      r.OUID,
		Origin:    origin,
		PolicyIDs: policyIDs,
		Rules:     rules,
	}
}
