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

package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// HTTPClientTestSuite defines the test suite for HTTP client service.
type HTTPClientTestSuite struct {
	suite.Suite
}

// TestHTTPClientSuite runs the HTTP client test suite.
func TestHTTPClientSuite(t *testing.T) {
	suite.Run(t, new(HTTPClientTestSuite))
}

func (suite *HTTPClientTestSuite) SetupSuite() {
	thunderConfig := &config.Config{
		TLS: config.TLSConfig{
			MinVersion: "1.3",
		},
	}
	err := config.InitializeServerRuntime("", thunderConfig)
	assert.NoError(suite.T(), err)
}

func (suite *HTTPClientTestSuite) TestNewHTTPClient() {
	client := NewHTTPClient()
	assert.NotNil(suite.T(), client)
	assert.Implements(suite.T(), (*HTTPClientInterface)(nil), client)
}

func (suite *HTTPClientTestSuite) TestNewHTTPClientWithTimeout() {
	timeout := 5 * time.Second
	client := NewHTTPClientWithTimeout(timeout)
	assert.NotNil(suite.T(), client)
	assert.Implements(suite.T(), (*HTTPClientInterface)(nil), client)

	// Verify timeout is set correctly
	httpClient := client.(*HTTPClient)
	assert.Equal(suite.T(), timeout, httpClient.client.Timeout)
}

func (suite *HTTPClientTestSuite) TestNewHTTPClientWithDefaultSettings() {
	// Test default behavior when no client is provided
	client := NewHTTPClient()
	assert.NotNil(suite.T(), client)
	assert.Implements(suite.T(), (*HTTPClientInterface)(nil), client)

	// Verify default timeout is set
	httpClient := client.(*HTTPClient)
	assert.Equal(suite.T(), 30*time.Second, httpClient.client.Timeout)
}

func (suite *HTTPClientTestSuite) TestDo() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	}))
	defer testServer.Close()

	client := NewHTTPClient()

	// Create a request
	req, err := http.NewRequest("GET", testServer.URL, nil)
	assert.NoError(suite.T(), err)

	// Execute the request
	resp, err := client.Do(req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestDoWithPost() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "POST", r.Method)
		assert.Equal(suite.T(), "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer testServer.Close()

	client := NewHTTPClient()

	// Create a POST request
	req, err := http.NewRequest("POST", testServer.URL, strings.NewReader(`{"test": "data"}`))
	assert.NoError(suite.T(), err)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := client.Do(req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestDoWithTimeout() {
	// Create a test server that delays response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create client with short timeout
	client := NewHTTPClientWithTimeout(100 * time.Millisecond)

	// Create a request
	req, err := http.NewRequest("GET", testServer.URL, nil)
	assert.NoError(suite.T(), err)

	// Execute the request - should timeout
	resp, err := client.Do(req)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	assert.Contains(suite.T(), err.Error(), "deadline exceeded")
}

func (suite *HTTPClientTestSuite) TestGet() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("get response"))
	}))
	defer testServer.Close()

	client := NewHTTPClient()

	// Execute the GET request
	resp, err := client.Get(testServer.URL)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestDoWithError() {
	// Create a test server and immediately close it to ensure the
	// connection attempt fails without relying on external network
	// conditions.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	testServer.Close()

	client := NewHTTPClient()

	// Create a request to the closed server
	req, err := http.NewRequest("GET", testServer.URL, nil)
	assert.NoError(suite.T(), err)

	// Execute the request - should fail because the server is closed
	resp, err := client.Do(req)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

func (suite *HTTPClientTestSuite) TestHead() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "HEAD", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	client := NewHTTPClient()

	// Execute the HEAD request
	resp, err := client.Head(testServer.URL)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestPost() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "POST", r.Method)
		assert.Equal(suite.T(), "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), `{"test": "data"}`, string(body))

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer testServer.Close()

	client := NewHTTPClient()

	// Execute the POST request
	resp, err := client.Post(testServer.URL, "application/json", strings.NewReader(`{"test": "data"}`))
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	_ = resp.Body.Close()
}

func (suite *HTTPClientTestSuite) TestSSRFSafeDialContext() {
	// IP literals — LookupIPAddr returns them directly without DNS, so this exercises
	// the same validation path as a hostname that DNS-resolves to a private address.
	blockedAddrs := []string{
		"127.0.0.1:443",       // IPv4 loopback
		"169.254.169.254:443", // IPv4 link-local (cloud metadata)
		"10.0.0.1:443",        // RFC1918
		"172.16.0.1:443",      // RFC1918
		"192.168.1.1:443",     // RFC1918
		"[::1]:443",           // IPv6 loopback
		"[fc00::1]:443",       // IPv6 unique-local (ULA)
		"[fe80::1]:443",       // IPv6 link-local
	}
	for _, addr := range blockedAddrs {
		_, err := ssrfSafeDialContext(context.Background(), "tcp", addr)
		assert.ErrorContains(suite.T(), err, "private address", "addr %s should be blocked", addr)
	}

	// Public IP: SSRF check passes; use an already-canceled context so the dial fails
	// immediately and deterministically with context.Canceled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ssrfSafeDialContext(ctx, "tcp", "1.1.1.1:443")
	assert.Error(suite.T(), err)
	assert.NotContains(suite.T(), err.Error(), "private address")
	assert.NotContains(suite.T(), err.Error(), "resolved to no usable")
}

func (suite *HTTPClientTestSuite) TestPostForm() {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(suite.T(), "POST", r.Method)
		assert.Equal(suite.T(), "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), "value1", r.FormValue("key1"))
		assert.Equal(suite.T(), "value2", r.FormValue("key2"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("form received"))
	}))
	defer testServer.Close()

	client := NewHTTPClient()

	// Prepare form data
	formData := url.Values{}
	formData.Set("key1", "value1")
	formData.Set("key2", "value2")

	// Execute the PostForm request
	resp, err := client.PostForm(testServer.URL, formData)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}
