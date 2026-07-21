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

package cors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeReader adapts per-layer functions to ServerConfigReader so a test can vary the read-only and
// writable layers independently across resolve calls. A nil layer function yields an empty OriginConfig.
type fakeReader struct {
	readOnly func() (any, *common.ServiceError)
	writable func() (any, *common.ServiceError)
}

func (r fakeReader) GetReadOnlyConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	if r.readOnly != nil {
		return r.readOnly()
	}
	return OriginConfig{AllowedOrigins: OriginEntries{}}, nil
}

func (r fakeReader) GetWritableConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	if r.writable != nil {
		return r.writable()
	}
	return OriginConfig{AllowedOrigins: OriginEntries{}}, nil
}

// writableReader serves an empty read-only layer and a writable layer from the given function.
func writableReader(writable func() (any, *common.ServiceError)) fakeReader {
	return fakeReader{writable: writable}
}

// originConfig builds an OriginConfig from literal origin strings, for the dynamic matcher tests.
func originConfig(origins ...string) OriginConfig {
	entries := make(OriginEntries, len(origins))
	for i, o := range origins {
		entries[i] = literalEntry{Value: o}
	}
	return OriginConfig{AllowedOrigins: entries}
}

func mustMatch(t *testing.T, m *Matcher, origin string) bool {
	t.Helper()
	parsed, err := ParseOrigin(origin)
	require.NoError(t, err)
	allow, _ := m.Match(parsed)
	return allow
}

func TestGetDynamicMatcher_NilReaderReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(nil)
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_CompilesAndMatches(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) {
		return originConfig("https://app.example.com"), nil
	}))

	m := GetDynamicMatcher(context.Background())
	require.NotNil(t, m)
	assert.True(t, mustMatch(t, m, "https://app.example.com"))
	assert.False(t, mustMatch(t, m, "https://other.example.com"))
}

func TestGetDynamicMatcher_ReadOnlyAndWritableBothMatch(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(fakeReader{
		readOnly: func() (any, *common.ServiceError) { return originConfig("https://static.example.com"), nil },
		writable: func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	m := GetDynamicMatcher(context.Background())
	require.NotNil(t, m)
	assert.True(t, mustMatch(t, m, "https://static.example.com")) // read-only layer
	assert.True(t, mustMatch(t, m, "https://app.example.com"))    // writable layer
	assert.False(t, mustMatch(t, m, "https://other.example.com"))
}

func TestGetDynamicMatcher_ReadOnlyCompiledOnce(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	readOnlyCalls := 0
	InitializeDynamicMatcher(fakeReader{
		readOnly: func() (any, *common.ServiceError) {
			readOnlyCalls++
			return originConfig("https://static.example.com"), nil
		},
		writable: func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	for range 3 {
		require.NotNil(t, GetDynamicMatcher(context.Background()))
	}
	// The static read-only layer is fetched and compiled exactly once, not on every request.
	assert.Equal(t, 1, readOnlyCalls)
}

func TestGetDynamicMatcher_UnchangedReusesMatcher(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) {
		return originConfig("https://app.example.com"), nil
	}))

	// The same writable value across calls must yield the identical combined matcher (no recompilation).
	assert.Same(t, GetDynamicMatcher(context.Background()), GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_ChangeRecompiles(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	current := originConfig("https://a.example.com")
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) { return current, nil }))

	first := GetDynamicMatcher(context.Background())
	require.NotNil(t, first)

	current = originConfig("https://b.example.com")
	second := GetDynamicMatcher(context.Background())
	require.NotNil(t, second)

	assert.NotSame(t, first, second)
	assert.True(t, mustMatch(t, second, "https://b.example.com"))
	assert.False(t, mustMatch(t, second, "https://a.example.com")) // removed origin no longer allowed
}

func TestGetDynamicMatcher_ClearRevokes(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	current := originConfig("https://app.example.com")
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) { return current, nil }))

	require.True(t, mustMatch(t, GetDynamicMatcher(context.Background()), "https://app.example.com"))

	current = OriginConfig{AllowedOrigins: OriginEntries{}}
	cleared := GetDynamicMatcher(context.Background())
	require.NotNil(t, cleared)
	assert.False(t, mustMatch(t, cleared, "https://app.example.com"))
}

