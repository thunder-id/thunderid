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

package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

type HTTPUtilTestSuite struct {
	suite.Suite
}

func TestHTTPUtilSuite(t *testing.T) {
	suite.Run(t, new(HTTPUtilTestSuite))
}

func (suite *HTTPUtilTestSuite) TestWriteJSONError() {
	testCases := []struct {
		name        string
		code        string
		desc        string
		statusCode  int
		respHeaders []map[string]string
	}{
		{
			name:       "BasicError",
			code:       "invalid_request",
			desc:       "The request is missing a required parameter",
			statusCode: http.StatusBadRequest,
			respHeaders: []map[string]string{
				{"X-Custom-Header": "custom-value"},
			},
		},
		{
			name:       "UnauthorizedError",
			code:       "unauthorized",
			desc:       "Authentication is required to access this resource",
			statusCode: http.StatusUnauthorized,
			respHeaders: []map[string]string{
				{"WWW-Authenticate": "Basic"},
			},
		},
		{
			name:        "NoHeaders",
			code:        "server_error",
			desc:        "Internal server error occurred",
			statusCode:  http.StatusInternalServerError,
			respHeaders: []map[string]string{},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteJSONError(w, tc.code, tc.desc, tc.statusCode, tc.respHeaders)

			// Verify status code
			assert.Equal(t, tc.statusCode, w.Code)

			// Verify content type header
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			// Verify custom headers
			for _, headerMap := range tc.respHeaders {
				for key, value := range headerMap {
					assert.Equal(t, value, w.Header().Get(key))
				}
			}

			// Verify response body
			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tc.code, response["error"])
			assert.Equal(t, tc.desc, response["error_description"])
		})
	}
}

func (suite *HTTPUtilTestSuite) TestParseURL() {
	testCases := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "ValidURL",
			url:         "https://example.com/path?query=value",
			expectError: false,
		},
		{
			name:        "ValidURLWithPort",
			url:         "http://localhost:8080/api",
			expectError: false,
		},
		{
			name:        "InvalidURL",
			url:         "://invalid-url",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			parsedURL, err := ParseURL(tc.url)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, parsedURL)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, parsedURL)
				assert.Equal(t, tc.url, parsedURL.String())
			}
		})
	}
}

func (suite *HTTPUtilTestSuite) TestIsValidLogoURI() {
	testCases := []struct {
		name     string
		uri      string
		expected bool
	}{
		{
			name:     "EmptyString",
			uri:      "",
			expected: false,
		},
		{
			name:     "HTTPSUrl",
			uri:      "https://example.com/logo.png",
			expected: true,
		},
		{
			name:     "HTTPUrl",
			uri:      "http://example.com/logo.png",
			expected: true,
		},
		{
			name:     "DataURI",
			uri:      "data:image/png;base64,abc123",
			expected: true,
		},
		{
			name:     "BlobURI",
			uri:      "blob:https://example.com/uuid",
			expected: true,
		},
		{
			name:     "AbsolutePath",
			uri:      "/images/logo.png",
			expected: true,
		},
		{
			name:     "RelativePath",
			uri:      "./logo.png",
			expected: false,
		},
		{
			name:     "RelativePathNoSlash",
			uri:      "logo.png",
			expected: false,
		},
		{
			name:     "InvalidURI",
			uri:      "://invalid",
			expected: false,
		},
		{
			name:     "JavascriptScheme",
			uri:      "javascript:alert(1)",
			expected: false,
		},
		{
			name:     "FileScheme",
			uri:      "file:///etc/passwd",
			expected: false,
		},
		{
			name:     "FTPScheme",
			uri:      "ftp://example.com/logo.png",
			expected: false,
		},
		{
			name:     "EmojiURI",
			uri:      "emoji:smile",
			expected: true,
		},
		{
			name:     "HTTPWithoutHost",
			uri:      "http:///no-host",
			expected: false,
		},
		{
			name:     "HTTPSWithoutHost",
			uri:      "https:///no-host",
			expected: false,
		},
		{
			name:     "JavaScriptScheme",
			uri:      "javascript:alert(1)",
			expected: false,
		},
		{
			name:     "FileScheme",
			uri:      "file:///etc/passwd",
			expected: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := IsValidLogoURI(tc.uri)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *HTTPUtilTestSuite) TestGetURIWithQueryParams() {
	testCases := []struct {
		name        string
		uri         string
		queryParams map[string]string
		expected    string
		expectError bool
	}{
		{
			name:        "NoQueryParams",
			uri:         "https://example.com/path",
			queryParams: map[string]string{},
			expected:    "https://example.com/path",
			expectError: false,
		},
		{
			name: "SingleQueryParam",
			uri:  "https://example.com/path",
			queryParams: map[string]string{
				"param1": "value1",
			},
			expected:    "https://example.com/path?param1=value1",
			expectError: false,
		},
		{
			name: "MultipleQueryParams",
			uri:  "https://example.com/path",
			queryParams: map[string]string{
				"param1": "value1",
				"param2": "value2",
			},
			expected:    "https://example.com/path?param1=value1&param2=value2",
			expectError: false,
		},
		{
			name: "QueryParamsWithExistingParams",
			uri:  "https://example.com/path?existing=value",
			queryParams: map[string]string{
				"param1": "value1",
			},
			expected:    "https://example.com/path?existing=value&param1=value1",
			expectError: false,
		},
		{
			name:        "InvalidURI",
			uri:         "://invalid-uri",
			queryParams: map[string]string{},
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := GetURIWithQueryParams(tc.uri, tc.queryParams)

			if tc.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)

				// Parse both URLs to compare them without caring about parameter order
				expectedURL, err := url.Parse(tc.expected)
				assert.NoError(t, err)

				resultURL, err := url.Parse(result)
				assert.NoError(t, err)

				assert.Equal(t, expectedURL.Scheme, resultURL.Scheme)
				assert.Equal(t, expectedURL.Host, resultURL.Host)
				assert.Equal(t, expectedURL.Path, resultURL.Path)

				// Compare query parameters
				expectedQuery := expectedURL.Query()
				resultQuery := resultURL.Query()

				assert.Equal(t, len(expectedQuery), len(resultQuery))
				for key := range expectedQuery {
					assert.Equal(t, expectedQuery.Get(key), resultQuery.Get(key))
				}
			}
		})
	}
}

type testStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func (suite *HTTPUtilTestSuite) TestDecodeJSONBody() {
	testCases := []struct {
		name        string
		jsonBody    string
		expected    testStruct
		expectError bool
	}{
		{
			name:        "ValidJSON",
			jsonBody:    `{"name":"test","value":123}`,
			expected:    testStruct{Name: "test", Value: 123},
			expectError: false,
		},
		{
			name:        "EmptyJSON",
			jsonBody:    `{}`,
			expected:    testStruct{},
			expectError: false,
		},
		{
			name:        "InvalidJSON",
			jsonBody:    `{"name":"test","value":}`,
			expected:    testStruct{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tc.jsonBody))
			req.Header.Set("Content-Type", "application/json")

			result, err := DecodeJSONBody[testStruct](req)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.expected.Name, result.Name)
				assert.Equal(t, tc.expected.Value, result.Value)
			}
		})
	}
}

func (suite *HTTPUtilTestSuite) TestSanitizeString() {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NormalString",
			input:    "Normal string",
			expected: "Normal string",
		},
		{
			name:     "StringWithHTML",
			input:    "String with <script>alert('XSS')</script> HTML",
			expected: "String with &lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt; HTML",
		},
		{
			name:     "StringWithControlChars",
			input:    "String with control \x00 chars",
			expected: "String with control  chars",
		},
		{
			name:     "StringWithWhitespace",
			input:    "  Whitespace  ",
			expected: "Whitespace",
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: "",
		},
		{
			name:     "OnlyWhitespace",
			input:    "   \t\n  ",
			expected: "",
		},
		{
			name:     "TabAndNewlinesPreserved",
			input:    "Line 1\nLine 2\tTabbed",
			expected: "Line 1\nLine 2\tTabbed",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := SanitizeString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *HTTPUtilTestSuite) TestSanitizeStringMap() {
	testCases := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "EmptyMap",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "MapWithNormalStrings",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "MapWithStringsNeedingSanitizing",
			input: map[string]string{
				"key1": "  value with spaces  ",
				"key2": "<script>alert('XSS')</script>",
				"key3": "Control\x00Char",
			},
			expected: map[string]string{
				"key1": "value with spaces",
				"key2": "&lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt;",
				"key3": "ControlChar",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := SanitizeStringMap(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *HTTPUtilTestSuite) TestExtractBearerToken() {
	testCases := []struct {
		name        string
		authHeader  string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "ValidBearerToken",
			authHeader:  "Bearer token123",
			expected:    "token123",
			expectError: false,
		},
		{
			name:        "ValidBearerTokenWithSpaces",
			authHeader:  "Bearer  token123  ",
			expected:    "token123",
			expectError: false,
		},
		{
			name:        "CaseInsensitiveBearer",
			authHeader:  "bearer token123",
			expected:    "token123",
			expectError: false,
		},
		{
			name:        "UpperCaseBearer",
			authHeader:  "BEARER token123",
			expected:    "token123",
			expectError: false,
		},
		{
			name:        "MixedCaseBearer",
			authHeader:  "BeArEr token123",
			expected:    "token123",
			expectError: false,
		},
		{
			name:        "EmptyHeader",
			authHeader:  "",
			expected:    "",
			expectError: true,
			errorMsg:    "missing Authorization header",
		},
		{
			name:        "MissingBearer",
			authHeader:  "token123",
			expected:    "",
			expectError: true,
			errorMsg:    "invalid Authorization header format. Expected: Bearer <token>",
		},
		{
			name:        "InvalidFormat",
			authHeader:  "Basic token123",
			expected:    "",
			expectError: true,
			errorMsg:    "invalid Authorization header format. Expected: Bearer <token>",
		},
		{
			name:        "MissingToken",
			authHeader:  "Bearer ",
			expected:    "",
			expectError: true,
			errorMsg:    "missing access token",
		},
		{
			name:        "OnlyBearer",
			authHeader:  "Bearer",
			expected:    "",
			expectError: true,
			errorMsg:    "invalid Authorization header format. Expected: Bearer <token>",
		},
		{
			name:        "TokenWithSpaces",
			authHeader:  "Bearer token with spaces",
			expected:    "token with spaces",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := ExtractBearerToken(tc.authHeader)

			if tc.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func (suite *HTTPUtilTestSuite) TestWriteSuccessResponse() {
	testCases := []struct {
		name       string
		statusCode int
		data       interface{}
	}{
		{
			name:       "SuccessWithSimpleData",
			statusCode: http.StatusOK,
			data: map[string]string{
				"message": "success",
				"status":  "ok",
			},
		},
		{
			name:       "SuccessWithStructData",
			statusCode: http.StatusCreated,
			data: testStruct{
				Name:  "test-object",
				Value: 42,
			},
		},
		{
			name:       "SuccessWithArrayData",
			statusCode: http.StatusOK,
			data: []string{
				"item1",
				"item2",
				"item3",
			},
		},
		{
			name:       "SuccessWithNilData",
			statusCode: http.StatusNoContent,
			data:       nil,
		},
		{
			name:       "SuccessWithEmptyMap",
			statusCode: http.StatusOK,
			data:       map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteSuccessResponse(w, tc.statusCode, tc.data)

			// Verify status code
			assert.Equal(t, tc.statusCode, w.Code)

			// Verify Content-Type header (except for 204 No Content)
			if tc.statusCode != http.StatusNoContent {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			}

			// Verify response body content
			if tc.data != nil {
				var response interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				// Verify the actual content matches the input data
				switch v := tc.data.(type) {
				case map[string]string:
					responseMap, ok := response.(map[string]interface{})
					assert.True(t, ok, "Response should be a map")
					for key, value := range v {
						assert.Equal(t, value, responseMap[key])
					}
				case testStruct:
					responseMap, ok := response.(map[string]interface{})
					assert.True(t, ok, "Response should be a map")
					assert.Equal(t, v.Name, responseMap["name"])
					assert.Equal(t, float64(v.Value), responseMap["value"]) // JSON numbers are float64
				case []string:
					responseArray, ok := response.([]interface{})
					assert.True(t, ok, "Response should be an array")
					assert.Equal(t, len(v), len(responseArray))
					for i, item := range v {
						assert.Equal(t, item, responseArray[i])
					}
				case map[string]interface{}:
					responseMap, ok := response.(map[string]interface{})
					assert.True(t, ok, "Response should be a map")
					assert.Equal(t, len(v), len(responseMap))
				}
			}
		})
	}
}

func (suite *HTTPUtilTestSuite) TestWriteSuccessResponse_EncodingError() {
	suite.T().Run("UnserializableData", func(t *testing.T) {
		w := httptest.NewRecorder()

		// Channel cannot be JSON encoded, should trigger the encoding error fallback
		WriteSuccessResponse(w, http.StatusOK, make(chan int))

		// Encoding fails before headers are sent, so we get 500
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Response must be JSON, not plain text
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Body must be valid JSON containing the ErrorEncodingError fields
		var resp apierror.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, serviceerror.ErrorEncodingError.Code, resp.Code)
		assert.Equal(t, serviceerror.ErrorEncodingError.Error.Key, resp.Message.Key)
		assert.Equal(t, serviceerror.ErrorEncodingError.ErrorDescription.Key, resp.Description.Key)
	})
}

// failingWriter returns an error on the first Write call, then succeeds.
// Used to simulate json.Encoder failing mid-stream in WriteErrorResponse.
type failingWriter struct {
	*httptest.ResponseRecorder
	writes int
}

func (fw *failingWriter) Write(b []byte) (int, error) {
	fw.writes++
	if fw.writes == 1 {
		return 0, errors.New("simulated write failure")
	}
	return fw.ResponseRecorder.Write(b)
}

func (suite *HTTPUtilTestSuite) TestWriteErrorResponse_EncodingFallback() {
	suite.T().Run("WriterFailure", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &failingWriter{ResponseRecorder: rec}

		errorResp := apierror.ErrorResponse{
			Code:        "test_error",
			Message:     core.I18nMessage{Key: "error.test", DefaultValue: "Test error"},
			Description: core.I18nMessage{Key: "error.test_desc", DefaultValue: "A test error"},
		}
		WriteErrorResponse(w, http.StatusBadRequest, errorResp)

		// The fallback JSON must be valid and carry the encoding error fields
		var resp apierror.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, serviceerror.ErrorEncodingError.Code, resp.Code)
		assert.Equal(t, serviceerror.ErrorEncodingError.Error.Key, resp.Message.Key)
		assert.Equal(t, serviceerror.ErrorEncodingError.ErrorDescription.Key, resp.Description.Key)
	})
}

