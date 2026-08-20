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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeConn struct {
	id       string
	instance string
	seen     time.Time
	closed   bool

	closeMsg string
	// closeDelay simulates a slow graceful-close handshake, so tests can prove that eviction never
	// takes this path.
	closeDelay time.Duration
	closedNow  bool
}

func (f *fakeConn) ID() string { return f.id }

// A connection with no instance set stands for a single-replica Data Plane.
func (f *fakeConn) Instance() string {
	if f.instance == "" {
		return defaultInstance
	}
	return f.instance
}

func (f *fakeConn) LastSeen() time.Time { return f.seen }
func (f *fakeConn) Close(reason string) {
	time.Sleep(f.closeDelay)
	f.closed = true
	f.closeMsg = reason
}
func (f *fakeConn) CloseNow() { f.closedNow = true }

func TestRegistryRegisterGetUnregister(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	c := &fakeConn{id: "dp-1"}
	r.Register(c)

	got, ok := r.Get("dp-1")
	assert.True(t, ok)
	assert.Same(t, c, got)

	r.Unregister("dp-1", c)
	_, ok = r.Get("dp-1")
	assert.False(t, ok)
}

func TestRegistryEvictsDuplicateID(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	old := &fakeConn{id: "dp-1"}
	r.Register(old)
	fresh := &fakeConn{id: "dp-1"}
	r.Register(fresh)

	assert.True(t, old.closedNow, "old connection should be closed immediately on duplicate register")
	got, _ := r.Get("dp-1")
	assert.Same(t, fresh, got)
}

func TestRegistryEvictionDoesNotBlockOnSlowGracefulClose(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	old := &fakeConn{id: "dp-1", closeDelay: 200 * time.Millisecond}
	r.Register(old)

	start := time.Now()
	fresh := &fakeConn{id: "dp-1"}
	r.Register(fresh)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 100*time.Millisecond,
		"Register must evict with CloseNow, not the slow graceful Close, so it never blocks")
	assert.True(t, old.closedNow, "evicted connection should be closed immediately")
	assert.False(t, old.closed, "eviction must not invoke the graceful Close handshake")

	got, _ := r.Get("dp-1")
	assert.Same(t, fresh, got)
}

func TestRegistryUnregisterOnlyRemovesMatchingEntry(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	fresh := &fakeConn{id: "dp-1"}
	r.Register(fresh)
	stale := &fakeConn{id: "dp-1"}

	r.Unregister("dp-1", stale) // stale is not the current entry; must be a no-op
	got, ok := r.Get("dp-1")
	assert.True(t, ok)
	assert.Same(t, fresh, got)
}

func TestRegistryListSnapshots(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	r.Register(&fakeConn{id: "dp-1", seen: time.Unix(10, 0)})
	r.Register(&fakeConn{id: "dp-2", seen: time.Unix(20, 0)})
	assert.Len(t, r.List(), 2)
}

// A Data Plane runs as several replicas and every one of them dials. Keying by the Data Plane alone
// would make each new connection evict the last, leaving the connection flapping and commands landing
// on whichever pod connected most recently.
func TestReplicasOfOneDataPlaneCoexist(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	a := &fakeConn{id: "org3:dev", instance: "pod-a"}
	b := &fakeConn{id: "org3:dev", instance: "pod-b"}

	r.Register(a)
	r.Register(b)

	assert.False(t, a.closedNow, "registering a sibling must not evict pod-a")
	assert.Equal(t, 2, r.Instances("org3:dev"))
}

// A replica reconnecting replaces its own socket, which is what the single-socket policy is for.
func TestAReplicaReconnectingEvictsOnlyItsOwnSocket(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	first := &fakeConn{id: "org3:dev", instance: "pod-a"}
	sibling := &fakeConn{id: "org3:dev", instance: "pod-b"}
	r.Register(first)
	r.Register(sibling)

	again := &fakeConn{id: "org3:dev", instance: "pod-a"}
	r.Register(again)

	assert.True(t, first.closedNow, "pod-a's stale socket should be evicted")
	assert.False(t, sibling.closedNow, "pod-b must be left alone")
	assert.Equal(t, 2, r.Instances("org3:dev"))
}

// A command goes to one replica, not to all of them: they share a database, so applying an import on
// one is visible to the others and sending it to each would apply it several times. Which one is
// chosen rotates, so the replicas share the work.
func TestDeliveryPicksOneReplicaAndRotates(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	r.Register(&fakeConn{id: "org3:dev", instance: "pod-a"})
	r.Register(&fakeConn{id: "org3:dev", instance: "pod-b"})

	first, ok := r.Get("org3:dev")
	assert.True(t, ok)
	second, ok := r.Get("org3:dev")
	assert.True(t, ok)
	third, ok := r.Get("org3:dev")
	assert.True(t, ok)

	assert.NotEqual(t, first.Instance(), second.Instance(), "a second command should go to the sibling")
	assert.Equal(t, first.Instance(), third.Instance(), "the rotation should come back around")
}

// Unregistering one replica leaves the rest serving.
func TestUnregisteringOneReplicaLeavesTheOthers(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	a := &fakeConn{id: "org3:dev", instance: "pod-a"}
	b := &fakeConn{id: "org3:dev", instance: "pod-b"}
	r.Register(a)
	r.Register(b)

	r.Unregister("org3:dev", a)

	assert.Equal(t, 1, r.Instances("org3:dev"))
	got, ok := r.Get("org3:dev")
	assert.True(t, ok)
	assert.Equal(t, "pod-b", got.Instance())
}
