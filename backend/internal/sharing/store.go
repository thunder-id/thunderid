// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

var getDBProvider = provider.GetDBProvider

// sharingPolicyStoreInterface is the only place SQL lives. Every method is scoped to one deployment.
type sharingPolicyStoreInterface interface {
	// CreatePolicy inserts a policy with its sections, named units and rules.
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
	// ListPoliciesIssuedBy returns every policy one organization unit issued, for any resource.
	ListPoliciesIssuedBy(ctx context.Context, ouID string) ([]Policy, error)
	// ReplacePolicyContents rewrites a policy's sections, named units and rules, bumping its version
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

// CreatePolicy inserts a policy with its sections, named units and rules.
func (s *sharingStore) CreatePolicy(ctx context.Context, p Policy) error {
	dbClient, err := s.client()
	if err != nil {
		return err
	}
	sections, err := encodeSections(p)
	if err != nil {
		return err
	}
	_, err = dbClient.ExecuteContext(ctx, queryCreatePolicy,
		p.ID, string(p.ResourceType), p.ResourceID, p.OwningOUID, p.InitiatingOUID,
		string(p.Stage), nullableString(p.ParentPolicyID), sections, max(p.Version, 1),
		s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to create sharing policy: %w", err)
	}
	return s.writeContents(ctx, dbClient, p)
}

// storedSection is how one section is recorded in a policy's SECTIONS document.
type storedSection struct {
	// Mode is all or selected. The section reaching every organization unit has none.
	Mode string `json:"mode,omitempty"`
	// ExcludedOUIDs is what a section reaching a whole group withholds from it.
	ExcludedOUIDs []string `json:"excludedOuIds,omitempty"`
}

// encodeSections renders which sections a policy states, in which mode, and what they exclude, as
// one document keyed by section. A section the policy does not state has no entry.
//
// The units a section names are rows of their own, because lookups search by them. Everything else
// about a section is this document on the policy row: it is loaded with the policy, so a new
// section needs no column of its own.
func encodeSections(p Policy) (string, error) {
	document := make(map[Section]storedSection)
	if p.AllUnits != nil {
		document[SectionAllUnits] = storedSection{
			ExcludedOUIDs: utils.UniqueStrings(p.AllUnits.ExcludedOUIDs),
		}
	}
	if p.ChildUnits != nil {
		document[SectionChildUnits] = storedSection{
			Mode:          string(p.ChildUnits.Mode),
			ExcludedOUIDs: utils.UniqueStrings(p.ChildUnits.ExcludedOUIDs),
		}
	}
	if p.OtherTreeRoots != nil {
		document[SectionOtherTreeRoots] = storedSection{
			Mode:          string(p.OtherTreeRoots.Mode),
			ExcludedOUIDs: utils.UniqueStrings(p.OtherTreeRoots.ExcludedOUIDs),
		}
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return "", fmt.Errorf("failed to encode sharing policy sections: %w", err)
	}
	return string(encoded), nil
}

// decodeSections reads the SECTIONS document back into the policy. An exclusion list on a section
// whose mode carries none is left out: only a section reaching a whole group withholds anything.
func decodeSections(p *Policy, column interface{}) error {
	raw := utils.ConvertInterfaceValueToString(column)
	if raw == "" {
		return nil
	}
	var document map[Section]storedSection
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return fmt.Errorf("failed to decode sharing policy sections: %w", err)
	}
	if stored, stated := document[SectionAllUnits]; stated {
		p.AllUnits = &AllUnits{ExcludedOUIDs: stored.ExcludedOUIDs}
	}
	if stored, stated := document[SectionChildUnits]; stated {
		p.ChildUnits = &ChildUnits{Mode: ChildUnitsMode(stored.Mode)}
		if p.ChildUnits.Mode == ChildUnitsAll {
			p.ChildUnits.ExcludedOUIDs = stored.ExcludedOUIDs
		}
	}
	if stored, stated := document[SectionOtherTreeRoots]; stated {
		p.OtherTreeRoots = &OtherTreeRoots{Mode: OtherTreeRootsMode(stored.Mode)}
		if p.OtherTreeRoots.Mode == OtherTreeRootsAll {
			p.OtherTreeRoots.ExcludedOUIDs = stored.ExcludedOUIDs
		}
	}
	return nil
}

