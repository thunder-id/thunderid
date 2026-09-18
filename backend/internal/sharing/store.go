// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

var getDBProvider = provider.GetDBProvider

// Member roles distinguish what a stored member row contributes to its rule. The requested set
// exists so re-materializing against a changed ancestor starts from the original ask.
const (
	memberValue             = "value"
	memberAllowed           = "allowed"
	memberExcluded          = "excluded"
	memberRequestedValue    = "requestedValue"
	memberRequestedAllowed  = "requestedAllowed"
	memberRequestedExcluded = "requestedExcluded"
)

// storeInterface is the only place SQL lives. Every method is scoped to one deployment.
type storeInterface interface {
	// CreatePolicy inserts a policy with its targets, exclusions and rules.
	CreatePolicy(ctx context.Context, p Policy) error
	// GetPolicy returns one policy by id, fully hydrated.
	GetPolicy(ctx context.Context, id string) (Policy, error)
	// GetPolicyByInitiator returns the one policy an organization unit holds for a resource.
	GetPolicyByInitiator(ctx context.Context, rt ResourceType, resourceID, initiatingOUID string) (Policy, error)
	// ListPoliciesForResource returns every policy recorded for one resource.
	ListPoliciesForResource(ctx context.Context, rt ResourceType, resourceID string) ([]Policy, error)
	// ListPoliciesRelevantToChain returns the policies that could cover any organization unit in
	// the chain. Coverage itself is decided in memory.
	ListPoliciesRelevantToChain(ctx context.Context, rt ResourceType, chainOUIDs []string) ([]Policy, error)
	// ReplacePolicyContents rewrites a policy's targets, exclusions and rules, bumping its version
	// only when expectedVersion still matches.
	ReplacePolicyContents(ctx context.Context, p Policy, expectedVersion int) error
	// DeletePolicy removes a policy and everything hanging off it.
	DeletePolicy(ctx context.Context, id string) error

	// GetOverlayValues returns one organization unit's own values for a resource, by field.
	GetOverlayValues(ctx context.Context, rt ResourceType, resourceID, ouID string) (map[string][]string, error)
	// SetOverlayValue records one organization unit's value for one field.
	SetOverlayValue(ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string, value []string) error
	// DeleteOverlayValue removes one organization unit's value for one field.
	DeleteOverlayValue(ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string) error
}

// sharingStore is the database-backed implementation of storeInterface.
type sharingStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newSharingStore creates the database store and the transactioner spanning its writes.
func newSharingStore() (storeInterface, providers.Transactioner, error) {
	dbProvider := getDBProvider()
	client, err := dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, nil, err
	}
	transactioner, err := client.GetTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &sharingStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

func (s *sharingStore) client() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

// CreatePolicy inserts a policy with its targets, exclusions and rules.
func (s *sharingStore) CreatePolicy(ctx context.Context, p Policy) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreatePolicy,
		p.ID, string(p.ResourceType), p.ResourceID, p.OwningOUID, p.InitiatingOUID,
		string(p.Stage), nullableString(p.ParentPolicyID), p.Declared, maxInt(p.Version, 1),
		s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to create sharing policy: %w", err)
	}
	return s.writeContents(ctx, dbClient, p)
}

// writeContents inserts the targets, exclusions and rules belonging to a policy.
func (s *sharingStore) writeContents(ctx context.Context, dbClient provider.DBClientInterface, p Policy) error {
	for _, t := range p.Targets {
		if _, err := dbClient.ExecuteContext(ctx, queryInsertTarget,
			t.ID, p.ID, string(t.Scope), nullableString(t.OUID), s.deploymentID); err != nil {
			return fmt.Errorf("failed to create sharing policy target: %w", err)
		}
	}
	for _, ouID := range dedupeStrings(p.ExcludedOUIDs) {
		if _, err := dbClient.ExecuteContext(ctx, queryInsertExclusion,
			p.ID, ouID, s.deploymentID); err != nil {
			return fmt.Errorf("failed to create sharing policy exclusion: %w", err)
		}
	}
	for _, r := range p.Rules {
		if err := s.writeRule(ctx, dbClient, p.ID, r); err != nil {
			return err
		}
	}
	return nil
}

