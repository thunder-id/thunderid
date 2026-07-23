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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type localFieldError interface {
	Tag() string
	Field() string
	Param() string
}

// GetCustomErrorMessage translates a structural validation tag failure into a precise user message.
func GetCustomErrorMessage(fe localFieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("The field '%s' is missing but is strictly required.", fe.Field())
	case "max":
		return fmt.Sprintf("The field '%s' exceeds its maximum allowed size of %s characters.", fe.Field(), fe.Param())
	case "min":
		return fmt.Sprintf("The field '%s' must be at least %s characters long.", fe.Field(), fe.Param())
	case "oneof":
		return fmt.Sprintf("The value provided for '%s' is invalid. It must be one of: [%s].", fe.Field(), fe.Param())
	case "json":
		return fmt.Sprintf("The field '%s' contains malformed or unparseable JSON formatting.", fe.Field())
	case "url":
		return fmt.Sprintf("The field '%s' must be a valid, well-formed URL.", fe.Field())
	default:
		return fmt.Sprintf("The field '%s' failed validation validation check (%s).", fe.Field(), fe.Tag())
	}
}

// WriteStructuredErrorResponse sends back a dictionary map of localized field errors to the client.
func WriteStructuredErrorResponse(w http.ResponseWriter, statusCode int, message string, errors map[string]string) {
	w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeJSON)
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"code":        "INVALID_INPUT_METADATA",
		"message":     message,
		"description": "One or more inbound fields failed structural edge-boundary rules.",
		"errors":      errors,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// Validatable defines the interface for types that support self-validation rules.
type Validatable interface {
	Validate() map[string]string
}

// ValidationError is a lightweight wrapper for native field errors
type ValidationError struct {
	Errors map[string]string
}

func (e *ValidationError) Error() string { return "Validation Failed" }

// DecodeJSONBody decodes JSON from the request body into any struct type T.
func DecodeJSONBody[T any](r *http.Request) (*T, error) {
	var data T
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return nil, errors.New("failed to decode JSON: " + err.Error())
	}
	if fieldErrors := validateStructNatively(data); fieldErrors != nil {
		return nil, &ValidationError{Errors: fieldErrors}
	}
	return &data, nil
}

func validateStructNatively(s interface{}) map[string]string {
	fieldErrors := make(map[string]string)
	val := reflect.ValueOf(s)

	// Handle pointers automatically
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Look for our company's approved native constraint tag
		tagValue := fieldType.Tag.Get("native")
		if tagValue == "" {
			continue
		}

		jsonName := fieldType.Tag.Get("json")
		if jsonName == "" || strings.Contains(jsonName, ",") {
			jsonName = fieldType.Name
		}
		jsonName = strings.Split(jsonName, ",")[0]

		// Parse individual rules split by commas (e.g., "required,min=3")
		rules := strings.Split(tagValue, ",")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)

			// Rule 1: Required
			if rule == "required" {
				if isZeroValue(field) {
					fieldErrors[jsonName] = "The field '" + jsonName + "' is missing but is strictly required."
					break
				}
			}

			// Rule 2: Minimum Length Bounds
			if strings.HasPrefix(rule, "min=") {
				minStr := strings.TrimPrefix(rule, "min=")
				minVal, _ := strconv.Atoi(minStr)
				if field.Kind() == reflect.String && len(field.String()) < minVal {
					fieldErrors[jsonName] = "The field '" + jsonName +
						"' must be at least " + minStr + " characters long."
					break
				}
			}

			// Rule 3: Maximum Length Bounds
			if strings.HasPrefix(rule, "max=") {
				maxStr := strings.TrimPrefix(rule, "max=")
				maxVal, _ := strconv.Atoi(maxStr)
				if field.Kind() == reflect.String && len(field.String()) > maxVal {
					fieldErrors[jsonName] = "The field '" + jsonName +
						"' exceeds its maximum allowed size of " + maxStr + " characters."
					break
				}
			}
		}
	}

	if len(fieldErrors) > 0 {
		return fieldErrors
	}
	return nil
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return strings.TrimSpace(v.String()) == ""
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}

