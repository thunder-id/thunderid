/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package log

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const testRemoteAddr = "192.168.1.1:12345"

type AccessLogTestSuite struct {
	suite.Suite
}

func TestAccessLogSuite(t *testing.T) {
	suite.Run(t, new(AccessLogTestSuite))
}

func (suite *AccessLogTestSuite) TestAccessLogHandler() {
	var buf bytes.Buffer

	logger = nil
	once = sync.Once{}

	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	logHandler := slog.NewTextHandler(&buf, handlerOptions)
	log := &Logger{
		internal: slog.New(logHandler),
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	})

	handler := AccessLogHandler(log, nil, testHandler)

	req := httptest.NewRequest("GET", "/test?include=display&token=secret", nil)
	req.RemoteAddr = testRemoteAddr

	rr := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Verify response
	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	assert.Equal(suite.T(), "OK", rr.Body.String())

	output := buf.String()
	assert.Contains(suite.T(), output, "192.168.1.1")
	assert.Contains(suite.T(), output, "GET /test")
	assert.Contains(suite.T(), output, "200")
	// Verify that query parameters are not written to the access log
	assert.NotContains(suite.T(), output, "include=display")
	assert.NotContains(suite.T(), output, "token=secret")
	// Verify that escape characters are not present in the log output
	assert.NotContains(suite.T(), output, `\"`)
}

func (suite *AccessLogTestSuite) TestAccessLogHandlerSkipPrefixes() {
	var buf bytes.Buffer

	logger = nil
	once = sync.Once{}

	logHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	log := &Logger{
		internal: slog.New(logHandler),
	}

	served := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AccessLogHandler(log, []string{"/gate/", "/console/"}, testHandler)

	// A skipped path is still served, but no access log line is emitted.
	req := httptest.NewRequest("GET", "/console/assets/index.js", nil)
	req.RemoteAddr = testRemoteAddr
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.True(suite.T(), served, "skipped request should still be served")
	assert.Equal(suite.T(), http.StatusOK, rr.Code)
	assert.Empty(suite.T(), buf.String(), "no access log should be written for skipped prefixes")

	// A non-skipped path is logged as usual.
	buf.Reset()
	req = httptest.NewRequest("GET", "/oauth2/token", nil)
	req.RemoteAddr = testRemoteAddr
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Contains(suite.T(), buf.String(), "GET /oauth2/token")
}

func (suite *AccessLogTestSuite) TestAccessLogHandlerSkipPrefixEdgeCases() {
	skipPrefixes := []string{"/gate/", "/console/", "/health/"}

	cases := []struct {
		path       string
		wantLogged bool
	}{
		{"/console/", false},
		{"/console/assets/index.js", false},
		{"/gate/", false},
		{"/gate/signin", false},
		{"/health/liveness", false},
		{"/health/readiness", false},
		// Bare prefixes without a trailing slash are not skipped (e.g. the /console -> /console/ redirect).
		{"/console", true},
		{"/gate", true},
		{"/health", true},
		// Paths that merely share a leading substring must not be skipped.
		{"/consolexyz", true},
		{"/gateway", true},
		{"/healthz", true},
		// Regular API traffic is always logged.
		{"/oauth2/token", true},
		{"/applications", true},
	}

	for _, tc := range cases {
		var buf bytes.Buffer
		logHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		log := &Logger{internal: slog.New(logHandler)}

		handler := AccessLogHandler(log, skipPrefixes, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", tc.path, nil)
		req.RemoteAddr = testRemoteAddr
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(suite.T(), http.StatusOK, rr.Code, "path %q should still be served", tc.path)
		if tc.wantLogged {
			assert.Contains(suite.T(), buf.String(), "GET "+tc.path, "path %q should be logged", tc.path)
		} else {
			assert.Empty(suite.T(), buf.String(), "path %q should not be logged", tc.path)
		}
	}
}

func (suite *AccessLogTestSuite) TestLoggingResponseWriter() {
	rec := httptest.NewRecorder()
	lrw := &loggingResponseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
		size:           0,
	}

	// Test writing headers
	lrw.WriteHeader(http.StatusNotFound)
	assert.Equal(suite.T(), http.StatusNotFound, lrw.statusCode)

	// Test writing content
	n, err := lrw.Write([]byte("test content"))
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 12, n)
	assert.Equal(suite.T(), 12, lrw.size)

	// Write more content
	n, err = lrw.Write([]byte(" more"))
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 5, n)
	assert.Equal(suite.T(), 17, lrw.size) // 12 + 5

	// Verify the actual content was written to the underlying ResponseWriter
	assert.Equal(suite.T(), "test content more", rec.Body.String())
}