// writeRule inserts one overlay rule and its members, keeping the absent-versus-empty distinction
// in the set flags rather than inferring it from the member count.
func (s *sharingStore) writeRule(
	ctx context.Context, dbClient provider.DBClientInterface, policyID string, r StoredRule,
) error {
	ruleID := newID()
	_, err := dbClient.ExecuteContext(ctx, queryInsertRule,
		ruleID, policyID, nullableString(r.TargetID), r.FieldKey,
		r.Resolved.Editable, r.Resolved.Value != nil, r.Resolved.AllowedValues != nil,
		r.Resolved.ExcludedValues != nil,
		r.Requested.Editable, r.Requested.Value != nil, r.Requested.AllowedValues != nil,
		r.Requested.ExcludedValues != nil,
		s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to create overlay rule: %w", err)
	}

	sets := []struct {
		role    string
		members *[]string
	}{
		{memberValue, r.Resolved.Value},
		{memberAllowed, r.Resolved.AllowedValues},
		{memberExcluded, r.Resolved.ExcludedValues},
		{memberRequestedValue, r.Requested.Value},
		{memberRequestedAllowed, r.Requested.AllowedValues},
		{memberRequestedExcluded, r.Requested.ExcludedValues},
	}
	for _, set := range sets {
		if set.members == nil {
			continue
		}
		for _, m := range dedupeStrings(*set.members) {
			if _, err := dbClient.ExecuteContext(ctx, queryInsertRuleMember,
				ruleID, set.role, m, s.deploymentID); err != nil {
				return fmt.Errorf("failed to create overlay rule member: %w", err)
			}
		}
	}
	return nil
}

// GetPolicy returns one policy by id, fully hydrated.
func (s *sharingStore) GetPolicy(ctx context.Context, id string) (Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return Policy{}, err
	}
	rows, err := dbClient.QueryContext(ctx, queryGetPolicyByID, id, s.deploymentID)
	if err != nil {
		return Policy{}, fmt.Errorf("failed to get sharing policy: %w", err)
	}
	if len(rows) == 0 {
		return Policy{}, ErrPolicyNotFound
	}
	return s.hydrate(ctx, dbClient, policyFromRow(rows[0]))
}

// GetPolicyByInitiator returns the one policy an organization unit holds for a resource.
func (s *sharingStore) GetPolicyByInitiator(
	ctx context.Context, rt ResourceType, resourceID, initiatingOUID string,
) (Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return Policy{}, err
	}
	rows, err := dbClient.QueryContext(ctx, queryGetPolicyByInitiator,
		string(rt), resourceID, initiatingOUID, s.deploymentID)
	if err != nil {
		return Policy{}, fmt.Errorf("failed to get sharing policy by initiator: %w", err)
	}
	if len(rows) == 0 {
		return Policy{}, ErrPolicyNotFound
	}
	return s.hydrate(ctx, dbClient, policyFromRow(rows[0]))
}

// ListPoliciesForResource returns every policy recorded for one resource.
func (s *sharingStore) ListPoliciesForResource(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	rows, err := dbClient.QueryContext(ctx, queryListPoliciesForResource,
		string(rt), resourceID, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sharing policies: %w", err)
	}
	return s.hydrateAll(ctx, dbClient, rows)
}

// ListPoliciesRelevantToChain returns the policies that could cover any organization unit in the
// chain.
func (s *sharingStore) ListPoliciesRelevantToChain(
	ctx context.Context, rt ResourceType, chainOUIDs []string,
) ([]Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	query, args := buildRelevantPoliciesQuery(rt, chainOUIDs, s.deploymentID)
	rows, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies relevant to chain: %w", err)
	}
	return s.hydrateAll(ctx, dbClient, rows)
}

// ReplacePolicyContents rewrites a policy's contents, bumping its version only when the caller's
// expected version still matches.
func (s *sharingStore) ReplacePolicyContents(ctx context.Context, p Policy, expectedVersion int) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	affected, err := dbClient.ExecuteContext(ctx, queryBumpPolicyVersion, p.ID, expectedVersion, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update sharing policy version: %w", err)
	}
	if affected == 0 {
		return ErrPolicyNotFound
	}

	if _, err := dbClient.ExecuteContext(ctx, queryDeleteRules, p.ID, s.deploymentID); err != nil {
		return fmt.Errorf("failed to clear overlay rules: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteExclusions, p.ID, s.deploymentID); err != nil {
		return fmt.Errorf("failed to clear sharing policy exclusions: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteTargets, p.ID, s.deploymentID); err != nil {
		return fmt.Errorf("failed to clear sharing policy targets: %w", err)
	}
	return s.writeContents(ctx, dbClient, p)
}

// DeletePolicy removes a policy; its targets, exclusions, rules and members cascade with it.
func (s *sharingStore) DeletePolicy(ctx context.Context, id string) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeletePolicy, id, s.deploymentID); err != nil {
		return fmt.Errorf("failed to delete sharing policy: %w", err)
	}
	return nil
}

// GetOverlayValues returns one organization unit's own values for a resource, by field.
func (s *sharingStore) GetOverlayValues(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (map[string][]string, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	rows, err := dbClient.QueryContext(ctx, queryGetOverlayValue,
		string(rt), resourceID, ouID, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay values: %w", err)
	}
	out := make(map[string][]string, len(rows))
	for _, row := range rows {
		var members []string
		if err := json.Unmarshal([]byte(stringField(row["value"])), &members); err != nil {
			return nil, fmt.Errorf("failed to decode overlay value: %w", err)
		}
		out[stringField(row["field_key"])] = members
	}
	return out, nil
}

// SetOverlayValue records one organization unit's value for one field.
func (s *sharingStore) SetOverlayValue(
	ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string, value []string,
) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode overlay value: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryUpsertOverlayValue,
		string(rt), resourceID, ouID, fieldKey, string(encoded), s.deploymentID); err != nil {
		return fmt.Errorf("failed to set overlay value: %w", err)
	}
	return nil
}

