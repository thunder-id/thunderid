// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package consent holds integration tests for runtime consent collection, covering the consent
// executor flow node and the consent service backing it.
package consent

import (
	"encoding/json"
	"fmt"
)

const (
	// consentPurposeTypeAttributes is the prompt type discriminator for attribute consent purposes.
	consentPurposeTypeAttributes = "attributes"
	// consentPurposeTypePermissions is the prompt type discriminator for permission consent purposes.
	consentPurposeTypePermissions = "permissions"
)

// consentPurposePrompt mirrors the server's ConsentPurposePrompt wire shape (the "consentPrompt"
// additional-data entry emitted by the ConsentExecutor when a consent decision is needed).
type consentPurposePrompt struct {
	PurposeName string                 `json:"purposeName"`
	PurposeID   string                 `json:"purposeId"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Essential   []consentPromptElement `json:"essential"`
	Optional    []consentPromptElement `json:"optional"`
}

type consentPromptElement struct {
	Name   string `json:"name"`
	Parent string `json:"parent,omitempty"`
}

// consentDecisions mirrors the server's ConsentDecisions wire shape, submitted back as the
// "consent_decisions" flow input.
type consentDecisions struct {
	Approved bool                     `json:"approved"`
	Purposes []consentPurposeDecision `json:"purposes"`
}

type consentPurposeDecision struct {
	PurposeName string                   `json:"purposeName"`
	Approved    bool                     `json:"approved"`
	Elements    []consentElementDecision `json:"elements"`
}

type consentElementDecision struct {
	Name     string `json:"name"`
	Approved bool   `json:"approved"`
}

// parseAttributePurpose decodes serialized consent prompt data and returns the single "attributes"
// purpose it is expected to contain.
func parseAttributePurpose(rawPrompt string) (consentPurposePrompt, error) {
	return parsePurposeOfType(rawPrompt, consentPurposeTypeAttributes)
}

// parsePermissionPurpose decodes serialized consent prompt data and returns the single "permissions"
// purpose it is expected to contain.
func parsePermissionPurpose(rawPrompt string) (consentPurposePrompt, error) {
	return parsePurposeOfType(rawPrompt, consentPurposeTypePermissions)
}

// parsePurposeOfType decodes serialized consent prompt data and returns the single purpose carrying
// the given type discriminator.
func parsePurposeOfType(rawPrompt, purposeType string) (consentPurposePrompt, error) {
	purposes, err := parsePromptPurposes(rawPrompt)
	if err != nil {
		return consentPurposePrompt{}, err
	}

	for _, p := range purposes {
		if p.Type == purposeType {
			return p, nil
		}
	}

	return consentPurposePrompt{}, fmt.Errorf("no %s consent purpose found in %v", purposeType, purposes)
}

// parsePromptPurposes decodes serialized consent prompt data into every purpose it carries.
func parsePromptPurposes(rawPrompt string) ([]consentPurposePrompt, error) {
	var purposes []consentPurposePrompt
	if err := json.Unmarshal([]byte(rawPrompt), &purposes); err != nil {
		return nil, fmt.Errorf("failed to parse consent prompt data: %w", err)
	}

	return purposes, nil
}

// buildConsentDecisionsInput builds the JSON payload for the "consent_decisions" flow input,
// approving or denying every element of the given purpose.
func buildConsentDecisionsInput(purpose consentPurposePrompt, approve bool) (string, error) {
	return buildConsentDecisionsInputFor(purpose, approve, approve)
}

// buildConsentDecisionsInputFor builds the "consent_decisions" payload with separate decisions for
// the purpose's essential and optional elements, so tests can mix grants and denials.
func buildConsentDecisionsInputFor(
	purpose consentPurposePrompt, approveEssential, approveOptional bool,
) (string, error) {
	elements := make([]consentElementDecision, 0, len(purpose.Essential)+len(purpose.Optional))
	for _, e := range purpose.Essential {
		elements = append(elements, consentElementDecision{Name: e.Name, Approved: approveEssential})
	}
	for _, e := range purpose.Optional {
		elements = append(elements, consentElementDecision{Name: e.Name, Approved: approveOptional})
	}

	// The purpose and the submission as a whole are approved whenever at least one element is, since
	// a denial at a higher level is pushed down onto every element beneath it.
	anyApproved := approveEssential || approveOptional
	decisions := consentDecisions{
		Approved: anyApproved,
		Purposes: []consentPurposeDecision{
			{PurposeName: purpose.PurposeName, Approved: anyApproved, Elements: elements},
		},
	}

	return marshalConsentDecisions(decisions)
}

// marshalConsentDecisions serializes a decisions payload for the "consent_decisions" flow input,
// so tests can submit shapes the prompt did not ask for.
func marshalConsentDecisions(decisions consentDecisions) (string, error) {
	decisionsJSON, err := json.Marshal(decisions)
	if err != nil {
		return "", err
	}

	return string(decisionsJSON), nil
}

func promptElementNames(elements []consentPromptElement) []string {
	names := make([]string, 0, len(elements))
	for _, e := range elements {
		names = append(names, e.Name)
	}

	return names
}

// promptElementParent returns the rollup parent the prompt reported for the named element.
func promptElementParent(elements []consentPromptElement, name string) (string, bool) {
	for _, e := range elements {
		if e.Name == name {
			return e.Parent, true
		}
	}

	return "", false
}
