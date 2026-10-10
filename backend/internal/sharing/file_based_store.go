// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"errors"
	"sync"
)

// fileBasedStore holds the policies of declaratively defined resources in memory, for the lifetime
// of the process.
//
// A declarative resource lives in its own file and is never written to the database, so neither are
// its policies: the file is the sole source of truth for both. Persisting them alone would strand
// rows whenever the file changed, and replaying on the next startup would append duplicates.
type fileBasedStore struct {
	mu sync.RWMutex
	// Kept as a slice so reads preserve declaration order, which is the order a file's policies
	// were replayed in.
	policies []Policy
}

// newFileBasedStore creates an empty file-declared policy store.
func newFileBasedStore() *fileBasedStore {
	return &fileBasedStore{}
}

// seed records a policy. Called only while declarative resources are loading.
func (f *fileBasedStore) seed(p Policy) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Re-loading a file replaces its policy rather than appending, which is what makes declarative
	// replay idempotent across restarts.
	for i, existing := range f.policies {
		if existing.ResourceType == p.ResourceType &&
			existing.ResourceID == p.ResourceID &&
			existing.InitiatingOUID == p.InitiatingOUID {
			f.policies[i] = p
			return
		}
	}
	f.policies = append(f.policies, p)
}

// listForType returns every declared policy of one resource type, which the reverse lookup needs
// because the chain-scoped query reads the database only.
func (f *fileBasedStore) listForType(rt ResourceType) []Policy {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var out []Policy
	for _, p := range f.policies {
		if p.ResourceType == rt {
			out = append(out, p)
		}
	}
	return out
}

// get returns the declared policy with the given id.
func (f *fileBasedStore) get(id string) (Policy, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, p := range f.policies {
		if p.ID == id {
			return p, true
		}
	}
	return Policy{}, false
}

// getByInitiator returns the declared policy an organization unit holds for a resource.
func (f *fileBasedStore) getByInitiator(
	rt ResourceType, resourceID, initiatingOUID string,
) (Policy, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, p := range f.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID && p.InitiatingOUID == initiatingOUID {
			return p, true
		}
	}
	return Policy{}, false
}

// listForResource returns every declared policy for one resource.
func (f *fileBasedStore) listForResource(rt ResourceType, resourceID string) []Policy {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]Policy, 0, len(f.policies))
	for _, p := range f.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID {
			out = append(out, p)
		}
	}
	return out
}

// The rest of sharingPolicyStoreInterface, so that a composite can hold this store and the database
// one behind one type. A declaration is its file: it is never written here and never carries an
// organization unit's own overlay values, which live in the database whichever store the governing
// policy came from. Those methods are refused rather than answered emptily, because the composite
// routes them to the database and a call reaching here would be a wiring mistake, not a miss.

// CreatePolicy is refused: a declared policy is seeded from its file, never inserted.
func (f *fileBasedStore) CreatePolicy(_ context.Context, _ Policy) error {
	return errFileStoreReadOnly
}

// GetPolicy returns the declared policy with an id, or reports it missing.
func (f *fileBasedStore) GetPolicy(_ context.Context, id string) (Policy, error) {
	if p, ok := f.get(id); ok {
		return p, nil
	}
	return Policy{}, errPolicyNotFound
}

// GetPolicyByInitiator returns the declared policy an organization unit holds for a resource.
func (f *fileBasedStore) GetPolicyByInitiator(
	_ context.Context, rt ResourceType, resourceID, initiatingOUID string,
) (Policy, error) {
	if p, ok := f.getByInitiator(rt, resourceID, initiatingOUID); ok {
		return p, nil
	}
	return Policy{}, errPolicyNotFound
}

// ListPoliciesForResource returns one page of a resource's declared policies. Declaration order is
// stable across restarts because it is the order the files were replayed in, so it is what a page
// boundary is measured against.
func (f *fileBasedStore) ListPoliciesForResource(
	_ context.Context, rt ResourceType, resourceID string, limit, offset int,
) ([]Policy, error) {
	all := f.listForResource(rt, resourceID)
	if offset >= len(all) {
		return nil, nil
	}
	return all[offset:min(offset+limit, len(all))], nil
}

// CountPoliciesForResource returns how many declared policies one resource has.
func (f *fileBasedStore) CountPoliciesForResource(
	_ context.Context, rt ResourceType, resourceID string,
) (int, error) {
	return len(f.listForResource(rt, resourceID)), nil
}

// ListAllPoliciesForResource returns every declared policy for one resource.
func (f *fileBasedStore) ListAllPoliciesForResource(
	_ context.Context, rt ResourceType, resourceID string,
) ([]Policy, error) {
	return f.listForResource(rt, resourceID), nil
}

// ListPoliciesRelevantToChain returns every declared policy that could bear on the chain. The chain
// is not consulted: coverage is decided in memory by the caller, and a declaration set is small
// enough that narrowing it here would only duplicate that decision.
func (f *fileBasedStore) ListPoliciesRelevantToChain(
	_ context.Context, rt ResourceType, resourceID string, _ []string,
) ([]Policy, error) {
	if resourceID == "" {
		return f.listForType(rt), nil
	}
	return f.listForResource(rt, resourceID), nil
}

// ReplacePolicyContents is refused: editing a declaration means editing its file.
func (f *fileBasedStore) ReplacePolicyContents(_ context.Context, _ Policy, _ int) error {
	return errFileStoreReadOnly
}

// DeletePolicy is refused: removing a declaration means removing it from its file.
func (f *fileBasedStore) DeletePolicy(_ context.Context, _ string) error {
	return errFileStoreReadOnly
}

// GetOverlayValues is refused: an organization unit's own values are database rows.
func (f *fileBasedStore) GetOverlayValues(
	_ context.Context, _ ResourceType, _, _ string,
) (map[string][]string, error) {
	return nil, errFileStoreReadOnly
}

// SetOverlayValue is refused: an organization unit's own values are database rows.
func (f *fileBasedStore) SetOverlayValue(
	_ context.Context, _ ResourceType, _, _, _ string, _ []string,
) error {
	return errFileStoreReadOnly
}

// DeleteOverlayValue is refused: an organization unit's own values are database rows.
func (f *fileBasedStore) DeleteOverlayValue(_ context.Context, _ ResourceType, _, _, _ string) error {
	return errFileStoreReadOnly
}

// DeleteOverlayValuesForOU is refused: an organization unit's own values are database rows.
func (f *fileBasedStore) DeleteOverlayValuesForOU(_ context.Context, _ ResourceType, _, _ string) error {
	return errFileStoreReadOnly
}

// errFileStoreReadOnly reports a call that belongs to the database store. It never reaches a caller:
// the composite routes every such method to the database, so this is a wiring guard.
var errFileStoreReadOnly = errors.New("sharing policy file store is read-only")