// WriteJSONError writes a JSON error response with the given details.
func WriteJSONError(ctx context.Context, w http.ResponseWriter, code, desc string, statusCode int,
	respHeaders []map[string]string) {
	logger := log.GetLogger()
	// Log 5xx as errors (server faults) and 4xx as debug (client errors).
	if statusCode >= http.StatusInternalServerError {
		logger.Error(ctx, "Error in HTTP response",
			log.String("error", code), log.String("description", desc))
	} else {
		logger.Debug(ctx, "Error in HTTP response",
			log.String("error", code), log.String("description", desc))
	}

	// Set the response headers.
	for _, header := range respHeaders {
		for key, value := range header {
			w.Header().Set(key, value)
		}
	}
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": desc,
	})
	if err != nil {
		logger.Error(ctx, "Failed to write JSON error response", log.Error(err))
		return
	}
}

// ParseURL parses the given URL string and returns a URL object.
func ParseURL(urlStr string) (*url.URL, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return parsedURL, nil
}

// MatchURIPattern reports whether incoming matches pattern.
// In the path component, * matches exactly one segment and ** matches zero or more segments.
// In the host component, * matches one or more alphanumeric characters within a single
// DNS label and does not cross label boundaries. Scheme and host comparison is
// case-insensitive; query must match exactly. Paths are cleaned (resolving . and ..
// segments) before matching to prevent path traversal. Returns (false, error) for malformed
// inputs, (false, nil) for no match, (true, nil) for a match.
func MatchURIPattern(pattern, incoming string) (bool, error) {
	patternURL, err := url.Parse(pattern)
	if err != nil || patternURL.Scheme == "" || patternURL.Host == "" {
		return false, errors.New("invalid pattern URI: missing scheme or host")
	}
	incomingURL, err := url.Parse(incoming)
	if err != nil || incomingURL.Scheme == "" || incomingURL.Host == "" {
		return false, errors.New("invalid incoming URI: missing scheme or host")
	}

	if !strings.EqualFold(patternURL.Scheme, incomingURL.Scheme) {
		return false, nil
	}
	if !matchHostPattern(patternURL.Host, incomingURL.Host) {
		return false, nil
	}
	if patternURL.RawQuery != incomingURL.RawQuery {
		return false, nil
	}
	if patternURL.Fragment != "" || incomingURL.Fragment != "" {
		return false, nil
	}
	return matchPathPattern(path.Clean(patternURL.Path), path.Clean(incomingURL.Path)), nil
}

// matchHostPattern matches incoming host against pattern host. * in the pattern matches
// one or more alphanumeric characters within a single DNS label. Comparison is
// case-insensitive. When the pattern contains no *, this is equivalent to strings.EqualFold.
func matchHostPattern(pattern, incoming string) bool {
	if !strings.ContainsRune(pattern, '*') {
		return strings.EqualFold(pattern, incoming)
	}
	pattern = strings.ToLower(pattern)
	incoming = strings.ToLower(incoming)
	pLabels := strings.Split(pattern, ".")
	iLabels := strings.Split(incoming, ".")
	if len(pLabels) != len(iLabels) {
		return false
	}
	for k := range pLabels {
		if !matchHostLabel(pLabels[k], iLabels[k]) {
			return false
		}
	}
	return true
}

// matchHostLabel matches a single host label. * in the pattern matches one or more
// alphanumeric chars (a-z, 0-9). Both inputs must already be lowercased.
func matchHostLabel(pat, inc string) bool {
	return matchHostLabelImpl(pat, inc, 0, 0)
}

// matchHostLabelImpl is the recursive backtracking matcher for matchHostLabel.
// pi and ii are the current positions in pat and inc respectively. * is greedy with
// backtracking so adjacent literals like *foo can match correctly.
func matchHostLabelImpl(pat, inc string, pi, ii int) bool {
	for pi < len(pat) {
		if pat[pi] == '*' {
			j := ii
			for j < len(inc) && isHostAlphaNum(inc[j]) {
				j++
			}
			// * must consume at least one character; try the longest match first then backtrack.
			for k := j; k > ii; k-- {
				if matchHostLabelImpl(pat, inc, pi+1, k) {
					return true
				}
			}
			return false
		}
		if ii >= len(inc) || pat[pi] != inc[ii] {
			return false
		}
		pi++
		ii++
	}
	return ii == len(inc)
}

// isHostAlphaNum reports whether the byte is a lowercase letter or a digit.
// The host matcher lowercases its inputs before comparing, so this is the full
// character class consumed by * within a host label.
func isHostAlphaNum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z')
}

// matchPathPattern reports whether incomingPath matches patternPath.
// Wildcards * (one segment) and ** (zero or more segments) are supported in patternPath.
func matchPathPattern(patternPath, incomingPath string) bool {
	patSegs := strings.Split(patternPath, "/")
	incSegs := strings.Split(incomingPath, "/")
	memo := make(map[[2]int]bool)
	return matchSegs(patSegs, incSegs, 0, 0, memo)
}

