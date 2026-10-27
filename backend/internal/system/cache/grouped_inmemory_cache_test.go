// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// GroupedInMemoryCacheTestSuite covers the in-memory cache that can drop a whole group at once.
// What it has to get right is that a group is an invalidation unit and nothing more: entries in
// one group are reachable and removable exactly as any other cache's are.
type GroupedInMemoryCacheTestSuite struct {
	suite.Suite
	cache GroupedCacheInterface[string]
}

func TestGroupedInMemoryCacheSuite(t *testing.T) {
	suite.Run(t, new(GroupedInMemoryCacheTestSuite))
}

func (s *GroupedInMemoryCacheTestSuite) SetupTest() {
	s.cache = newGroupedInMemoryCache[string]("TestGroupedCache", true,
		engineconfig.CacheConfig{TTL: 600, Size: 100}, engineconfig.CacheProperty{})
}

// key is one entry of one group.
func key(group, entry string) GroupedCacheKey {
	return GroupedCacheKey{Group: group, Key: entry}
}

// A value written to a group comes back from it.
func (s *GroupedInMemoryCacheTestSuite) TestAValueRoundTrips() {
	ctx := context.Background()
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-a"), "visible"))

	got, found := s.cache.Get(ctx, key("resource-1", "ou-a"))

	s.True(found)
	s.Equal("visible", got)
}

// Two groups may hold the same entry key without seeing each other's value, which is what lets
// one organization unit be cached per resource rather than once globally.
func (s *GroupedInMemoryCacheTestSuite) TestTheSameEntryKeyInTwoGroupsStaysSeparate() {
	ctx := context.Background()
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-a"), "for-one"))
	s.Require().NoError(s.cache.Set(ctx, key("resource-2", "ou-a"), "for-two"))

	first, _ := s.cache.Get(ctx, key("resource-1", "ou-a"))
	second, _ := s.cache.Get(ctx, key("resource-2", "ou-a"))

	s.Equal("for-one", first)
	s.Equal("for-two", second)
}

// Deleting one entry leaves the rest of its group alone.
func (s *GroupedInMemoryCacheTestSuite) TestDeletingOneEntryKeepsTheGroup() {
	ctx := context.Background()
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-a"), "a"))
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-b"), "b"))

	s.Require().NoError(s.cache.Delete(ctx, key("resource-1", "ou-a")))

	_, foundA := s.cache.Get(ctx, key("resource-1", "ou-a"))
	remaining, foundB := s.cache.Get(ctx, key("resource-1", "ou-b"))
	s.False(foundA)
	s.True(foundB)
	s.Equal("b", remaining)
}

// Dropping a group drops every entry in it and nothing outside it. This is the whole point of the
// grouped cache: one policy write invalidates one resource, not the entire cache.
func (s *GroupedInMemoryCacheTestSuite) TestDeletingAGroupSparesTheOthers() {
	ctx := context.Background()
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-a"), "a"))
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-b"), "b"))
	s.Require().NoError(s.cache.Set(ctx, key("resource-2", "ou-a"), "other"))

	s.Require().NoError(s.cache.DeleteGroup(ctx, "resource-1"))

	_, foundA := s.cache.Get(ctx, key("resource-1", "ou-a"))
	_, foundB := s.cache.Get(ctx, key("resource-1", "ou-b"))
	survivor, foundOther := s.cache.Get(ctx, key("resource-2", "ou-a"))
	s.False(foundA)
	s.False(foundB)
	s.True(foundOther, "another resource's answers are untouched")
	s.Equal("other", survivor)
}

// Invalidating a resource nobody has cached is the ordinary case on a first write, not a failure.
func (s *GroupedInMemoryCacheTestSuite) TestDeletingAnUnknownGroupIsNotAnError() {
	s.NoError(s.cache.DeleteGroup(context.Background(), "never-cached"))
}

// A disabled cache answers every read with a miss and accepts every write silently, so a caller
// needs no special case for it.
func (s *GroupedInMemoryCacheTestSuite) TestADisabledCacheIsANoOp() {
	ctx := context.Background()
	disabled := newGroupedInMemoryCache[string]("TestDisabled", false,
		engineconfig.CacheConfig{TTL: 600}, engineconfig.CacheProperty{})

	s.False(disabled.IsEnabled())
	s.NoError(disabled.Set(ctx, key("resource-1", "ou-a"), "a"))
	_, found := disabled.Get(ctx, key("resource-1", "ou-a"))
	s.False(found)
	s.NoError(disabled.Delete(ctx, key("resource-1", "ou-a")))
	s.NoError(disabled.DeleteGroup(ctx, "resource-1"))
}

