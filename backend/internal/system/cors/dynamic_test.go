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

// fakeReader adapts functions to the ServerConfigReader interface for tests, so a test can vary the version
// and merged value (and errors) returned across successive resolve calls.
type fakeReader struct {
	version func() (int, *common.ServiceError)
	merged  func() (any, *common.ServiceError)
}

func (r fakeReader) GetConfigVersion(_ context.Context, _ string) (int, *common.ServiceError) {
	return r.version()
}

func (r fakeReader) GetMergedConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	return r.merged()
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
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return 1, nil },
		merged:  func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	m := GetDynamicMatcher(context.Background())
	require.NotNil(t, m)
	assert.True(t, mustMatch(t, m, "https://app.example.com"))
	assert.False(t, mustMatch(t, m, "https://other.example.com"))
}

func TestGetDynamicMatcher_UnchangedReusesMatcher(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	mergedCalls := 0
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return 1, nil },
		merged: func() (any, *common.ServiceError) {
			mergedCalls++
			return originConfig("https://app.example.com"), nil
		},
	})

	// A stable version yields the identical compiled matcher without re-reading the merged config.
	first := GetDynamicMatcher(context.Background())
	second := GetDynamicMatcher(context.Background())
	assert.Same(t, first, second)
	assert.Equal(t, 1, mergedCalls)
}

func TestGetDynamicMatcher_ChangeRecompiles(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	version := 1
	current := originConfig("https://a.example.com")
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return version, nil },
		merged:  func() (any, *common.ServiceError) { return current, nil },
	})

	first := GetDynamicMatcher(context.Background())
	require.NotNil(t, first)

	version = 2
	current = originConfig("https://b.example.com")
	second := GetDynamicMatcher(context.Background())
	require.NotNil(t, second)

	assert.NotSame(t, first, second)
	assert.True(t, mustMatch(t, second, "https://b.example.com"))
	assert.False(t, mustMatch(t, second, "https://a.example.com"))
}

func TestGetDynamicMatcher_ClearRevokes(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	version := 1
	current := originConfig("https://app.example.com")
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return version, nil },
		merged:  func() (any, *common.ServiceError) { return current, nil },
	})

	require.True(t, mustMatch(t, GetDynamicMatcher(context.Background()), "https://app.example.com"))

	version = 2
	current = OriginConfig{AllowedOrigins: OriginEntries{}}
	cleared := GetDynamicMatcher(context.Background())
	require.NotNil(t, cleared)
	assert.False(t, mustMatch(t, cleared, "https://app.example.com"))
}

func TestGetDynamicMatcher_BadEntriesReturnNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	version := 1
	current := originConfig("https://good.example.com")
	mergedCalls := 0
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return version, nil },
		merged: func() (any, *common.ServiceError) {
			mergedCalls++
			return current, nil
		},
	})

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// Invalid regex entries remove the matcher until the config is fixed.
	version = 2
	current = OriginConfig{AllowedOrigins: OriginEntries{regexEntry{Pattern: "("}}}
	assert.Nil(t, GetDynamicMatcher(context.Background()))

	// The bad version is cached: a second call at the same version returns nil without recompiling.
	callsAfterBad := mergedCalls
	assert.Nil(t, GetDynamicMatcher(context.Background()))
	assert.Equal(t, callsAfterBad, mergedCalls)
}

func TestGetDynamicMatcher_WrongTypeReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	version := 1
	var merged any = originConfig("https://good.example.com")
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return version, nil },
		merged:  func() (any, *common.ServiceError) { return merged, nil },
	})

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// Unexpected merged values are rejected.
	version = 2
	merged = "not an origin config"
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_ReadErrorReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	fail := false
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) {
			if fail {
				return 0, &common.InternalServerError
			}
			return 1, nil
		},
		merged: func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// A failed version read removes the matcher even if an earlier value compiled.
	fail = true
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_MergedReadErrorReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	version := 1
	mergedFail := false
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return version, nil },
		merged: func() (any, *common.ServiceError) {
			if mergedFail {
				return nil, &common.InternalServerError
			}
			return originConfig("https://app.example.com"), nil
		},
	})

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// A new version forces a re-read; a failed merged read removes the matcher.
	version = 2
	mergedFail = true
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_ReadErrorBeforeAnyGoodReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) { return 0, &common.InternalServerError },
		merged:  func() (any, *common.ServiceError) { return nil, &common.InternalServerError },
	})
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_RecoversAfterError(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	fail := false
	InitializeDynamicMatcher(fakeReader{
		version: func() (int, *common.ServiceError) {
			if fail {
				return 0, &common.InternalServerError
			}
			return 1, nil
		},
		merged: func() (any, *common.ServiceError) { return originConfig("https://app.example.com"), nil },
	})

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	fail = true
	require.Nil(t, GetDynamicMatcher(context.Background()))

	// Once reads succeed again the matcher resumes evaluating origins.
	fail = false
	recovered := GetDynamicMatcher(context.Background())
	require.NotNil(t, recovered)
	assert.True(t, mustMatch(t, recovered, "https://app.example.com"))
}
