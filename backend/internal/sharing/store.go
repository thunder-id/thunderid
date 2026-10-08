// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

var getDBProvider = provider.GetDBProvider

// sharingPolicyStoreInterface is the only place SQL lives. Every method is scoped to one deployment.
type sharingPolicyStoreInterface interface {
	// CreatePolicy inserts a policy with its targets, exclusions and rules.
	CreatePolicy(ctx context.Context, p Policy) error
	// GetPolicy returns one policy by id, fully hydrated.
	GetPolicy(ctx context.Context, id string) (Policy, error)
	// GetPolicyByInitiator returns the one policy an organization unit holds for a resource.
	GetPolicyByInitiator(ctx context.Context, rt ResourceType, resourceID, initiatingOUID string) (Policy, error)
	// ListPoliciesForResource returns one page of the policies recorded for one resource, for a
	// management API to serve.
	ListPoliciesForResource(
		ctx context.Context, rt ResourceType, resourceID string, limit, offset int,
	) ([]Policy, error)
	// CountPoliciesForResource returns how many policies one resource has, for that listing's total.
	CountPoliciesForResource(ctx context.Context, rt ResourceType, resourceID string) (int, error)
	// ListAllPoliciesForResource returns every policy recorded for one resource, with no bound.
	//
	// Policy evaluation reads through here, and every question it asks is about the set as a whole:
	// whether exactly one policy covers an organization unit, and what coverage looked like before
	// and after an edit. A policy left out is not a shorter answer but a different one, quietly
	// narrowing who can see a resource, so this read takes neither a page nor the composite store's
	// record cap.
	ListAllPoliciesForResource(ctx context.Context, rt ResourceType, resourceID string) ([]Policy, error)
	// ListPoliciesRelevantToChain returns the policies that could cover any organization unit in
	// the chain. Coverage itself is decided in memory. An empty resourceID spans every resource of
	// the type, which is what the reverse lookup needs.
	ListPoliciesRelevantToChain(
		ctx context.Context, rt ResourceType, resourceID string, chainOUIDs []string,
	) ([]Policy, error)
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
	// DeleteOverlayValuesForOU removes every value one organization unit holds for a resource.
	DeleteOverlayValuesForOU(ctx context.Context, rt ResourceType, resourceID, ouID string) error
}

// sharingStore is the database-backed implementation of sharingPolicyStoreInterface.
type sharingStore struct {
	dbProvider provider.DBProviderInterface
}

// newSharingStore creates the database store and the transactioner spanning its writes.
func newSharingStore() (sharingPolicyStoreInterface, providers.Transactioner, error) {
	dbProvider := getDBProvider()
	client, err := dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, nil, err
	}
	transactioner, err := client.GetTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &sharingStore{dbProvider: dbProvider}, transactioner, nil
}

// scope resolves the deployment scope from the request context.
func (s *sharingStore) scope(ctx context.Context) string {
	return deployment.Resolve(ctx)
}

// client returns the configuration database client every query in this store runs against.
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
		string(p.Stage), nullableString(p.ParentPolicyID), max(p.Version, 1),
		s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to create sharing policy: %w", err)
	}
	return s.writeContents(ctx, dbClient, p)
}

