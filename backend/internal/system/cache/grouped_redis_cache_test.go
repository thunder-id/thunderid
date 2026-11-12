// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// GroupedRedisCacheTestSuite covers the Redis-backed grouped cache. There is no Redis server in
// unit tests, so what is checked here is the key layout, which is what keeps a group's hash from
// landing on a key another cache holds as a string, and the disabled path.
type GroupedRedisCacheTestSuite struct {
	suite.Suite
}

func TestGroupedRedisCacheSuite(t *testing.T) {
	suite.Run(t, new(GroupedRedisCacheTestSuite))
}

// newDisabledGroupedRedis returns a grouped Redis cache with no client behind it.
func newDisabledGroupedRedis() GroupedCacheInterface[string] {
	return newGroupedRedisCache[string]("TestGroupedRedis", false, nil, "test",
		engineconfig.CacheConfig{TTL: 60}, engineconfig.CacheProperty{})
}

// Without a client every operation is a no-op rather than a panic or an error, so a deployment
// with Redis configured but unreachable degrades to no caching.
func (s *GroupedRedisCacheTestSuite) TestADisabledCacheIsANoOp() {
	ctx := context.Background()
	c := newDisabledGroupedRedis()

	s.False(c.IsEnabled())
	s.Equal("TestGroupedRedis", c.GetName())
	s.NoError(c.Set(ctx, GroupedCacheKey{Group: "g", Key: "k"}, "v"))
	_, found := c.Get(ctx, GroupedCacheKey{Group: "g", Key: "k"})
	s.False(found)
	s.NoError(c.Delete(ctx, GroupedCacheKey{Group: "g", Key: "k"}))
	s.NoError(c.DeleteGroup(ctx, "g"))
	s.False(c.GetStats().Enabled)
}

// A group's hash lives under its own key segment.
func (s *GroupedRedisCacheTestSuite) TestAGroupKeyCarriesItsOwnSegment() {
	c := &groupedRedisCache[string]{name: "SharingVisibilityCache", keyPrefix: "thunderid:dep1"}

	s.Equal("thunderid:dep1:SharingVisibilityCache:g:role:role-1",
		c.buildGroupKey("role:role-1"))
}

// The segment separates the two layouts under one cache name: an ordinary entry key and a group
// name produce different Redis keys, so a hash never lands where a string is held. Redis answers
// every command against a key of the wrong type with WRONGTYPE, which no test of either cache on
// its own would catch.
//
// The separation is by convention, not by construction. The keys are colons joined without
// escaping, so a plain entry keyed literally "g:role:role-1" still reaches the same Redis key as
// the group "role:role-1". That is the same ambiguity two plain caches already have between
// themselves (cache "A" with key "B:x" and cache "A:B" with key "x"), and it is why one cache name
// must not be registered as both kinds.
func (s *GroupedRedisCacheTestSuite) TestAGroupKeyIsDistinctFromAnOrdinaryEntryKey() {
	plain := &redisCache[string]{name: "SharingVisibilityCache", keyPrefix: "thunderid:dep1"}
	grouped := &groupedRedisCache[string]{name: "SharingVisibilityCache", keyPrefix: "thunderid:dep1"}

	s.NotEqual(plain.buildKey(CacheKey{Key: "role:role-1"}), grouped.buildGroupKey("role:role-1"),
		"an entry key and a group of the same name resolved to the same Redis key")
}

// A cache configured with no TTL stores entries that do not expire, the same as the plain Redis
// cache does. The expiry commands must not be sent at all in that case: to Redis a TTL of zero
// means delete now, so HEXPIRE would drop the field just written and EXPIRE the group with it.
func (s *GroupedRedisCacheTestSuite) TestNoConfiguredTTLMeansNoExpiryCommands() {
	withTTL := newGroupedRedisCache[string]("TestTTL", true, nil, "test",
		engineconfig.CacheConfig{TTL: 60}, engineconfig.CacheProperty{})
	withoutTTL := newGroupedRedisCache[string]("TestNoTTL", true, nil, "test",
		engineconfig.CacheConfig{}, engineconfig.CacheProperty{})

	s.Equal(60*time.Second, withTTL.(*groupedRedisCache[string]).ttl)
	s.Zero(withoutTTL.(*groupedRedisCache[string]).ttl,
		"a zero TTL reaches Set, where it must suppress the expiry commands rather than be sent")
}

// newUnreachableGroupedRedis returns an enabled cache whose server does not answer, so every
// command fails. That is what a Redis outage looks like to this layer.
func newUnreachableGroupedRedis[T any]() GroupedCacheInterface[T] {
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 50 * time.Millisecond,
		MaxRetries:  -1,
	})
	return newGroupedRedisCache[T]("TestUnreachable", true, client, "test",
		engineconfig.CacheConfig{TTL: 60}, engineconfig.CacheProperty{})
}

// Every command failing is reported, not hidden. The wrapper above turns a failed read or write
// into a miss; what this layer owes its caller is the error.
func (s *GroupedRedisCacheTestSuite) TestAnUnreachableServerFailsEveryOperation() {
	ctx := context.Background()
	c := newUnreachableGroupedRedis[string]()
	k := GroupedCacheKey{Group: "g", Key: "k"}

	s.Error(c.Set(ctx, k, "v"))
	_, found := c.Get(ctx, k)
	s.False(found, "an unreachable server answers a read with a miss")
	s.Error(c.Delete(ctx, k))
	s.Error(c.DeleteGroup(ctx, "g"))
}

// A read that cannot reach the server counts as a miss, so the hit rate reflects what was actually
// served from the cache rather than what was asked of it.
func (s *GroupedRedisCacheTestSuite) TestAFailedReadCountsAsAMiss() {
	c := newUnreachableGroupedRedis[string]()

	_, _ = c.Get(context.Background(), GroupedCacheKey{Group: "g", Key: "k"})

	stats := c.GetStats()
	s.True(stats.Enabled)
	s.EqualValues(1, stats.MissCount)
	s.Zero(stats.HitCount)
	s.Zero(stats.HitRate)
}

// A value JSON cannot represent is refused before any command is sent, rather than storing
// something that would not read back.
func (s *GroupedRedisCacheTestSuite) TestAnUnmarshalableValueIsRefused() {
	c := newUnreachableGroupedRedis[chan int]()

	err := c.Set(context.Background(), GroupedCacheKey{Group: "g", Key: "k"}, make(chan int))

	s.Require().Error(err)
	s.Contains(err.Error(), "json", "the failure should name the encoding, not the transport")
}

// Redis expires entries itself, so the periodic cleanup has nothing to do.
func (s *GroupedRedisCacheTestSuite) TestCleanupIsANoOp() {
	s.NotPanics(func() { newUnreachableGroupedRedis[string]().CleanupExpired() })
}