// matchSegs is a memoized entry point for the recursive segment matching.
func matchSegs(patSegs, incSegs []string, i, j int, memo map[[2]int]bool) bool {
	key := [2]int{i, j}
	if cached, ok := memo[key]; ok {
		return cached
	}
	result := matchSegsImpl(patSegs, incSegs, i, j, memo)
	memo[key] = result
	return result
}

// matchSegsImpl performs the actual recursive segment matching logic.
func matchSegsImpl(patSegs, incSegs []string, i, j int, memo map[[2]int]bool) bool {
	// Both exhausted.
	if i == len(patSegs) && j == len(incSegs) {
		return true
	}
	// Pattern exhausted but incoming still has segments.
	if i == len(patSegs) {
		return false
	}
	// Incoming exhausted but pattern still has segments:
	// only true if all remaining pattern segments are "**".
	if j == len(incSegs) {
		for k := i; k < len(patSegs); k++ {
			if patSegs[k] != "**" {
				return false
			}
		}
		return true
	}

	pSeg := patSegs[i]

	if pSeg == "**" {
		// Try consuming zero incoming segments (advance pattern only).
		if matchSegs(patSegs, incSegs, i+1, j, memo) {
			return true
		}
		// Try consuming one incoming segment (keep pattern position).
		return matchSegs(patSegs, incSegs, i, j+1, memo)
	}

	if pSeg == "*" {
		// Must match exactly one non-empty segment.
		if incSegs[j] == "" {
			return false
		}
		return matchSegs(patSegs, incSegs, i+1, j+1, memo)
	}

	// Literal segment: must match exactly.
	if pSeg != incSegs[j] {
		return false
	}
	return matchSegs(patSegs, incSegs, i+1, j+1, memo)
}

// IsValidURI checks if the provided URI is valid.
func IsValidURI(uri string) bool {
	if uri == "" {
		return false
	}
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return true
}

// IsValidLogoURI checks if the provided URI is valid for use as a logo URL.
// It enforces a scheme allowlist: http/https require a non-empty host, data/blob/emoji/avatar
// schemes are always accepted, and relative paths (no scheme, non-empty path) are accepted.
// All other schemes (e.g. javascript, file) are rejected.
func IsValidLogoURI(uri string) bool {
	if uri == "" {
		return false
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "http", "https":
		return parsed.Host != ""
	case "data", "blob", "emoji", "avatar":
		return true
	case "":
		// Accept relative paths (no scheme, but path must start with /)
		return strings.HasPrefix(parsed.Path, "/")
	default:
		return false
	}
}

// GetURIWithQueryParams constructs a URI with the given query parameters.
func GetURIWithQueryParams(uri string, queryParams map[string]string) (string, error) {
	// Parse the URI.
	parsedURL, err := ParseURL(uri)
	if err != nil {
		return "", errors.New("failed to parse the return URI: " + err.Error())
	}

	// Return the URI if there are no query parameters.
	if len(queryParams) == 0 {
		return parsedURL.String(), nil
	}

	// Add the query parameters to the URI.
	query := parsedURL.Query()
	for key, value := range queryParams {
		query.Add(key, value)
	}
	parsedURL.RawQuery = query.Encode()

	// Return the constructed URI.
	return parsedURL.String(), nil
}

// DecodeJSONResponse decodes JSON from the response body into any struct type T.
// TODO: Unify DecodeJSONBody and DecodeJSONResponse into a single method.
func DecodeJSONResponse[T any](resp *http.Response) (*T, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("failed to decode JSON response: response or body is nil")
	}
	var data T
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, errors.New("failed to decode JSON response: " + err.Error())
	}
	return &data, nil
}

// SanitizeString trims whitespace, removes control characters, and escapes HTML.
func SanitizeString(input string) string {
	if input == "" {
		return input
	}

	// Trim leading and trailing whitespace
	trimmed := strings.TrimSpace(input)

	// Remove non-printable/control characters (except newline and tab)
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, trimmed)

	// Escape HTML to prevent XSS
	safe := html.EscapeString(cleaned)

	return safe
}

// SanitizeStringMap sanitizes a map of strings.
// This function trim whitespace, removes control characters, and escapes HTML in each map entry.
func SanitizeStringMap(inputs map[string]string) map[string]string {
	if len(inputs) == 0 {
		return inputs
	}

	sanitized := make(map[string]string, len(inputs))
	for key, value := range inputs {
		sanitized[key] = SanitizeString(value)
	}
	return sanitized
}

