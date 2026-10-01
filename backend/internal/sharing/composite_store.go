// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
)

// compositeStore combines the file-declared policies with the stored ones, so the service asks one
// store rather than merging two.
//
//   - Reads consult the database first and fall back to the declarations.
//   - Writes go to the database alone. A declaration is its file, and the service refuses to edit
//     or delete one before it ever reaches a store.
//   - An organization unit's own overlay values are database rows whichever store the policy
//     governing them came from, so those calls never consult the declarations at all.
type compositeStore struct {
	fileStore sharingPolicyStoreInterface
	dbStore   sharingPolicyStoreInterface
}

// newCompositeStore combines the two stores. A deployment that declares nothing gets the database
// store unwrapped, so the fallbacks below cost nothing when there is nothing to fall back to.
func newCompositeStore(fileStore, dbStore sharingPolicyStoreInterface) sharingPolicyStoreInterface {
	if fileStore == nil {
		return dbStore
	}
	return &compositeStore{fileStore: fileStore, dbStore: dbStore}
}

// CreatePolicy inserts into the database. A declared policy is seeded from its file instead, and
// never arrives here.
func (c *compositeStore) CreatePolicy(ctx context.Context, p Policy) error {
	return c.dbStore.CreatePolicy(ctx, p)
}

// GetPolicy returns a policy by id, asking the database first.
//
// The order matters for a policy whose declaration has been withdrawn from its file while a row it
// left behind survives: the row is what still applies, so it answers rather than being reported
// missing.
func (c *compositeStore) GetPolicy(ctx context.Context, id string) (Policy, error) {
	p, err := c.dbStore.GetPolicy(ctx, id)
	if err == nil || !errors.Is(err, errPolicyNotFound) {
		return p, err
	}
	return c.fileStore.GetPolicy(ctx, id)
}

// GetPolicyByInitiator returns the one policy an organization unit holds for a resource.
//
// The two stores cannot both hold one for the same organization unit, because requireSoleGovernor
// refuses the second whichever way round it arrives. The database is still asked first, for the
// same withdrawn-declaration reason as GetPolicy.
func (c *compositeStore) GetPolicyByInitiator(
	ctx context.Context, rt ResourceType, resourceID, initiatingOUID string,
) (Policy, error) {
	p, err := c.dbStore.GetPolicyByInitiator(ctx, rt, resourceID, initiatingOUID)
	if err == nil || !errors.Is(err, errPolicyNotFound) {
		return p, err
	}
	return c.fileStore.GetPolicyByInitiator(ctx, rt, resourceID, initiatingOUID)
}

// ListPoliciesForResource returns every policy recorded for one resource, from both stores.
func (c *compositeStore) ListPoliciesForResource(
	ctx context.Context, rt ResourceType, resourceID string,
) ([]Policy, error) {
	stored, err := c.dbStore.ListPoliciesForResource(ctx, rt, resourceID)
	if err != nil {
		return nil, err
	}
	declared, err := c.fileStore.ListPoliciesForResource(ctx, rt, resourceID)
	if err != nil {
		return nil, err
	}
	return appendUnsuperseded(stored, declared), nil
}

// ListPoliciesRelevantToChain returns the policies that could cover any organization unit in the
// chain, from both stores. An empty resourceID spans every resource of the type.
func (c *compositeStore) ListPoliciesRelevantToChain(
	ctx context.Context, rt ResourceType, resourceID string, chainOUIDs []string,
) ([]Policy, error) {
	stored, err := c.dbStore.ListPoliciesRelevantToChain(ctx, rt, resourceID, chainOUIDs)
	if err != nil {
		return nil, err
	}
	declared, err := c.fileStore.ListPoliciesRelevantToChain(ctx, rt, resourceID, chainOUIDs)
	if err != nil {
		return nil, err
	}
	return appendUnsuperseded(stored, declared), nil
}

// ReplacePolicyContents rewrites a stored policy. An edit to a declared one is refused by the
// service, which knows it is declared before any store is asked.
func (c *compositeStore) ReplacePolicyContents(ctx context.Context, p Policy, expectedVersion int) error {
	return c.dbStore.ReplacePolicyContents(ctx, p, expectedVersion)
}

// DeletePolicy removes a stored policy, for the same reason.
func (c *compositeStore) DeletePolicy(ctx context.Context, id string) error {
	return c.dbStore.DeletePolicy(ctx, id)
}

// GetOverlayValues reads an organization unit's own values, which are always database rows.
func (c *compositeStore) GetOverlayValues(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) (map[string][]string, error) {
	return c.dbStore.GetOverlayValues(ctx, rt, resourceID, ouID)
}

// SetOverlayValue records one organization unit's value for one field.
func (c *compositeStore) SetOverlayValue(
	ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string, value []string,
) error {
	return c.dbStore.SetOverlayValue(ctx, rt, resourceID, ouID, fieldKey, value)
}

// DeleteOverlayValue removes one organization unit's value for one field.
func (c *compositeStore) DeleteOverlayValue(
	ctx context.Context, rt ResourceType, resourceID, ouID, fieldKey string,
) error {
	return c.dbStore.DeleteOverlayValue(ctx, rt, resourceID, ouID, fieldKey)
}

// DeleteOverlayValuesForOU removes every value one organization unit holds for a resource.
func (c *compositeStore) DeleteOverlayValuesForOU(
	ctx context.Context, rt ResourceType, resourceID, ouID string,
) error {
	return c.dbStore.DeleteOverlayValuesForOU(ctx, rt, resourceID, ouID)
}

// appendUnsuperseded adds the declared policies whose organization unit holds no stored policy.
//
// The two sets are disjoint in practice, because requireSoleGovernor refuses a second policy for an
// organization unit whichever store the first is in. The filter is what keeps that a fact rather
// than an assumption: were a row and a declaration ever to name the same unit, reporting both would
// hand the caller two policies for one unit, which every reader here treats as impossible.
func appendUnsuperseded(stored, declared []Policy) []Policy {
	if len(declared) == 0 {
		return stored
	}
	superseded := make(map[string]struct{}, len(stored))
	for _, p := range stored {
		superseded[p.InitiatingOUID] = struct{}{}
	}
	for _, d := range declared {
		if _, edited := superseded[d.InitiatingOUID]; !edited {
			stored = append(stored, d)
		}
	}
	return stored
}
