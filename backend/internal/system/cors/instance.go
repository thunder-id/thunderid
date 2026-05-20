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

// Package cors provides origin parsing, rule compilation, and matching for
// the CORS allowed-origins configuration, plus the process-wide matcher
// singleton that the HTTP middleware reads on every request. The package
// does not depend on the config package and performs no I/O on the request
// path; boot-time diagnostics for misconfigured regex entries are emitted by
// InitializeMatcher.
package cors

import "github.com/thunder-id/thunderid/internal/system/log"

// instance holds the process-wide compiled CORS matcher. It is populated once
// at server start by InitializeMatcher (called from the server bootstrap
// after configuration has been validated) and read on every CORS request via
// GetMatcher.
//
// The field is a plain pointer rather than an atomic — Initialize runs on the
// main goroutine before the HTTP server starts, so there is no concurrent
// reader/writer overlap to guard against.
var instance *Matcher

// InitializeMatcher compiles the given allowed-origin entries and installs the
// resulting Matcher as the process-wide singleton. Compilation errors are
// returned with the offending entry's index so a configuration mistake is
// surfaced at server start rather than on the first cross-origin request; on
// error the previously installed matcher is left unchanged.
//
// InitializeMatcher also emits a boot-time warning for each regex entry whose
// pattern lacks start/end anchors — a common operator mistake that turns the
// pattern into a partial-match filter and almost always allows far more
// origins than intended.
func InitializeMatcher(entries OriginEntries) error {
	rules, err := compileAll(entries)
	if err != nil {
		return err
	}
	m := newMatcher(rules)

	for i, e := range entries {
		rx, ok := e.(regexEntry)
		if !ok {
			continue
		}
		if !isRegexAnchored(rx.Pattern) {
			logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CORS"))
			logger.Warn("cors.allowed_origins regex is not fully anchored; partial matches are likely",
				log.Int("index", i),
				log.String("pattern", rx.Pattern))
		}
	}

	instance = m
	return nil
}

// GetMatcher returns the process-wide CORS matcher installed by
// InitializeMatcher, or nil if Initialize has not run. The middleware treats
// nil as "no origins configured" — the same fail-closed outcome as an empty
// AllowedOrigins list.
func GetMatcher() *Matcher {
	return instance
}
