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

// Package cors parses, compiles, and matches CORS allowed origins.
package cors

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ServerConfigReader reads the merged value and change-detection version of a server-config section.
type ServerConfigReader interface {
	GetMergedConfig(ctx context.Context, name string) (any, *common.ServiceError)
	GetConfigVersion(ctx context.Context, name string) (int, *common.ServiceError)
}

// configSectionCORS is the section the dynamic matcher reads origins from.
const configSectionCORS = "cors"

// dynamicCachedMatcher pairs a compiled matcher with the config version it was built from.
type dynamicCachedMatcher struct {
	version int
	matcher *Matcher
}

// dynamicMatcherState stores the runtime CORS matcher and serializes recompilation.
type dynamicMatcherState struct {
	reader ServerConfigReader
	cache  atomic.Pointer[dynamicCachedMatcher]
	mu     sync.Mutex
}

// dynamic is the process-wide dynamic matcher state.
var dynamic dynamicMatcherState

// InitializeDynamicMatcher installs the server-config reader used by CORS matching.
func InitializeDynamicMatcher(reader ServerConfigReader) {
	dynamic.mu.Lock()
	defer dynamic.mu.Unlock()
	dynamic.reader = reader
	dynamic.cache.Store(nil)
}

// GetDynamicMatcher returns the current CORS matcher, recompiling it when the config section's version
// changes. A nil matcher means no origins are allowed.
func GetDynamicMatcher(ctx context.Context) *Matcher {
	return dynamic.resolve(ctx)
}

func (d *dynamicMatcherState) resolve(ctx context.Context) *Matcher {
	reader := d.reader
	if reader == nil {
		return nil
	}
	currentVersion, svcErr := reader.GetConfigVersion(ctx, configSectionCORS)
	if svcErr != nil {
		logUnavailable("Failed to read the CORS server-config version", log.String("code", svcErr.Code))
		return nil
	}
	if c := d.cache.Load(); c != nil && c.version == currentVersion {
		return c.matcher
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if c := d.cache.Load(); c != nil && c.version == currentVersion {
		return c.matcher
	}
	merged, svcErr := reader.GetMergedConfig(ctx, configSectionCORS)
	if svcErr != nil {
		logUnavailable("Failed to read the CORS server-config section", log.String("code", svcErr.Code))
		return nil
	}
	cfg, ok := merged.(OriginConfig)
	if !ok {
		logUnavailable("CORS merged config value has an unexpected type",
			log.String("type", fmt.Sprintf("%T", merged)))
		return nil
	}
	m, err := CompileMatcher(cfg.AllowedOrigins)
	if err != nil {
		// Cache the version with a nil matcher so repeated requests do not recompile the same invalid value.
		d.cache.Store(&dynamicCachedMatcher{version: currentVersion, matcher: nil})
		logUnavailable("Failed to compile the CORS allowed origins", log.Error(err))
		return nil
	}
	warnUnanchoredRegexes(cfg.AllowedOrigins)
	d.cache.Store(&dynamicCachedMatcher{version: currentVersion, matcher: m})
	return m
}

// logUnavailable records why CORS matching is unavailable.
func logUnavailable(cause string, fields ...log.Field) {
	corsLogger().Warn(context.Background(), cause+"; denying all cross-origin requests", fields...)
}

// warnUnanchoredRegexes logs a warning for each regex entry that is not fully anchored.
func warnUnanchoredRegexes(entries OriginEntries) {
	for i, e := range entries {
		rx, ok := e.(regexEntry)
		if !ok {
			continue
		}
		if !isRegexAnchored(rx.Pattern) {
			corsLogger().Warn(context.Background(),
				"CORS allowedOrigins regex is not fully anchored; partial matches are likely",
				log.Int("index", i),
				log.String("pattern", rx.Pattern))
		}
	}
}

// corsLogger returns the package logger tagged with the CORS component name.
func corsLogger() *log.Logger {
	return log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CORS"))
}
