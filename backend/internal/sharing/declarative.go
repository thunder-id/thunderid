// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import "sync"

// declarativePolicyStore holds the policies of declaratively defined resources in memory, for the
// lifetime of the process.
//
// A declarative resource lives in its own file and is never written to the database, so neither are
// its policies: the file is the sole source of truth for both. Persisting them alone would strand
// rows whenever the file changed, and replaying on the next startup would append duplicates.
type declarativePolicyStore struct {
	mu sync.RWMutex
	// Kept as a slice so reads preserve declaration order, which is the order a file's policies
	// were replayed in.
	policies []Policy
}

// newDeclarativePolicyStore creates an empty declarative policy store.
func newDeclarativePolicyStore() *declarativePolicyStore {
	return &declarativePolicyStore{}
}

// seed records a policy. Called only while declarative resources are loading.
func (d *declarativePolicyStore) seed(p Policy) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Re-loading a file replaces its policy rather than appending, which is what makes declarative
	// replay idempotent across restarts.
	for i, existing := range d.policies {
		if existing.ResourceType == p.ResourceType &&
			existing.ResourceID == p.ResourceID &&
			existing.InitiatingOUID == p.InitiatingOUID {
			d.policies[i] = p
			return
		}
	}
	d.policies = append(d.policies, p)
}

// listForType returns every declared policy of one resource type, which the reverse lookup needs
// because the chain-scoped query reads the database only.
func (d *declarativePolicyStore) listForType(rt ResourceType) []Policy {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []Policy
	for _, p := range d.policies {
		if p.ResourceType == rt {
			out = append(out, p)
		}
	}
	return out
}

// get returns the declared policy with the given id.
func (d *declarativePolicyStore) get(id string) (Policy, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, p := range d.policies {
		if p.ID == id {
			return p, true
		}
	}
	return Policy{}, false
}

// getByInitiator returns the declared policy an organization unit holds for a resource.
func (d *declarativePolicyStore) getByInitiator(
	rt ResourceType, resourceID, initiatingOUID string,
) (Policy, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, p := range d.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID && p.InitiatingOUID == initiatingOUID {
			return p, true
		}
	}
	return Policy{}, false
}

// listForResource returns every declared policy for one resource.
func (d *declarativePolicyStore) listForResource(rt ResourceType, resourceID string) []Policy {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Policy, 0, len(d.policies))
	for _, p := range d.policies {
		if p.ResourceType == rt && p.ResourceID == resourceID {
			out = append(out, p)
		}
	}
	return out
}