func (suite *HTTPUtilTestSuite) TestWriteI18nErrorResponse() {
	testCases := []struct {
		name       string
		statusCode int
		errorResp  apierror.ErrorResponse
	}{
		{
			name:       "BadRequestError",
			statusCode: http.StatusBadRequest,
			errorResp: apierror.ErrorResponse{
				Code:    "invalid_request",
				Message: core.I18nMessage{Key: "error.invalid_request", DefaultValue: "Invalid Request"},
				Description: core.I18nMessage{
					Key:          "error.invalid_request_desc",
					DefaultValue: "The request is missing required parameters",
				},
			},
		},
		{
			name:       "UnauthorizedError",
			statusCode: http.StatusUnauthorized,
			errorResp: apierror.ErrorResponse{
				Code:    "unauthorized",
				Message: core.I18nMessage{Key: "error.unauthorized", DefaultValue: "Unauthorized"},
				Description: core.I18nMessage{
					Key:          "error.unauthorized_desc",
					DefaultValue: "Authentication is required",
				},
			},
		},
		{
			name:       "NotFoundError",
			statusCode: http.StatusNotFound,
			errorResp: apierror.ErrorResponse{
				Code:    "not_found",
				Message: core.I18nMessage{Key: "error.not_found", DefaultValue: "Not Found"},
				Description: core.I18nMessage{
					Key:          "error.not_found_desc",
					DefaultValue: "The requested resource was not found",
				},
			},
		},
		{
			name:       "InternalServerError",
			statusCode: http.StatusInternalServerError,
			errorResp: apierror.ErrorResponse{
				Code:        "internal_error",
				Message:     core.I18nMessage{Key: "error.internal", DefaultValue: "Internal Server Error"},
				Description: core.I18nMessage{Key: "error.internal_desc", DefaultValue: "An unexpected error occurred"},
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteErrorResponse(w, tc.statusCode, tc.errorResp)

			// Verify status code
			assert.Equal(t, tc.statusCode, w.Code)

			// Verify Content-Type header
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			// Verify response body
			var response apierror.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tc.errorResp.Code, response.Code)
			assert.Equal(t, tc.errorResp.Message, response.Message)
			assert.Equal(t, tc.errorResp.Description, response.Description)
		})
	}
}