// writeContents inserts the named units and rules belonging to a policy. Which sections the policy
// states, in which mode, and what they exclude is on the policy row itself.
func (s *sharingStore) writeContents(ctx context.Context, dbClient provider.DBClientInterface, p Policy) error {
	writeUnit := func(section Section, ouID string, reach UnitReach) error {
		if _, err := dbClient.ExecuteContext(ctx, queryInsertUnit,
			p.ID, string(section), ouID, string(reach), s.scope(ctx)); err != nil {
			return fmt.Errorf("failed to create sharing policy unit: %w", err)
		}
		return nil
	}

	if p.ChildUnits != nil {
		for _, u := range p.ChildUnits.Units {
			if err := writeUnit(SectionChildUnits, u.OUID, u.Reach); err != nil {
				return err
			}
		}
	}
	if p.OtherTreeRoots != nil {
		// A tree is reached at its root and no deeper, which is the reach a row records.
		for _, ouID := range p.OtherTreeRoots.Units {
			if err := writeUnit(SectionOtherTreeRoots, ouID, ReachUnit); err != nil {
				return err
			}
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
		ruleID, policyID, string(r.Grant.Section), nullableString(r.Grant.OUID), r.FieldKey,
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
	return s.hydrate(ctx, dbClient, rows[0])
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
	return s.hydrate(ctx, dbClient, rows[0])
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

// ListPoliciesIssuedBy returns every policy one organization unit issued, for any resource.
func (s *sharingStore) ListPoliciesIssuedBy(ctx context.Context, ouID string) ([]Policy, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}
	rows, err := dbClient.QueryContext(ctx, queryListPoliciesIssuedBy, ouID, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list sharing policies issued by an organization unit: %w", err)
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
	sections, err := encodeSections(p)
	if err != nil {
		return err
	}
	affected, err := dbClient.ExecuteContext(ctx, queryReplacePolicySections,
		sections, p.ID, expectedVersion, s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to update sharing policy: %w", err)
	}
	if affected == 0 {
		return errPolicyNotFound
	}

	if _, err := dbClient.ExecuteContext(ctx, queryDeleteRules, p.ID, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to clear overlay rules: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteUnits, p.ID, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to clear sharing policy units: %w", err)
	}
	return s.writeContents(ctx, dbClient, p)
}

// DeletePolicy removes a policy; its named units and rules cascade with it.
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

// hydrateAll builds a policy from every row, loading the named units and rules of the whole set in
// one statement each per batch, rather than two per policy.
func (s *sharingStore) hydrateAll(
	ctx context.Context, dbClient provider.DBClientInterface, rows []map[string]interface{},
) ([]Policy, error) {
	out := make([]Policy, 0, len(rows))
	positions := make(map[string]int, len(rows))
	for _, row := range rows {
		p := policyFromRow(row)
		if err := decodeSections(&p, row["sections"]); err != nil {
			return nil, err
		}
		positions[p.ID] = len(out)
		out = append(out, p)
	}

	ids := make([]string, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	for chunk := range slices.Chunk(ids, hydrateBatchSize) {
		query, args := buildListUnitsForPoliciesQuery(chunk, s.scope(ctx))
		unitRows, err := dbClient.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to list sharing policy units: %w", err)
		}
		for _, row := range unitRows {
			if i, ok := positions[utils.ConvertInterfaceValueToString(row["policy_id"])]; ok {
				addUnit(&out[i], row)
			}
		}

		query, args = buildListRulesForPoliciesQuery(chunk, s.scope(ctx))
		ruleRows, err := dbClient.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to list overlay rules: %w", err)
		}
		for _, row := range ruleRows {
			i, ok := positions[utils.ConvertInterfaceValueToString(row["policy_id"])]
			if !ok {
				continue
			}
			rule, err := hydrateRule(row)
			if err != nil {
				return nil, err
			}
			out[i].Rules = append(out[i].Rules, rule)
		}
	}
	return out, nil
}

// hydrate builds one policy from its row and loads the named units and rules belonging to it.
func (s *sharingStore) hydrate(
	ctx context.Context, dbClient provider.DBClientInterface, row map[string]interface{},
) (Policy, error) {
	p := policyFromRow(row)
	if err := decodeSections(&p, row["sections"]); err != nil {
		return Policy{}, err
	}

	unitRows, err := dbClient.QueryContext(ctx, queryListUnits, p.ID, s.scope(ctx))
	if err != nil {
		return Policy{}, fmt.Errorf("failed to list sharing policy units: %w", err)
	}
	for _, row := range unitRows {
		addUnit(&p, row)
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

// addUnit adds one named unit row to its policy. A unit row for a section the policy row does not
// state, or for a mode that names none, is left out: the policy row is what says which decisions
// were made.
func addUnit(p *Policy, row map[string]interface{}) {
	ouID := utils.ConvertInterfaceValueToString(row["ou_id"])
	switch Section(utils.ConvertInterfaceValueToString(row["section"])) {
	case SectionChildUnits:
		if p.ChildUnits != nil && p.ChildUnits.Mode == ChildUnitsSelected {
			p.ChildUnits.Units = append(p.ChildUnits.Units, ChildUnit{
				OUID:  ouID,
				Reach: UnitReach(utils.ConvertInterfaceValueToString(row["reach"])),
			})
		}
	case SectionOtherTreeRoots:
		if p.OtherTreeRoots != nil && p.OtherTreeRoots.Mode == OtherTreeRootsSelected {
			p.OtherTreeRoots.Units = append(p.OtherTreeRoots.Units, ouID)
		}
	}
}

// hydrateRule rebuilds one rule from its stored documents. Absent and empty come back as they went
// in: an omitted list decodes to nil, an explicitly empty one to a list of no members.
func hydrateRule(row map[string]interface{}) (StoredRule, error) {
	out := StoredRule{
		FieldKey: utils.ConvertInterfaceValueToString(row["field_key"]),
		Grant: GrantRef{
			Section: Section(utils.ConvertInterfaceValueToString(row["section"])),
			OUID:    utils.ConvertInterfaceValueToString(row["ou_id"]),
		},
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

// policyFromRow builds the policy from the plain columns of its row, leaving its sections to
// hydrate.
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