// writeContents inserts the targets, exclusions and rules belonging to a policy.
func (s *sharingStore) writeContents(ctx context.Context, dbClient provider.DBClientInterface, p Policy) error {
	for _, t := range p.Targets {
		if _, err := dbClient.ExecuteContext(ctx, queryInsertTarget,
			t.ID, p.ID, string(t.Scope), nullableString(t.OUID), s.scope(ctx)); err != nil {
			return fmt.Errorf("failed to create sharing policy target: %w", err)
		}
	}
	for _, ouID := range utils.UniqueStrings(p.ExcludedOUIDs) {
		if _, err := dbClient.ExecuteContext(ctx, queryInsertExclusion,
			p.ID, ouID, s.scope(ctx)); err != nil {
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

// writeRule inserts one overlay rule, both forms of it encoded as documents.
//
// The sets are documents rather than rows per member because nothing is ever read by member: the
// only query is "give me this rule", and containment does every comparison in memory. A document
// also keeps absent apart from empty on its own, null against [], and bounds no single member.
func (s *sharingStore) writeRule(
	ctx context.Context, dbClient provider.DBClientInterface, policyID string, r StoredRule,
) error {
	resolved, err := json.Marshal(r.Resolved)
	if err != nil {
		return fmt.Errorf("failed to encode overlay rule: %w", err)
	}
	requested, err := json.Marshal(r.Requested)
	if err != nil {
		return fmt.Errorf("failed to encode requested overlay rule: %w", err)
	}
	ruleID, err := utils.GenerateUUIDv7()
	if err != nil {
		return fmt.Errorf("failed to generate an overlay rule identifier: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryInsertRule,
		ruleID, policyID, nullableString(r.TargetID), r.FieldKey,
		string(resolved), string(requested), s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to create overlay rule: %w", err)
	}
	return nil
}

// GetPolicy returns one policy by id, fully hydrated.
func (s *sharingStore) GetPolicy(ctx context.Context, id string) (Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return Policy{}, err
	}
	rows, err := dbClient.QueryContext(ctx, queryGetPolicyByID, id, s.scope(ctx))
	if err != nil {
		return Policy{}, fmt.Errorf("failed to get sharing policy: %w", err)
	}
	if len(rows) == 0 {
		return Policy{}, errPolicyNotFound
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
		string(rt), resourceID, initiatingOUID, s.scope(ctx))
	if err != nil {
		return Policy{}, fmt.Errorf("failed to get sharing policy by initiator: %w", err)
	}
	if len(rows) == 0 {
		return Policy{}, errPolicyNotFound
	}
	return s.hydrate(ctx, dbClient, policyFromRow(rows[0]))
}

// ListPoliciesForResource returns one page of the policies recorded for one resource.
func (s *sharingStore) ListPoliciesForResource(
	ctx context.Context, rt ResourceType, resourceID string, limit, offset int,
) ([]Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	rows, err := dbClient.QueryContext(ctx, queryListPoliciesForResource,
		string(rt), resourceID, s.scope(ctx), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sharing policies: %w", err)
	}
	return s.hydrateAll(ctx, dbClient, rows)
}

// ListAllPoliciesForResource returns every policy recorded for one resource.
func (s *sharingStore) ListAllPoliciesForResource(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	rows, err := dbClient.QueryContext(ctx, queryListAllPoliciesForResource,
		string(rt), resourceID, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list sharing policies: %w", err)
	}
	return s.hydrateAll(ctx, dbClient, rows)
}

// CountPoliciesForResource returns how many policies one resource has.
func (s *sharingStore) CountPoliciesForResource(
	ctx context.Context, rt ResourceType, resourceID string,
) (int, error) {
	dbClient, err := s.client()
	if err != nil {
		return 0, err
	}
	rows, err := dbClient.QueryContext(ctx, queryCountPoliciesForResource,
		string(rt), resourceID, s.scope(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to count sharing policies: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	// Read through ToInt64 rather than asserting int64: the drivers do not agree on the type a
	// COUNT comes back as.
	count, ok := utils.ToInt64(rows[0]["total"])
	if !ok {
		return 0, fmt.Errorf("failed to read sharing policy count from %v", rows[0]["total"])
	}
	return int(count), nil
}

// ListPoliciesRelevantToChain returns the policies that could cover any organization unit in the
// chain.
func (s *sharingStore) ListPoliciesRelevantToChain(
	ctx context.Context, rt ResourceType, resourceID string, chainOUIDs []string,
) ([]Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	query, args := buildRelevantPoliciesQuery(rt, resourceID, chainOUIDs, s.scope(ctx))
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
	affected, err := dbClient.ExecuteContext(ctx, queryBumpPolicyVersion, p.ID, expectedVersion, s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to update sharing policy version: %w", err)
	}
	if affected == 0 {
		return errPolicyNotFound
	}

	if _, err := dbClient.ExecuteContext(ctx, queryDeleteRules, p.ID, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to clear overlay rules: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteExclusions, p.ID, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to clear sharing policy exclusions: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteTargets, p.ID, s.scope(ctx)); err != nil {
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
	if _, err := dbClient.ExecuteContext(ctx, queryDeletePolicy, id, s.scope(ctx)); err != nil {
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
		string(rt), resourceID, ouID, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay values: %w", err)
	}
	out := make(map[string][]string, len(rows))
	for _, row := range rows {
		var members []string
		if err := json.Unmarshal([]byte(utils.ConvertInterfaceValueToString(row["value"])), &members); err != nil {
			return nil, fmt.Errorf("failed to decode overlay value: %w", err)
		}
		out[utils.ConvertInterfaceValueToString(row["field_key"])] = members
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
		string(rt), resourceID, ouID, fieldKey, string(encoded), s.scope(ctx)); err != nil {
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
		string(rt), resourceID, ouID, fieldKey, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete overlay value: %w", err)
	}
	return nil
}

// DeleteOverlayValuesForOU removes every value one organization unit holds for a resource.
func (s *sharingStore) DeleteOverlayValuesForOU(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteOverlayValuesForOU,
		string(rt), resourceID, ouID, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete overlay values: %w", err)
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
	targetRows, err := dbClient.QueryContext(ctx, queryListTargets, p.ID, s.scope(ctx))
	if err != nil {
		return Policy{}, fmt.Errorf("failed to list sharing policy targets: %w", err)
	}
	for _, row := range targetRows {
		p.Targets = append(p.Targets, Target{
			ID:    utils.ConvertInterfaceValueToString(row["id"]),
			Scope: targetScope(utils.ConvertInterfaceValueToString(row["target_scope"])),
			OUID:  utils.ConvertInterfaceValueToString(row["target_ou_id"]),
		})
	}

	exclusionRows, err := dbClient.QueryContext(ctx, queryListExclusions, p.ID, s.scope(ctx))
	if err != nil {
		return Policy{}, fmt.Errorf("failed to list sharing policy exclusions: %w", err)
	}
	for _, row := range exclusionRows {
		p.ExcludedOUIDs = append(p.ExcludedOUIDs, utils.ConvertInterfaceValueToString(row["excluded_ou_id"]))
	}

	ruleRows, err := dbClient.QueryContext(ctx, queryListRules, p.ID, s.scope(ctx))
	if err != nil {
		return Policy{}, fmt.Errorf("failed to list overlay rules: %w", err)
	}
	for _, row := range ruleRows {
		rule, err := hydrateRule(row)
		if err != nil {
			return Policy{}, err
		}
		p.Rules = append(p.Rules, rule)
	}
	return p, nil
}

// hydrateRule rebuilds one rule from its stored documents. Absent and empty come back as they went
// in: an omitted list decodes to nil, an explicitly empty one to a list of no members.
func hydrateRule(row map[string]interface{}) (StoredRule, error) {
	out := StoredRule{
		FieldKey: utils.ConvertInterfaceValueToString(row["field_key"]),
		TargetID: utils.ConvertInterfaceValueToString(row["target_id"]),
	}
	resolved := utils.ConvertInterfaceValueToString(row["resolved"])
	if err := json.Unmarshal([]byte(resolved), &out.Resolved); err != nil {
		return StoredRule{}, fmt.Errorf("failed to decode overlay rule: %w", err)
	}
	requested := utils.ConvertInterfaceValueToString(row["requested"])
	if err := json.Unmarshal([]byte(requested), &out.Requested); err != nil {
		return StoredRule{}, fmt.Errorf("failed to decode requested overlay rule: %w", err)
	}
	return out, nil
}

// policyFromRow builds the policy row itself, leaving its contents to hydrate.
func policyFromRow(row map[string]interface{}) Policy {
	// A driver hands a numeric column back as whatever width it decoded, so the version is read
	// through the converter that accepts them all. An unreadable one is zero, which is what a row
	// carrying no version means anyway.
	version, _ := utils.ToInt64(row["version"])

	return Policy{
		ID:             utils.ConvertInterfaceValueToString(row["id"]),
		ResourceType:   ResourceType(utils.ConvertInterfaceValueToString(row["resource_type"])),
		ResourceID:     utils.ConvertInterfaceValueToString(row["resource_id"]),
		OwningOUID:     utils.ConvertInterfaceValueToString(row["owning_ou_id"]),
		InitiatingOUID: utils.ConvertInterfaceValueToString(row["initiating_ou_id"]),
		Stage:          policyStage(utils.ConvertInterfaceValueToString(row["policy_stage"])),
		ParentPolicyID: utils.ConvertInterfaceValueToString(row["parent_policy_id"]),
		Version:        int(version),
	}
}