// sanitizeRaw trims whitespace and removes control characters but does NOT HTML-escape.
// Use this for values that must remain structurally intact (e.g. JSON, URIs, JWTs)
// and are not rendered in an HTML context.
func sanitizeRaw(input string) string {
	if input == "" {
		return input
	}

	trimmed := strings.TrimSpace(input)
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, trimmed)
}

// SanitizeRawMultiValueStringMap sanitizes a map[string][]string by trimming whitespace and
// removing control characters from values but does NOT HTML-escape and does NOT modify keys.
// Keys are preserved verbatim to prevent normalization collisions (e.g. " X " and "X" trimming
// to the same key). Use this for structured HTTP values such as JSON, URIs, and JWTs.
func SanitizeRawMultiValueStringMap(inputs map[string][]string) map[string][]string {
	if len(inputs) == 0 {
		return inputs
	}

	sanitized := make(map[string][]string, len(inputs))
	for key, values := range inputs {
		sanitizedValues := make([]string, len(values))
		for i, v := range values {
			sanitizedValues[i] = sanitizeRaw(v)
		}
		sanitized[key] = sanitizedValues
	}

	return sanitized
}

// sensitiveHeaders is the deny-list of header names (lowercase) that must not be forwarded
// beyond the HTTP boundary into provider metadata or initiator requests.
var sensitiveHeaders = map[string]bool{
	strings.ToLower(constants.AuthorizationHeaderName):      true,
	strings.ToLower(constants.CookieHeaderName):             true,
	strings.ToLower(constants.SetCookieHeaderName):          true,
	strings.ToLower(constants.ProxyAuthorizationHeaderName): true,
}

// FilterSensitiveHeaders returns a copy of the header map with credential-bearing headers removed.
func FilterSensitiveHeaders(h map[string][]string) map[string][]string {
	if len(h) == 0 {
		return h
	}

	filtered := make(map[string][]string, len(h))
	for name, values := range h {
		if !sensitiveHeaders[strings.ToLower(name)] {
			filtered[name] = values
		}
	}

	return filtered
}

// IsBearerAuth checks if the Authorization header uses the Bearer scheme (case-insensitive).
func IsBearerAuth(authHeader string) bool {
	parts := strings.SplitN(authHeader, " ", 2)
	return len(parts) >= 1 && strings.EqualFold(parts[0], constants.TokenTypeBearer)
}

// ExtractBearerToken extracts the Bearer token from the Authorization header value.
// It validates that the header is not empty, starts with "Bearer" (case-insensitive),
// and contains a non-empty token. Returns the token and an error if validation fails.
func ExtractBearerToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("missing Authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], constants.TokenTypeBearer) {
		return "", errors.New("invalid Authorization header format. Expected: Bearer <token>")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("missing access token")
	}

	return token, nil
}

// WriteSuccessResponse writes a JSON success response with the given status code and data.
func WriteSuccessResponse(ctx context.Context, w http.ResponseWriter, statusCode int, data interface{}) {
	logger := log.GetLogger()

	if statusCode == http.StatusNoContent {
		w.WriteHeader(statusCode)
		return
	}

	// Encode to buffer first to ensure encoding succeeds before sending headers
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		logger.Error(ctx, "Failed to encode response", log.Error(err))
		errResp := apierror.ErrorResponse{
			Code:        tidcommon.ErrorEncodingError.Code,
			Message:     tidcommon.ErrorEncodingError.Error,
			Description: tidcommon.ErrorEncodingError.ErrorDescription,
		}
		b, _ := json.Marshal(errResp)
		w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(b)
		return
	}

	// Encoding succeeded, now safe to send headers and write response
	w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeJSON)
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf.Bytes())
}

// WriteErrorResponse writes a JSON i18n error response with the given status code and error details.
func WriteErrorResponse(ctx context.Context, w http.ResponseWriter, statusCode int, errorResp apierror.ErrorResponse) {
	logger := log.GetLogger()
	w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeJSON)
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		logger.Error(ctx, "Failed to encode i18n error response", log.Error(err))
		errResp := apierror.ErrorResponse{
			Code:        tidcommon.ErrorEncodingError.Code,
			Message:     tidcommon.ErrorEncodingError.Error,
			Description: tidcommon.ErrorEncodingError.ErrorDescription,
		}
		b, _ := json.Marshal(errResp)
		w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(b)
	}
}
