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
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"sync"
	"sync/atomic"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ServerConfigReader reads the decoded read-only and writable layers of a server-config section.
type ServerConfigReader interface {
	GetReadOnlyConfig(ctx context.Context, name string) (any, *common.ServiceError)
	GetWritableConfig(ctx context.Context, name string) (any, *common.ServiceError)
}

// configSectionCORS is the section the dynamic matcher reads origins from.
const configSectionCORS = "cors"

// builtMatcher holds the once-compiled read-only matcher. A nil matcher means the layer contributes no
// origins (unset or malformed); it is cached either way so the read-only layer compiles at most once.
type builtMatcher struct {
	matcher *Matcher
}

// dynamicCachedMatcher pairs the combined matcher with the writable-layer signature it was built from.
// empty distinguishes an empty writable layer from one whose signature is coincidentally zero.
type dynamicCachedMatcher struct {
	sig      uint64
	empty    bool
	combined *Matcher
}

// dynamicMatcherState stores the runtime CORS matchers and serializes recompilation: the read-only layer
// is compiled once (readonly); only the writable layer is re-checked per request (cache).
type dynamicMatcherState struct {
	reader   ServerConfigReader
	readonly atomic.Pointer[builtMatcher]
	cache    atomic.Pointer[dynamicCachedMatcher]
	mu       sync.Mutex
}

// dynamic is the process-wide dynamic matcher state.
var dynamic dynamicMatcherState

// InitializeDynamicMatcher installs the server-config reader and clears any compiled matchers so the next
// request rebuilds from scratch.
func InitializeDynamicMatcher(reader ServerConfigReader) {
	dynamic.mu.Lock()
	defer dynamic.mu.Unlock()
	dynamic.reader = reader
	dynamic.readonly.Store(nil)
	dynamic.cache.Store(nil)
}

// GetDynamicMatcher returns the current CORS matcher, recompiling the writable layer when its origins
// change. A nil matcher means no origins are allowed.
func GetDynamicMatcher(ctx context.Context) *Matcher {
	return dynamic.resolve(ctx)
}

func (d *dynamicMatcherState) resolve(ctx context.Context) *Matcher {
	reader := d.reader
	if reader == nil {
		return nil
	}
	ro := d.readOnlyMatcher(ctx, reader)
	if ro == nil {
		// The read-only layer could not be read (transient); deny this request and retry on the next.
		return nil
	}
	return d.writableCombined(ctx, reader, ro)
}

// readOnlyMatcher returns the once-compiled read-only matcher, building it on first use. It returns nil
// only when the layer cannot be read (transient — denied without caching, so retried next request). A
// malformed layer is isolated: cached as a nil matcher, so the request still falls through to writable.
func (d *dynamicMatcherState) readOnlyMatcher(ctx context.Context, reader ServerConfigReader) *builtMatcher {
	if ro := d.readonly.Load(); ro != nil {
		return ro
	}
	v, svcErr := reader.GetReadOnlyConfig(ctx, configSectionCORS)
	if svcErr != nil {
		logUnavailable("Failed to read the CORS read-only server-config layer", log.String("code", svcErr.Code))
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if ro := d.readonly.Load(); ro != nil {
		return ro
	}
	var m *Matcher
	if cfg, ok := v.(OriginConfig); !ok {
		logUnavailable("CORS read-only config value has an unexpected type",
			log.String("type", fmt.Sprintf("%T", v)))
	} else if compiled, err := CompileMatcher(cfg.AllowedOrigins); err != nil {
		logUnavailable("Failed to compile the CORS read-only allowed origins", log.Error(err))
	} else {
		warnUnanchoredRegexes(cfg.AllowedOrigins)
		m = compiled
	}
	ro := &builtMatcher{matcher: m}
	d.readonly.Store(ro)
	return ro
}

// writableCombined combines the writable layer with the read-only matcher, recompiling only when the
// writable signature changes. A read/type error denies the request; a compile error denies and caches the
// bad signature so it is not retried at the same value.
func (d *dynamicMatcherState) writableCombined(ctx context.Context, reader ServerConfigReader,
	ro *builtMatcher) *Matcher {
	v, svcErr := reader.GetWritableConfig(ctx, configSectionCORS)
	if svcErr != nil {
		logUnavailable("Failed to read the CORS writable server-config layer", log.String("code", svcErr.Code))
		return nil
	}
	cfg, ok := v.(OriginConfig)
	if !ok {
		logUnavailable("CORS writable config value has an unexpected type",
			log.String("type", fmt.Sprintf("%T", v)))
		return nil
	}

	empty := len(cfg.AllowedOrigins) == 0
	var sig uint64
	if !empty {
		sig = signature(cfg.AllowedOrigins)
	}

	if c := d.cache.Load(); c != nil && c.sig == sig && c.empty == empty {
		return c.combined
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if c := d.cache.Load(); c != nil && c.sig == sig && c.empty == empty {
		return c.combined
	}

	combined := ro.matcher
	if !empty {
		wm, err := CompileMatcher(cfg.AllowedOrigins)
		if err != nil {
			// Cache the bad signature so repeated requests do not recompile the same invalid value.
			d.cache.Store(&dynamicCachedMatcher{sig: sig, empty: empty})
			logUnavailable("Failed to compile the CORS writable allowed origins", log.Error(err))
			return nil
		}
		warnUnanchoredRegexes(cfg.AllowedOrigins)
		combined = combine(ro.matcher, wm)
	}
	d.cache.Store(&dynamicCachedMatcher{sig: sig, empty: empty, combined: combined})
	return combined
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

// signature returns a stable digest used to detect changes to the origin list.
func signature(entries OriginEntries) uint64 {
	h := fnv.New64a()
	var lenBuf [8]byte
	for _, e := range entries {
		key := entryKey(e)
		binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(key)))
		_, _ = h.Write(lenBuf[:])
		_, _ = io.WriteString(h, key)
	}
	return h.Sum64()
}
