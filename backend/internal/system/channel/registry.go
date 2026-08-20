/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package channel

import (
	"sort"
	"sync"
	"time"
)

// ConnEntry is a registered Data Plane connection tracked by the registry.
//
// ID names the Data Plane, and Instance names which of its replicas this connection belongs to. A
// Data Plane runs as several pods and every one of them dials, so the pair is what identifies a
// socket while the id alone identifies the deployment a command is addressed to.
type ConnEntry interface {
	ID() string
	Instance() string
	LastSeen() time.Time
	Close(reason string)
	// CloseNow closes the connection immediately, without the close handshake. It is used to evict a
	// superseded connection so that eviction cannot block on an unresponsive peer.
	CloseNow()
}

// ConnInfo is a point-in-time snapshot of a registered connection, used for observability.
type ConnInfo struct {
	ID       string
	Instance string
	LastSeen time.Time
}

// connKey identifies one socket: a Data Plane and which of its replicas holds it.
type connKey struct {
	id       string
	instance string
}

// Registry tracks active Data Plane connections on the Control Plane.
//
// A Data Plane may hold several connections at once, one per replica. The single-active-socket policy
// applies per replica rather than per Data Plane: a pod reconnecting replaces its own socket, and
// never a sibling's. Keying by the Data Plane alone would make replicas evict each other in turn and
// leave the connection flapping.
type Registry[T ConnEntry] struct {
	mu    sync.RWMutex
	conns map[connKey]T
	// next round-robins delivery across a Data Plane's replicas, so one pod does not take every
	// command while its siblings idle.
	next map[string]int
}

// NewRegistry creates an empty registry.
func NewRegistry[T ConnEntry]() *Registry[T] {
	return &Registry[T]{conns: make(map[connKey]T), next: make(map[string]int)}
}

// Register stores c under its id, evicting any existing connection for that id. Eviction uses
// CloseNow rather than the graceful Close: the old connection is a zombie that does not deserve a
// close handshake, and a slow or unresponsive peer must not be allowed to block the caller (Register
// runs on the new connection's handshake path, before its read loop has started).
func (r *Registry[T]) Register(c T) {
	key := connKey{id: c.ID(), instance: c.Instance()}
	r.mu.Lock()
	old, existed := r.conns[key]
	r.conns[key] = c
	r.mu.Unlock()
	if existed {
		old.CloseNow()
	}
}

// Unregister removes the entry for a connection only if it is still c, avoiding a race where a newer
// connection replaced c between its read loop ending and this call.
func (r *Registry[T]) Unregister(id string, c T) {
	key := connKey{id: id, instance: c.Instance()}
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.conns[key]; ok && any(cur) == any(c) {
		delete(r.conns, key)
	}
}

// Get returns one active connection for a Data Plane, if it holds any.
//
// A command goes to a single replica, not to all of them: the replicas share a database, so applying
// an import on one is visible to every other, and sending it to each would apply it several times.
// Which one is chosen rotates, so a Data Plane's replicas share the work.
func (r *Registry[T]) Get(id string) (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	matches := make([]T, 0, 2)
	for key, c := range r.conns {
		if key.id == id {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		var zero T
		return zero, false
	}
	// Ordering a map range is not stable, so sort by instance to make the rotation meaningful rather
	// than a second source of randomness.
	sort.Slice(matches, func(i, j int) bool { return matches[i].Instance() < matches[j].Instance() })

	chosen := matches[r.next[id]%len(matches)]
	r.next[id] = (r.next[id] + 1) % len(matches)
	return chosen, true
}

// Instances reports how many replicas of a Data Plane are connected.
func (r *Registry[T]) Instances(id string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for key := range r.conns {
		if key.id == id {
			count++
		}
	}
	return count
}

// List returns a snapshot of all active connections.
func (r *Registry[T]) List() []ConnInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConnInfo, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, ConnInfo{ID: c.ID(), Instance: c.Instance(), LastSeen: c.LastSeen()})
	}
	return out
}

// entries returns a snapshot of all active connections for internal fan-out (for example, closing
// every connection during shutdown).
func (r *Registry[T]) entries() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]T, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, c)
	}
	return out
}