// DeleteOverlayValue removes one organization unit's value for one field.
func (s *sharingStore) DeleteOverlayValue(
	ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteOverlayValue,
		string(rt), resourceID, ouID, fieldKey, s.deploymentID); err != nil {
		return fmt.Errorf("failed to delete overlay value: %w", err)
	}
	return nil
}

// hydrateAll loads the targets, exclusions and rules for every policy row.
func (s *sharingStore) hydrateAll(
	ctx context.Context, dbClient provider.DBClientInterface, rows []map[string]interface{},
) ([]Policy, error) {
	out := make([]Policy, 0, len(rows))
	for _, row := range rows {
		p, err := s.hydrate(ctx, dbClient, policyFromRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// hydrate loads the targets, exclusions and rules belonging to one policy.
func (s *sharingStore) hydrate(
	ctx context.Context, dbClient provider.DBClientInterface, p Policy,
) (Policy, error) {
	targetRows, err := dbClient.QueryContext(ctx, queryListTargets, p.ID, s.deploymentID)
	if err != nil {
		return Policy{}, fmt.Errorf("failed to list sharing policy targets: %w", err)
	}
	for _, row := range targetRows {
		p.Targets = append(p.Targets, Target{
			ID:    stringField(row["id"]),
			Scope: TargetScope(stringField(row["target_scope"])),
			OUID:  stringField(row["target_ou_id"]),
		})
	}

	exclusionRows, err := dbClient.QueryContext(ctx, queryListExclusions, p.ID, s.deploymentID)
	if err != nil {
		return Policy{}, fmt.Errorf("failed to list sharing policy exclusions: %w", err)
	}
	for _, row := range exclusionRows {
		p.ExcludedOUIDs = append(p.ExcludedOUIDs, stringField(row["excluded_ou_id"]))
	}

	ruleRows, err := dbClient.QueryContext(ctx, queryListRules, p.ID, s.deploymentID)
	if err != nil {
		return Policy{}, fmt.Errorf("failed to list overlay rules: %w", err)
	}
	for _, row := range ruleRows {
		rule, err := s.hydrateRule(ctx, dbClient, row)
		if err != nil {
			return Policy{}, err
		}
		p.Rules = append(p.Rules, rule)
	}
	return p, nil
}

// hydrateRule rebuilds one rule, restoring absent-versus-empty from the stored set flags.
func (s *sharingStore) hydrateRule(
	ctx context.Context, dbClient provider.DBClientInterface, row map[string]interface{},
) (StoredRule, error) {
	ruleID := stringField(row["id"])
	memberRows, err := dbClient.QueryContext(ctx, queryListRuleMembers, ruleID, s.deploymentID)
	if err != nil {
		return StoredRule{}, fmt.Errorf("failed to list overlay rule members: %w", err)
	}
	byRole := make(map[string][]string)
	for _, m := range memberRows {
		role := stringField(m["member_role"])
		byRole[role] = append(byRole[role], stringField(m["member_key"]))
	}

	members := func(role string, present bool) *[]string {
		if !present {
			return nil
		}
		v := byRole[role]
		if v == nil {
			v = []string{}
		}
		return &v
	}

	return StoredRule{
		FieldKey: stringField(row["field_key"]),
		TargetID: stringField(row["target_id"]),
		Resolved: OverlayRule{
			Editable:       boolField(row["editable"]),
			Value:          members(memberValue, boolField(row["value_set"])),
			AllowedValues:  members(memberAllowed, boolField(row["allowed_set"])),
			ExcludedValues: members(memberExcluded, boolField(row["excluded_set"])),
		},
		Requested: OverlayRule{
			Editable:       boolField(row["requested_editable"]),
			Value:          members(memberRequestedValue, boolField(row["req_value_set"])),
			AllowedValues:  members(memberRequestedAllowed, boolField(row["req_allowed_set"])),
			ExcludedValues: members(memberRequestedExcluded, boolField(row["req_excluded_set"])),
		},
	}, nil
}

// policyFromRow builds the policy row itself, leaving its contents to hydrate.
func policyFromRow(row map[string]interface{}) Policy {
	return Policy{
		ID:             stringField(row["id"]),
		ResourceType:   ResourceType(stringField(row["resource_type"])),
		ResourceID:     stringField(row["resource_id"]),
		OwningOUID:     stringField(row["owning_ou_id"]),
		InitiatingOUID: stringField(row["initiating_ou_id"]),
		Stage:          PolicyStage(stringField(row["policy_stage"])),
		ParentPolicyID: stringField(row["parent_policy_id"]),
		Declared:       boolField(row["declared"]),
		Version:        intField(row["version"]),
	}
}
