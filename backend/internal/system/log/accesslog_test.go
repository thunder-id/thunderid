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

	handler := AccessLogHandler(log, testHandler)

	req := httptest.NewRequest("GET", "/test?include=display&token=secret", nil)
	req.RemoteAddr = "192.168.1.1:12345"

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
