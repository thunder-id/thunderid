// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"container/list"
	"fmt"
	"strings"
	"sync"
)

// A version never changes once captured, and a master never changes once written, so what is read
// from one, and the diff of one against the other, holds for as long as either is asked about. Both are
// kept, a bounded number of each, so a console reading the same gateway again and again reads each
// bundle once. A key is made of what the kept value was read from, so a new version or master is a new
// key rather than a stale value.

const (
	// keptBundles is how many parsed bundles are kept: the versions and masters being compared.
	keptBundles = 16
	// keptPlans is how many diffs are kept: a version against a master, a few per gateway.
	keptPlans = 32
)

// parsedBundle is a bundle split into its resources, with what is read from all of them together, each
// read the first time it is asked for.
type parsedBundle struct {
	resources []bundleResource

	referencesOnce sync.Once
	references     []ValueReference

	needsOnce sync.Once
	needs     map[string][]string

	misplacedOnce sync.Once
	misplaced     []ValueReference
}

// referencesOf returns each place the bundle refers to a variable or a secret, in document order. The
// slice is shared: a caller copies it before changing it.
func (b *parsedBundle) referencesOf() []ValueReference {
	b.referencesOnce.Do(func() { b.references = referencesAmong(b.resources) })
	return b.references
}

// needsOf returns what each resource of the bundle needs, by key. The map is shared and read only.
func (b *parsedBundle) needsOf() map[string][]string {
	b.needsOnce.Do(func() { b.needs = needsOf(b.resources) })
	return b.needs
}

// misplacedSecrets returns where the bundle refers to a secret from a field that holds no credential.
// The fields are the service's own, the same on every call.
func (b *parsedBundle) misplacedSecrets(secretFields map[string][]string) []ValueReference {
	b.misplacedOnce.Do(func() { b.misplaced = misplacedSecretsAmong(b.resources, secretFields) })
	return b.misplaced
}

// planCacheInterface keeps what a plan reads from versions and masters, and the diffs it works out.
type planCacheInterface interface {
	// bundle returns a bundle's content split into its resources, read once while it is kept.
	bundle(content string) *parsedBundle
	// diff returns the diff of a version against a gateway's master, kept or worked out by compute from
	// the two bundles parsed, and kept. Concurrent callers asking for one not kept share one
	// computation. The diff returned is the caller's own to change.
	diff(gatewayID string, master *Master, target *Version, compute func(held, resources *parsedBundle) *Diff) *Diff
}

// planCache keeps a bounded number of parsed bundles, by their content, and of diffs, by what each was
// worked out from.
type planCache struct {
	bundles *boundedCache[*parsedBundle]
	diffs   *boundedCache[plannedDiff]
}

// plannedDiff is a kept diff with the bundles it was worked out from, which it holds on to so that no
// other bundle is parsed at the same place while the diff is kept under their addresses.
type plannedDiff struct {
	diff            *Diff
	held, resources *parsedBundle
}

func newPlanCache() planCacheInterface {
	return &planCache{
		bundles: newBoundedCache[*parsedBundle](keptBundles),
		diffs:   newBoundedCache[plannedDiff](keptPlans),
	}
}

func (c *planCache) bundle(content string) *parsedBundle {
	return c.bundles.get(content, func() *parsedBundle {
		return &parsedBundle{resources: parseBundle(content)}
	})
}

func (c *planCache) diff(gatewayID string, master *Master, target *Version,
	compute func(held, resources *parsedBundle) *Diff) *Diff {
	held, resources := c.bundle(master.Resources), c.bundle(target.Resources)
	planned := c.diffs.get(planKey(gatewayID, master, held, target, resources), func() plannedDiff {
		return plannedDiff{diff: compute(held, resources), held: held, resources: resources}
	})
	return copyDiff(planned.diff)
}

// copyDiff copies a kept diff so the caller can mark its changes. A change's own lists are only ever
// replaced, never written into, so they are shared.
func copyDiff(kept *Diff) *Diff {
	copied := *kept
	if kept.Changes != nil {
		copied.Changes = make([]Change, len(kept.Changes))
		copy(copied.Changes, kept.Changes)
	}
	if kept.MisplacedSecrets != nil {
		copied.MisplacedSecrets = make([]ValueReference, len(kept.MisplacedSecrets))
		copy(copied.MisplacedSecrets, kept.MisplacedSecrets)
	}
	return &copied
}

// planKey names a diff by what it is worked out from: the gateway, the master's hash and the resources
// it refused, and the version's hash, with the addresses of the parsed bundles, each parsed from one
// content.
func planKey(gatewayID string, master *Master, held *parsedBundle, target *Version, resources *parsedBundle) string {
	var failed []string
	for _, entry := range master.Entries {
		if entry.State == EntryFailed {
			failed = append(failed, entry.Key)
		}
	}
	return strings.Join([]string{gatewayID, master.Hash, target.Hash, fmt.Sprintf("%p", held),
		fmt.Sprintf("%p", resources), strings.Join(failed, "\x1f")}, "\x00")
}

// boundedCache keeps up to limit values by key, dropping the one used least recently, and has
// concurrent callers asking for a key that is not kept share one computation.
type boundedCache[V any] struct {
	mu      sync.Mutex
	limit   int
	entries map[string]*list.Element
	order   *list.List
	flights map[string]*cacheFlight[V]
}

// cacheEntry is one kept value.
type cacheEntry[V any] struct {
	key   string
	value V
}

// cacheFlight is a computation callers wait on. ok is false when it did not finish, so each caller
// waiting on it computes the value itself.
type cacheFlight[V any] struct {
	done  chan struct{}
	value V
	ok    bool
}

func newBoundedCache[V any](limit int) *boundedCache[V] {
	return &boundedCache[V]{limit: limit, entries: map[string]*list.Element{}, order: list.New(),
		flights: map[string]*cacheFlight[V]{}}
}

// get returns the value kept under key, or computes it, keeps it and returns it.
func (c *boundedCache[V]) get(key string, compute func() V) V {
	c.mu.Lock()
	if element, ok := c.entries[key]; ok {
		c.order.MoveToFront(element)
		value := element.Value.(*cacheEntry[V]).value
		c.mu.Unlock()
		return value
	}
	if running, ok := c.flights[key]; ok {
		c.mu.Unlock()
		<-running.done
		if running.ok {
			return running.value
		}
		return compute()
	}
	flight := &cacheFlight[V]{done: make(chan struct{})}
	c.flights[key] = flight
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.flights, key)
		if flight.ok {
			c.keep(key, flight.value)
		}
		c.mu.Unlock()
		close(flight.done)
	}()
	flight.value = compute()
	flight.ok = true
	return flight.value
}

// keep adds a value, dropping the least recently used beyond the limit. The lock is held.
func (c *boundedCache[V]) keep(key string, value V) {
	c.entries[key] = c.order.PushFront(&cacheEntry[V]{key: key, value: value})
	for c.order.Len() > c.limit {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*cacheEntry[V]).key)
	}
}