func TestGetDynamicMatcher_EmptyThenPopulated(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	current := OriginConfig{AllowedOrigins: OriginEntries{}}
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) { return current, nil }))

	// Starts empty: the matcher allows nothing.
	empty := GetDynamicMatcher(context.Background())
	require.NotNil(t, empty)
	assert.False(t, mustMatch(t, empty, "https://app.example.com"))

	// Adding a writable origin drives the empty→non-empty transition and starts allowing it.
	current = originConfig("https://app.example.com")
	populated := GetDynamicMatcher(context.Background())
	require.NotNil(t, populated)
	assert.True(t, mustMatch(t, populated, "https://app.example.com"))
}

func TestGetDynamicMatcher_WritableBadEntriesReturnNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	current := originConfig("https://good.example.com")
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) { return current, nil }))

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// Invalid regex entries remove the matcher until the config is fixed.
	current = OriginConfig{AllowedOrigins: OriginEntries{regexEntry{Pattern: "("}}}
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_WritableWrongTypeReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	var writable any = originConfig("https://good.example.com")
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) { return writable, nil }))

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// Unexpected writable values are rejected (fail-closed).
	writable = "not an origin config"
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_WritableReadErrorReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	fail := false
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) {
		if fail {
			return nil, &common.InternalServerError
		}
		return originConfig("https://app.example.com"), nil
	}))

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// Read errors remove the matcher even if an earlier value compiled.
	fail = true
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_RecoversAfterError(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	fail := false
	InitializeDynamicMatcher(writableReader(func() (any, *common.ServiceError) {
		if fail {
			return nil, &common.InternalServerError
		}
		return originConfig("https://app.example.com"), nil
	}))

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	fail = true
	require.Nil(t, GetDynamicMatcher(context.Background()))

	// Once reads succeed again the matcher resumes evaluating origins.
	fail = false
	recovered := GetDynamicMatcher(context.Background())
	require.NotNil(t, recovered)
	assert.True(t, mustMatch(t, recovered, "https://app.example.com"))
}

func TestGetDynamicMatcher_ReadOnlyFetchErrorDeniesThenRecovers(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	fail := true
	InitializeDynamicMatcher(fakeReader{
		readOnly: func() (any, *common.ServiceError) {
			if fail {
				return nil, &common.InternalServerError
			}
			return originConfig("https://static.example.com"), nil
		},
		writable: func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	// A read-only fetch error denies the request and is not cached, so the next request retries.
	assert.Nil(t, GetDynamicMatcher(context.Background()))

	fail = false
	m := GetDynamicMatcher(context.Background())
	require.NotNil(t, m)
	assert.True(t, mustMatch(t, m, "https://static.example.com"))
	assert.True(t, mustMatch(t, m, "https://app.example.com"))
}

func TestGetDynamicMatcher_ReadOnlyWrongTypeFallsThroughToWritable(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(fakeReader{
		readOnly: func() (any, *common.ServiceError) { return "not an origin config", nil },
		writable: func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	// A malformed read-only layer is isolated: it contributes nothing, but the writable layer still works.
	m := GetDynamicMatcher(context.Background())
	require.NotNil(t, m)
	assert.True(t, mustMatch(t, m, "https://app.example.com"))
	assert.False(t, mustMatch(t, m, "https://static.example.com"))
}

func TestGetDynamicMatcher_ReadOnlyCompileErrorFallsThroughToWritable(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(fakeReader{
		readOnly: func() (any, *common.ServiceError) {
			return OriginConfig{AllowedOrigins: OriginEntries{regexEntry{Pattern: "("}}}, nil
		},
		writable: func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	// A read-only layer that fails to compile is isolated; the writable layer still works.
	m := GetDynamicMatcher(context.Background())
	require.NotNil(t, m)
	assert.True(t, mustMatch(t, m, "https://app.example.com"))
}
