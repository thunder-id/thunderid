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

// readerFunc adapts a function to the MergedConfigReader interface for tests, so a test can vary the merged
// value (and error) returned across successive resolve calls.
type readerFunc func() (any, *common.ServiceError)

func (r readerFunc) GetMergedConfig(_ context.Context, _ string) (any, *common.ServiceError) {
	return r()
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
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) {
		return originConfig("https://app.example.com"), nil
	}))

	m := GetDynamicMatcher(context.Background())
	require.NotNil(t, m)
	assert.True(t, mustMatch(t, m, "https://app.example.com"))
	assert.False(t, mustMatch(t, m, "https://other.example.com"))
}

func TestGetDynamicMatcher_UnchangedReusesMatcher(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) {
		return originConfig("https://app.example.com"), nil
	}))

	// The same merged value across calls must yield the identical compiled matcher (no recompilation).
	assert.Same(t, GetDynamicMatcher(context.Background()), GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_ChangeRecompiles(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	current := originConfig("https://a.example.com")
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) { return current, nil }))

	first := GetDynamicMatcher(context.Background())
	require.NotNil(t, first)

	current = originConfig("https://b.example.com")
	second := GetDynamicMatcher(context.Background())
	require.NotNil(t, second)

	assert.NotSame(t, first, second)
	assert.True(t, mustMatch(t, second, "https://b.example.com"))
	assert.False(t, mustMatch(t, second, "https://a.example.com"))
}

func TestGetDynamicMatcher_ClearRevokes(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	current := originConfig("https://app.example.com")
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) { return current, nil }))

	require.True(t, mustMatch(t, GetDynamicMatcher(context.Background()), "https://app.example.com"))

	current = OriginConfig{AllowedOrigins: OriginEntries{}}
	cleared := GetDynamicMatcher(context.Background())
	require.NotNil(t, cleared)
	assert.False(t, mustMatch(t, cleared, "https://app.example.com"))
}

func TestGetDynamicMatcher_BadEntriesReturnNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	current := originConfig("https://good.example.com")
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) { return current, nil }))

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// Invalid regex entries remove the matcher until the config is fixed.
	current = OriginConfig{AllowedOrigins: OriginEntries{regexEntry{Pattern: "("}}}
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_WrongTypeReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	var merged any = originConfig("https://good.example.com")
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) { return merged, nil }))

	require.NotNil(t, GetDynamicMatcher(context.Background()))

	// Unexpected merged values are rejected.
	merged = "not an origin config"
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_ReadErrorReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	fail := false
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) {
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

func TestGetDynamicMatcher_ReadErrorBeforeAnyGoodReturnsNil(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) {
		return nil, &common.InternalServerError
	}))
	assert.Nil(t, GetDynamicMatcher(context.Background()))
}

func TestGetDynamicMatcher_RecoversAfterError(t *testing.T) {
	t.Cleanup(func() { InitializeDynamicMatcher(nil) })
	fail := false
	InitializeDynamicMatcher(readerFunc(func() (any, *common.ServiceError) {
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
