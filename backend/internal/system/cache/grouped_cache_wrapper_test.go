// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// GroupedCacheWrapperTestSuite covers the wrapper the manager hands out, and the factory that
// builds it. What the wrapper decides is how a cache that is off or failing behaves: a caller
// should never have to special-case either.
type GroupedCacheWrapperTestSuite struct {
	suite.Suite
	impl  *GroupedCacheInterfaceMock[string]
	cache *GroupedCache[string]
}

func TestGroupedCacheWrapperSuite(t *testing.T) {
	suite.Run(t, new(GroupedCacheWrapperTestSuite))
}

func (s *GroupedCacheWrapperTestSuite) SetupTest() {
	s.impl = NewGroupedCacheInterfaceMock[string](s.T())
	s.cache = &GroupedCache[string]{enabled: true, cacheName: "TestWrapped", cacheImpl: s.impl}
}

// entry is one key of one group.
func entry(group, key string) GroupedCacheKey {
	return GroupedCacheKey{Group: group, Key: key}
}

// An enabled cache passes every operation through to the implementation.
func (s *GroupedCacheWrapperTestSuite) TestOperationsReachTheImplementation() {
	ctx := context.Background()
	s.impl.EXPECT().IsEnabled().Return(true)
	s.impl.EXPECT().Set(ctx, entry("g", "k"), "v").Return(nil).Once()
	s.impl.EXPECT().Get(ctx, entry("g", "k")).Return("v", true).Once()
	s.impl.EXPECT().Delete(ctx, entry("g", "k")).Return(nil).Once()
	s.impl.EXPECT().DeleteGroup(ctx, "g").Return(nil).Once()
	s.impl.EXPECT().CleanupExpired().Return().Once()

	s.Require().NoError(s.cache.Set(ctx, entry("g", "k"), "v"))
	got, found := s.cache.Get(ctx, entry("g", "k"))
	s.True(found)
	s.Equal("v", got)
	s.Require().NoError(s.cache.Delete(ctx, entry("g", "k")))
	s.Require().NoError(s.cache.DeleteGroup(ctx, "g"))
	s.cache.CleanupExpired()
	s.Equal("TestWrapped", s.cache.GetName())
	s.True(s.cache.IsEnabled())
}

// A failing Set or Delete is logged and swallowed. A cache that cannot store something is not a
// reason to fail the request that was being served.
func (s *GroupedCacheWrapperTestSuite) TestAFailedWriteIsNotTheCallersProblem() {
	ctx := context.Background()
	s.impl.EXPECT().IsEnabled().Return(true)
	s.impl.EXPECT().Set(ctx, entry("g", "k"), "v").Return(errors.New("unreachable")).Once()
	s.impl.EXPECT().Delete(ctx, entry("g", "k")).Return(errors.New("unreachable")).Once()

	s.NoError(s.cache.Set(ctx, entry("g", "k"), "v"))
	s.NoError(s.cache.Delete(ctx, entry("g", "k")))
}

// A failed DeleteGroup is returned, unlike a failed write. This is how a caller invalidates after
// a policy write, and a failure there leaves stale answers being served, which the caller may want
// to act on rather than never hear about.
func (s *GroupedCacheWrapperTestSuite) TestAFailedInvalidationIsReported() {
	ctx := context.Background()
	unreachable := errors.New("unreachable")
	s.impl.EXPECT().IsEnabled().Return(true)
	s.impl.EXPECT().DeleteGroup(ctx, "g").Return(unreachable).Once()

	s.ErrorIs(s.cache.DeleteGroup(ctx, "g"), unreachable)
}

// A cache switched off in configuration answers every read with a miss and accepts every write,
// without reaching an implementation that may not exist.
func (s *GroupedCacheWrapperTestSuite) TestADisabledCacheNeverReachesTheImplementation() {
	ctx := context.Background()
	disabled := &GroupedCache[string]{enabled: false, cacheName: "TestOff", cacheImpl: nil}

	s.False(disabled.IsEnabled())
	s.NoError(disabled.Set(ctx, entry("g", "k"), "v"))
	_, found := disabled.Get(ctx, entry("g", "k"))
	s.False(found)
	s.NoError(disabled.Delete(ctx, entry("g", "k")))
	s.NoError(disabled.DeleteGroup(ctx, "g"))
	disabled.CleanupExpired()
	s.False(disabled.GetStats().Enabled)
}

// An implementation that reports itself disabled is treated the same way, which is how a Redis
// cache with no reachable client behaves.
func (s *GroupedCacheWrapperTestSuite) TestAnImplementationThatIsOffIsNotCalled() {
	ctx := context.Background()
	s.impl.EXPECT().IsEnabled().Return(false)

	s.NoError(s.cache.Set(ctx, entry("g", "k"), "v"))
	_, found := s.cache.Get(ctx, entry("g", "k"))
	s.False(found)
	s.NoError(s.cache.Delete(ctx, entry("g", "k")))
	s.NoError(s.cache.DeleteGroup(ctx, "g"))
	s.cache.CleanupExpired()

	s.impl.AssertNotCalled(s.T(), "Set", ctx, entry("g", "k"), "v")
	s.impl.AssertNotCalled(s.T(), "DeleteGroup", ctx, "g")
}

