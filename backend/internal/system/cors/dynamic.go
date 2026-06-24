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
	"bytes"
	"context"
	"encoding/json"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// dynamicInstance holds the runtime CORS matcher. It is installed once at server start by
// InitializeDynamicMatcher and read on every CORS request via GetDynamicMatcher.
//
// Like instance, it is a plain var rather than an atomic — installation runs on the main goroutine
// before the HTTP server starts, so there is no concurrent reader/writer overlap to guard against.
var dynamicInstance *dynamicMatcher

// OriginConfigReader returns the current raw JSON origin config and whether it is set. It is supplied
// by the configuration consumer so the cors package never imports the configuration store.
type OriginConfigReader func() (raw []byte, ok bool)

// dynamicMatcher resolves the runtime origin config into a Matcher on demand. It memoizes the
// compiled matcher and recompiles only when the raw config changes since the last compile.
type dynamicMatcher struct {
	read        OriginConfigReader
	logger      *log.Logger
	mu          sync.Mutex
	lastRaw     []byte
	lastMatcher *Matcher
}

// InitializeDynamicMatcher installs the runtime origin-config reader. It mirrors InitializeMatcher
// for the boot-time global matcher and is called once at the composition root, before the HTTP
// server starts. The matcher itself is compiled lazily on the first GetDynamicMatcher call.
func InitializeDynamicMatcher(read OriginConfigReader) {
	dynamicInstance = &dynamicMatcher{
		read:   read,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CORSDynamicMatcher")),
	}
}

// GetDynamicMatcher returns the current runtime CORS matcher, or nil if no dynamic matcher has been
// installed. It mirrors GetMatcher for the boot-time global matcher; the middleware treats nil as
// "no runtime origins configured".
func GetDynamicMatcher() *Matcher {
	if dynamicInstance == nil {
		return nil
	}
	return dynamicInstance.resolve()
}

// resolve returns the memoized matcher, recompiling only when the raw config has changed since the
// last compile. On unset, unparseable, or uncompilable config it keeps the last good matcher
// (fail-safe). It is safe for concurrent use.
func (d *dynamicMatcher) resolve() *Matcher {
	raw, ok := d.read()

	d.mu.Lock()
	defer d.mu.Unlock()

	if !ok {
		return d.lastMatcher
	}
	if d.lastRaw != nil && bytes.Equal(raw, d.lastRaw) {
		return d.lastMatcher
	}

	var entries OriginEntries
	if err := json.Unmarshal(raw, &entries); err != nil {
		d.logger.Warn(context.Background(),
			"Failed to parse runtime CORS config; keeping last-good matcher", log.Error(err))
		return d.lastMatcher
	}
	matcher, err := CompileMatcher(entries)
	if err != nil {
		d.logger.Warn(context.Background(),
			"Failed to compile runtime CORS config; keeping last-good matcher", log.Error(err))
		return d.lastMatcher
	}

	d.lastRaw = raw
	d.lastMatcher = matcher
	return matcher
}