func (suite *HTTPUtilTestSuite) TestDecodeJSONResponse() {
	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	suite.T().Run("ValidJSONResponse", func(t *testing.T) {
		obj := testStruct{Name: "Alice", Age: 30}
		buf := new(bytes.Buffer)
		_ = json.NewEncoder(buf).Encode(obj)
		resp := &http.Response{
			Body:       io.NopCloser(buf),
			StatusCode: http.StatusOK,
		}
		result, err := DecodeJSONResponse[testStruct](resp)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, obj.Name, result.Name)
		assert.Equal(t, obj.Age, result.Age)
	})

	suite.T().Run("InvalidJSONResponse", func(t *testing.T) {
		buf := bytes.NewBufferString("{invalid json}")
		resp := &http.Response{
			Body:       io.NopCloser(buf),
			StatusCode: http.StatusOK,
		}
		result, err := DecodeJSONResponse[testStruct](resp)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	suite.T().Run("EmptyJSONResponse", func(t *testing.T) {
		buf := bytes.NewBufferString("")
		resp := &http.Response{
			Body:       io.NopCloser(buf),
			StatusCode: http.StatusOK,
		}
		result, err := DecodeJSONResponse[testStruct](resp)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	suite.T().Run("NilResponse", func(t *testing.T) {
		result, err := DecodeJSONResponse[testStruct](nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "response or body is nil")
	})

	suite.T().Run("NilBody", func(t *testing.T) {
		resp := &http.Response{Body: nil, StatusCode: http.StatusOK}
		result, err := DecodeJSONResponse[testStruct](resp)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "response or body is nil")
	})
}

func (suite *HTTPUtilTestSuite) TestMatchURIPattern() {
	tests := []struct {
		name      string
		pattern   string
		incoming  string
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "ExactMatchNoWildcard",
			pattern:   "https://example.com/callback",
			incoming:  "https://example.com/callback",
			wantMatch: true,
		},
		{
			name:      "ExactMismatch",
			pattern:   "https://example.com/callback",
			incoming:  "https://example.com/other",
			wantMatch: false,
		},
		{
			name:      "SingleStarMatchesOneSegment",
			pattern:   "https://example.com/callback/*",
			incoming:  "https://example.com/callback/abc",
			wantMatch: true,
		},
		{
			name:      "SingleStarNoMatchTwoSegments",
			pattern:   "https://example.com/callback/*",
			incoming:  "https://example.com/callback/a/b",
			wantMatch: false,
		},
		{
			name:      "SingleStarNoMatchEmptySegment",
			pattern:   "https://example.com/*",
			incoming:  "https://example.com/",
			wantMatch: false,
		},
		{
			name:      "DoubleStarMatchesZeroSegments",
			pattern:   "https://example.com/callback/**",
			incoming:  "https://example.com/callback",
			wantMatch: true,
		},
		{
			name:      "DoubleStarMatchesOneSegment",
			pattern:   "https://example.com/callback/**",
			incoming:  "https://example.com/callback/a",
			wantMatch: true,
		},
		{
			name:      "DoubleStarMatchesMultipleSegments",
			pattern:   "https://example.com/callback/**",
			incoming:  "https://example.com/callback/a/b/c",
			wantMatch: true,
		},
		{
			name:      "DoubleStarMidPathZeroSegments",
			pattern:   "https://example.com/a/**/b",
			incoming:  "https://example.com/a/b",
			wantMatch: true,
		},
		{
			name:      "DoubleStarMidPathMultipleSegments",
			pattern:   "https://example.com/a/**/b",
			incoming:  "https://example.com/a/x/y/b",
			wantMatch: true,
		},
		{
			name:      "DoubleStarMatchesDeepPath",
			pattern:   "https://example.com/a/**/b",
			incoming:  "https://example.com/a/" + strings.Repeat("x/", 28) + "b",
			wantMatch: true,
		},
		{
			name:      "DoubleStarMatchesVeryDeepPath",
			pattern:   "https://example.com/a/**/b",
			incoming:  "https://example.com/a/" + strings.Repeat("x/", 29) + "b",
			wantMatch: true,
		},
		{
			name:      "SchemeMismatch",
			pattern:   "https://example.com/callback",
			incoming:  "http://example.com/callback",
			wantMatch: false,
		},
		{
			name:      "HostMismatch",
			pattern:   "https://example.com/callback",
			incoming:  "https://other.com/callback",
			wantMatch: false,
		},
		{
			name:      "QueryMatchesExactly",
			pattern:   "https://example.com/callback?foo=bar",
			incoming:  "https://example.com/callback?foo=bar",
			wantMatch: true,
		},
		{
			name:      "QueryValueMismatch",
			pattern:   "https://example.com/callback?foo=bar",
			incoming:  "https://example.com/callback?foo=baz",
			wantMatch: false,
		},
		{
			name:      "QueryPresentOnPatternOnly",
			pattern:   "https://example.com/callback?foo=bar",
			incoming:  "https://example.com/callback",
			wantMatch: false,
		},
		{
			name:      "IncomingWithFragment",
			pattern:   "https://example.com/callback",
			incoming:  "https://example.com/callback#frag",
			wantMatch: false,
		},
		{
			name:      "PatternWithFragment",
			pattern:   "https://example.com/callback#frag",
			incoming:  "https://example.com/callback",
			wantMatch: false,
		},
		{
			name:      "DeeplinkExactMatch",
			pattern:   "myapp://callback",
			incoming:  "myapp://callback",
			wantMatch: true,
		},
		{
			name:      "DeeplinkSingleStarMatch",
			pattern:   "myapp://callback/*",
			incoming:  "myapp://callback/session",
			wantMatch: true,
		},
		{
			name:      "DeeplinkSingleStarNoMatchMultiSegment",
			pattern:   "myapp://callback/*",
			incoming:  "myapp://callback/a/b",
			wantMatch: false,
		},
		{
			name:     "MalformedPattern",
			pattern:  "://bad",
			incoming: "https://example.com/callback",
			wantErr:  true,
		},
		{
			name:     "MalformedIncoming",
			pattern:  "https://example.com/callback",
			incoming: "://bad",
			wantErr:  true,
		},
		{
			name:      "PathTraversalDotDotInIncoming",
			pattern:   "https://example.com/app/**",
			incoming:  "https://example.com/app/../admin",
			wantMatch: false,
		},
		{
			name:      "PathTraversalPercentEncodedDotDotInIncoming",
			pattern:   "https://example.com/app/*",
			incoming:  "https://example.com/app/%2e%2e/admin",
			wantMatch: false,
		},
		// Host wildcard cases: * matches one or more alphanumeric chars within a single label.
		{
			name:      "HostWildcardLabelInternal",
			pattern:   "https://tenant-app-*-*.gateway.example.com",
			incoming:  "https://tenant-app-019dfc78-f19ab4f2.gateway.example.com",
			wantMatch: true,
		},
		{
			name:      "HostWildcardCaseInsensitive",
			pattern:   "https://foo-*-bar.example.com",
			incoming:  "https://FOO-AbCd-Bar.EXAMPLE.com",
			wantMatch: true,
		},
		{
			name:      "HostWildcardDoesNotCrossDot",
			pattern:   "https://foo-*-bar.example.com",
			incoming:  "https://foo-x.y-bar.example.com",
			wantMatch: false,
		},
		{
			name:      "HostWildcardDoesNotMatchHyphenInDynamic",
			pattern:   "https://foo-*-bar.example.com",
			incoming:  "https://foo-a-b-bar.example.com",
			wantMatch: false,
		},
		{
			name:      "HostWildcardLabelCountMismatch",
			pattern:   "https://*-app.example.com",
			incoming:  "https://x-app.dev.example.com",
			wantMatch: false,
		},
		{
			name:      "HostWildcardSingleStarRequiresAtLeastOneChar",
			pattern:   "https://prefix-*.example.com",
			incoming:  "https://prefix-.example.com",
			wantMatch: false,
		},
		{
			name:      "HostWildcardWithPath",
			pattern:   "https://app-*.example.com/cb/*",
			incoming:  "https://app-prod.example.com/cb/v1",
			wantMatch: true,
		},
		{
			name:      "HostWildcardAdjacentLiteralBacktrack",
			pattern:   "https://*foo.example.com",
			incoming:  "https://abcfoo.example.com",
			wantMatch: true,
		},
		{
			name:      "HostWildcardAdjacentLiteralNoMatch",
			pattern:   "https://*foo.example.com",
			incoming:  "https://abcbar.example.com",
			wantMatch: false,
		},
		{
			name:      "HostNoWildcardFastPath",
			pattern:   "https://example.com/cb",
			incoming:  "https://EXAMPLE.com/cb",
			wantMatch: true,
		},
		{
			name:      "HostWildcardWithMatchingPort",
			pattern:   "https://app-*.example.com:8443/cb",
			incoming:  "https://app-prod.example.com:8443/cb",
			wantMatch: true,
		},
		{
			name:      "HostWildcardWithMismatchedPort",
			pattern:   "https://app-*.example.com:8443/cb",
			incoming:  "https://app-prod.example.com:8080/cb",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			matched, err := MatchURIPattern(tt.pattern, tt.incoming)
			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, matched)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantMatch, matched)
			}
		})
	}
}