// Statistics come from the implementation while there is one.
func (s *GroupedCacheWrapperTestSuite) TestStatsComeFromTheImplementation() {
	s.impl.EXPECT().GetStats().Return(CacheStat{Enabled: true, HitCount: 3}).Once()

	stats := s.cache.GetStats()

	s.True(stats.Enabled)
	s.EqualValues(3, stats.HitCount)
}

// The key prints as group and entry together, which is what a log line needs to identify it.
func (s *GroupedCacheWrapperTestSuite) TestTheKeyPrintsBothHalves() {
	s.Equal("role:role-1/ou-a", entry("role:role-1", "ou-a").ToString())
}

// Asking twice for the same grouped cache returns the same instance, so every caller shares one.
func (s *GroupedCacheWrapperTestSuite) TestTheFactoryReturnsOneInstancePerName() {
	cm := Initialize(engineconfig.CacheConfig{TTL: 60, Size: 10}, "test-deployment")

	first := GetGroupedCache[string](cm, "SharedName")
	second := GetGroupedCache[string](cm, "SharedName")

	s.Require().NotNil(first)
	s.Same(first, second)
	s.True(first.IsEnabled())
}

// A grouped cache and a plain cache of the same name are separate instances. They share the
// manager's registry, so the factory has to keep their entries apart.
func (s *GroupedCacheWrapperTestSuite) TestAGroupedCacheIsNotThePlainCacheOfTheSameName() {
	cm := Initialize(engineconfig.CacheConfig{TTL: 60, Size: 10}, "test-deployment")

	grouped := GetGroupedCache[string](cm, "SameName")
	plain := GetCache[string](cm, "SameName")

	s.Require().NotNil(grouped)
	s.Require().NotNil(plain)
	s.NotSame(any(grouped), any(plain))
}

// Caching switched off globally, or for this cache alone, yields a cache that answers misses.
func (s *GroupedCacheWrapperTestSuite) TestConfigurationCanSwitchTheCacheOff() {
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		cfg  engineconfig.CacheConfig
	}{
		{"all caching disabled", engineconfig.CacheConfig{Disabled: true}},
		{
			"this cache disabled",
			engineconfig.CacheConfig{
				TTL:        60,
				Properties: []engineconfig.CacheProperty{{Name: "OffCache", Disabled: true}},
			},
		},
	} {
		s.Run(tc.name, func() {
			c := GetGroupedCache[string](Initialize(tc.cfg, "test-deployment"), "OffCache")

			s.Require().NotNil(c)
			s.False(c.IsEnabled())
			s.NoError(c.Set(ctx, entry("g", "k"), "v"))
			_, found := c.Get(ctx, entry("g", "k"))
			s.False(found)
		})
	}
}

// Redis configured without a reachable client leaves the cache off rather than failing startup.
func (s *GroupedCacheWrapperTestSuite) TestRedisWithoutAClientLeavesTheCacheOff() {
	cm := Initialize(engineconfig.CacheConfig{Type: "redis", TTL: 60}, "test-deployment")

	c := GetGroupedCache[string](cm, "RedisNoClient")

	s.Require().NotNil(c)
	s.False(c.IsEnabled())
}

// An unrecognized cache type falls back to in-memory rather than leaving the cache unusable.
func (s *GroupedCacheWrapperTestSuite) TestAnUnknownTypeFallsBackToInMemory() {
	cm := Initialize(engineconfig.CacheConfig{Type: "nonsense", TTL: 60, Size: 10}, "test-deployment")

	c := GetGroupedCache[string](cm, "UnknownType")

	s.Require().NotNil(c)
	s.True(c.IsEnabled())
	s.Require().NoError(c.Set(context.Background(), entry("g", "k"), "v"))
	got, found := c.Get(context.Background(), entry("g", "k"))
	s.True(found)
	s.Equal("v", got)
}

// Callers race to obtain a cache during startup, and all of them must end up with the same one.
// A second instance would mean two halves of the deployment caching into different maps, so an
// invalidation through one would leave the other serving stale answers.
func (s *GroupedCacheWrapperTestSuite) TestConcurrentCallersShareOneInstance() {
	cm := Initialize(engineconfig.CacheConfig{TTL: 60, Size: 10}, "test-deployment")

	const callers = 16
	got := make([]GroupedCacheInterface[string], callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() { defer wg.Done(); got[i] = GetGroupedCache[string](cm, "RacedName") }()
	}
	wg.Wait()

	s.Require().NotNil(got[0])
	for i := 1; i < callers; i++ {
		s.Same(got[0], got[i], "a caller received a second instance of the same cache")
	}
}