// The group index names keys the wrapped cache holds. Once an entry expires, its name has to go
// too, or a group that is read often and invalidated rarely would grow for the life of the process.
func (s *GroupedInMemoryCacheTestSuite) TestCleanupDropsTheIndexEntriesForExpiredValues() {
	ctx := context.Background()
	expiring := newGroupedInMemoryCache[string]("TestExpiring", true,
		engineconfig.CacheConfig{TTL: -1, Size: 100}, engineconfig.CacheProperty{})
	s.Require().NoError(expiring.Set(ctx, key("resource-1", "ou-a"), "a"))

	expiring.CleanupExpired()

	grouped, ok := expiring.(*groupedInMemoryCache[string])
	s.Require().True(ok)
	s.Empty(grouped.groups, "the index still names an entry the cache no longer holds")
}

// Two different (group, entry) pairs must address two different entries, whatever either string
// contains. The wrapped cache has one flat keyspace, so a separator-joined key would let group
// "a:b" entry "c" and group "a" entry "b:c" land on the same place: one write would overwrite the
// other, and invalidating either group would take both.
func (s *GroupedInMemoryCacheTestSuite) TestColonsInAGroupOrEntryDoNotCollide() {
	ctx := context.Background()
	first := GroupedCacheKey{Group: "a:b", Key: "c"}
	second := GroupedCacheKey{Group: "a", Key: "b:c"}

	s.Require().NoError(s.cache.Set(ctx, first, "first"))
	s.Require().NoError(s.cache.Set(ctx, second, "second"))

	gotFirst, foundFirst := s.cache.Get(ctx, first)
	gotSecond, foundSecond := s.cache.Get(ctx, second)
	s.Require().True(foundFirst)
	s.Require().True(foundSecond)
	s.Equal("first", gotFirst, "the second write landed on the first entry")
	s.Equal("second", gotSecond)

	// And invalidating one group leaves the other's value alone.
	s.Require().NoError(s.cache.DeleteGroup(ctx, "a"))

	survivor, stillThere := s.cache.Get(ctx, first)
	s.True(stillThere, "invalidating group \"a\" also dropped group \"a:b\"")
	s.Equal("first", survivor)
}

// Reads, writes and invalidations run concurrently in the sharing service: a request populates the
// cache while a policy write invalidates it. This exercises those paths together so the race
// detector sees them, and checks the one ordering guarantee the lock gives: once DeleteGroup has
// returned and no further write follows, the group is empty.
func (s *GroupedInMemoryCacheTestSuite) TestConcurrentUseLeavesNoEntryBehind() {
	ctx := context.Background()
	const workers = 8

	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(3)
		go func() { defer wg.Done(); _ = s.cache.Set(ctx, key("resource-1", "ou-"+strconv.Itoa(i)), "v") }()
		go func() { defer wg.Done(); s.cache.Get(ctx, key("resource-1", "ou-"+strconv.Itoa(i))) }()
		go func() { defer wg.Done(); _ = s.cache.DeleteGroup(ctx, "resource-1") }()
	}
	wg.Wait()

	// Nothing is writing now, so the final invalidation is the last word.
	s.Require().NoError(s.cache.DeleteGroup(ctx, "resource-1"))

	for i := range workers {
		_, found := s.cache.Get(ctx, key("resource-1", "ou-"+strconv.Itoa(i)))
		s.False(found, "an entry outlived the invalidation of its group")
	}
	grouped, ok := s.cache.(*groupedInMemoryCache[string])
	s.Require().True(ok)
	s.Empty(grouped.groups, "the index still names a group that was invalidated")
}

// The wrapper reports the wrapped cache's name and statistics, so a grouped cache is observable
// the same way every other in-memory cache is.
func (s *GroupedInMemoryCacheTestSuite) TestNameAndStatsComeFromTheWrappedCache() {
	ctx := context.Background()
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-a"), "a"))
	s.cache.Get(ctx, key("resource-1", "ou-a"))
	s.cache.Get(ctx, key("resource-1", "missing"))

	s.Equal("TestGroupedCache", s.cache.GetName())
	stats := s.cache.GetStats()
	s.True(stats.Enabled)
	s.EqualValues(1, stats.HitCount)
	s.EqualValues(1, stats.MissCount)
}

// Deleting the last entry of a group drops the group itself, so the index does not keep an empty
// shell for every resource ever cached.
func (s *GroupedInMemoryCacheTestSuite) TestDeletingTheLastEntryDropsTheGroup() {
	ctx := context.Background()
	s.Require().NoError(s.cache.Set(ctx, key("resource-1", "ou-a"), "a"))

	s.Require().NoError(s.cache.Delete(ctx, key("resource-1", "ou-a")))

	grouped, ok := s.cache.(*groupedInMemoryCache[string])
	s.Require().True(ok)
	s.Empty(grouped.groups)
}

// Deleting an entry of a group that was never cached is a no-op, not a panic on a missing map.
func (s *GroupedInMemoryCacheTestSuite) TestDeletingFromAnUnknownGroupIsSafe() {
	s.NoError(s.cache.Delete(context.Background(), key("never-cached", "ou-a")))
}
