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

package executor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	httpRequestLoggerComponentName = "HTTPRequestExecutor"

	// default http method
	defaultHTTPMethod = "GET"
	// Default timeout for HTTP requests in seconds
	defaultHTTPTimeout = 10
	// Maximum allowed timeout for HTTP requests in seconds
	maxHTTPRequestTimeout = 20
	// Maximum allowed retry count
	maxHTTPRequestRetryCount = 5
	// Maximum allowed retry delay in milliseconds
	maxHTTPRequestRetryDelay = 5000
)

// validHTTPMethods defines the supported HTTP methods.
var validHTTPMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

// httpRequestConfig represents the HTTP request configuration from node properties.
type httpRequestConfig struct {
	URL             string                 `json:"url"`
	Method          string                 `json:"method"`
	Headers         map[string]string      `json:"headers"`
	Body            map[string]interface{} `json:"body"`
	Timeout         int                    `json:"timeout"`
	ResponseMapping map[string]string      `json:"responseMapping"`
	ErrorHandling   *errorHandlingConfig   `json:"errorHandling"`
}

// errorHandlingConfig represents error handling configuration for HTTP requests.
type errorHandlingConfig struct {
	FailOnError bool `json:"failOnError"`
	RetryCount  int  `json:"retryCount"`
	RetryDelay  int  `json:"retryDelay"`
}

// httpRequestExecutor implements the ExecutorInterface for making HTTP requests to external endpoints.
type httpRequestExecutor struct {
	core.ExecutorInterface
	ouService ou.OrganizationUnitServiceInterface
	logger    *log.Logger
}

var _ core.ExecutorInterface = (*httpRequestExecutor)(nil)

// newHTTPRequestExecutor creates a new instance of HTTPRequestExecutor.
func newHTTPRequestExecutor(
	flowFactory core.FlowFactoryInterface,
	ouService ou.OrganizationUnitServiceInterface,
) *httpRequestExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, httpRequestLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameHTTPRequest))

	base := flowFactory.CreateExecutor(ExecutorNameHTTPRequest, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{})

	return &httpRequestExecutor{
		ExecutorInterface: base,
		ouService:         ouService,
		logger:            logger,
	}
}

// Execute executes the HTTP request logic.
func (h *httpRequestExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := h.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing HTTP request executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	config, err := h.parseAndValidateConfig(ctx.NodeProperties)
	if err != nil {
		logger.Error("Failed to parse/validate HTTP request configuration", log.Error(err))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Configuration error: " + err.Error()
		return execResp, nil
	}

	h.enrichOURuntimeData(ctx, config)
	h.resolvePlaceholders(ctx, config)

	response, err := h.executeRequestWithRetry(ctx, config)
	if err != nil {
		logger.Error("Failed to execute HTTP request", log.Error(err))
		return h.handleRequestError(execResp, config, err.Error(), logger), nil
	}

	if err := h.processResponse(ctx, config, response, execResp); err != nil {
		logger.Error("Failed to process response", log.Error(err))
		return h.handleRequestError(execResp, config, err.Error(), logger), nil
	}

	execResp.Status = common.ExecComplete
	logger.Debug("HTTP request executor execution completed", log.String("status", string(execResp.Status)))

	return execResp, nil
}

// parseAndValidateConfig parses the HTTP request configuration from node properties,
// validates it, and applies defaults and limits.
func (h *httpRequestExecutor) parseAndValidateConfig(properties map[string]interface{}) (
	*httpRequestConfig, error) {
	if len(properties) == 0 {
		return nil, errors.New("node properties are empty")
	}

	// Convert properties map to JSON
	jsonBytes, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal properties: %w", err)
	}

	// Parse into intermediate map to handle nested structures
	var propsMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &propsMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
	}

	config := &httpRequestConfig{
		Headers:         make(map[string]string),
		Body:            make(map[string]interface{}),
		ResponseMapping: make(map[string]string),
	}

	// Parse URL
	if url, ok := propsMap["url"].(string); ok {
		config.URL = url
	}
	if config.URL == "" {
		return nil, errors.New("url is required")
	}

	// Parse method
	if method, ok := propsMap["method"].(string); ok {
		config.Method = strings.ToUpper(method)
	}
	if config.Method == "" {
		config.Method = defaultHTTPMethod
	}
	if !slices.Contains(validHTTPMethods, config.Method) {
		return nil, fmt.Errorf("invalid HTTP method: %s", config.Method)
	}

	h.parseTimeout(config, propsMap)
	h.parseHeaderAndBody(config, propsMap)
	h.parseResponseMapping(config, propsMap)
	h.parseErrorHandling(config, propsMap)

	return config, nil
}

// parseTimeout parses timeout from node properties with limits.
func (h *httpRequestExecutor) parseTimeout(config *httpRequestConfig, propsMap map[string]interface{}) {
	if timeout, ok := propsMap["timeout"]; ok {
		switch v := timeout.(type) {
		case string:
			if timeoutInt, err := strconv.Atoi(v); err == nil {
				config.Timeout = timeoutInt
			}
		case float64:
			config.Timeout = int(v)
		}
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultHTTPTimeout
	}
	if config.Timeout > maxHTTPRequestTimeout {
		config.Timeout = maxHTTPRequestTimeout
	}
}

// parseHeaderAndBody parses headers and body from node properties.
func (h *httpRequestExecutor) parseHeaderAndBody(config *httpRequestConfig, propsMap map[string]interface{}) {
	if headersStr, ok := propsMap["headers"].(string); ok {
		if err := json.Unmarshal([]byte(headersStr), &config.Headers); err != nil {
			h.logger.Warn("Failed to parse headers JSON string, ignoring headers", log.Error(err))
		}
	} else if headersMap, ok := propsMap["headers"].(map[string]interface{}); ok {
		for k, v := range headersMap {
			if strVal, ok := v.(string); ok {
				config.Headers[k] = strVal
			}
		}
	}

	if bodyStr, ok := propsMap["body"].(string); ok {
		if err := json.Unmarshal([]byte(bodyStr), &config.Body); err != nil {
			h.logger.Warn("Failed to parse body JSON string, ignoring body", log.Error(err))
		}
	} else if bodyMap, ok := propsMap["body"].(map[string]interface{}); ok {
		config.Body = bodyMap
	}
}

// parseResponseMapping parses response mapping from node properties.
func (h *httpRequestExecutor) parseResponseMapping(config *httpRequestConfig, propsMap map[string]interface{}) {
	if mappingStr, ok := propsMap["responseMapping"].(string); ok {
		if err := json.Unmarshal([]byte(mappingStr), &config.ResponseMapping); err != nil {
			h.logger.Warn("Failed to parse response mapping JSON string, ignoring response mapping",
				log.Error(err))
		}
	} else if mappingMap, ok := propsMap["responseMapping"].(map[string]interface{}); ok {
		for k, v := range mappingMap {
			if strVal, ok := v.(string); ok {
				config.ResponseMapping[k] = strVal
			}
		}
	}
}

// parseErrorHandling parses error handling configuration with limits.
func (h *httpRequestExecutor) parseErrorHandling(config *httpRequestConfig, propsMap map[string]interface{}) {
	if errorHandlingStr, ok := propsMap["errorHandling"].(string); ok {
		var eh errorHandlingConfig
		if err := json.Unmarshal([]byte(errorHandlingStr), &eh); err == nil {
			config.ErrorHandling = &eh
		} else {
			h.logger.Warn("Failed to parse error handling JSON string, ignoring error handling", log.Error(err))
		}
	} else if ehMap, ok := propsMap["errorHandling"].(map[string]interface{}); ok {
		eh := &errorHandlingConfig{}
		if failOnError, ok := ehMap["failOnError"].(bool); ok {
			eh.FailOnError = failOnError
		}
		if retryCount, ok := ehMap["retryCount"].(float64); ok {
			eh.RetryCount = int(retryCount)
		}
		if retryDelay, ok := ehMap["retryDelay"].(float64); ok {
			eh.RetryDelay = int(retryDelay)
		}
		config.ErrorHandling = eh
	}

	if config.ErrorHandling != nil {
		if config.ErrorHandling.RetryCount > maxHTTPRequestRetryCount {
			config.ErrorHandling.RetryCount = maxHTTPRequestRetryCount
		}
		if config.ErrorHandling.RetryDelay > maxHTTPRequestRetryDelay {
			config.ErrorHandling.RetryDelay = maxHTTPRequestRetryDelay
		}
	}
}

// enrichOURuntimeData fetches OU details and populates ouHandle and ouName into RuntimeData
// so that placeholders like {{ context.ouHandle }} can be resolved for the current request.
// It only performs the fetch when an OU placeholder is referenced in the config and the OU ID
// is available in the context.
func (h *httpRequestExecutor) enrichOURuntimeData(ctx *core.NodeContext, config *httpRequestConfig) {
	if h.ouService == nil {
		return
	}

	ouPlaceholderPattern := regexp.MustCompile(`{{\s*context\.\s*(ouHandle|ouName|ouDescription)\s*}}`)

	// Build a single searchable string from all resolvable config fields.
	var sb strings.Builder
	sb.WriteString(config.URL)
	for _, v := range config.Headers {
		sb.WriteString(v)
	}
	if bodyJSON, err := json.Marshal(config.Body); err == nil {
		sb.Write(bodyJSON)
	}
	if !ouPlaceholderPattern.MatchString(sb.String()) {
		return
	}

	ouID := ctx.AuthenticatedUser.OUID
	if ouID == "" {
		ouID = ctx.RuntimeData[ouIDKey]
	}
	if ouID == "" {
		return
	}

	logger := h.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	organizationUnit, svcErr := h.ouService.GetOrganizationUnit(ctx.Context, ouID)
	if svcErr != nil {
		logger.Warn("Failed to fetch OU details for placeholder enrichment",
			log.String(ouIDKey, ouID), log.String("error", svcErr.Error.DefaultValue))
		return
	}

	ctx.RuntimeData["ouHandle"] = organizationUnit.Handle
	ctx.RuntimeData["ouName"] = organizationUnit.Name
	ctx.RuntimeData["ouDescription"] = organizationUnit.Description
}

// resolvePlaceholders resolves placeholders in the configuration using context data.
func (h *httpRequestExecutor) resolvePlaceholders(ctx *core.NodeContext, config *httpRequestConfig) {
	config.URL = core.ResolvePlaceholder(ctx, config.URL)

	// Resolve headers
	for key, value := range config.Headers {
		config.Headers[key] = core.ResolvePlaceholder(ctx, value)
	}

	// Resolve body
	config.Body = h.resolveMapPlaceholders(ctx, config.Body).(map[string]interface{})
}

// resolveMapPlaceholders recursively resolves placeholders in a map or slice.
func (h *httpRequestExecutor) resolveMapPlaceholders(ctx *core.NodeContext, data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = h.resolveMapPlaceholders(ctx, value)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, value := range v {
			result[i] = h.resolveMapPlaceholders(ctx, value)
		}
		return result
	case string:
		return core.ResolvePlaceholder(ctx, v)
	default:
		return v
	}
}

// executeRequestWithRetry executes the HTTP request with retry logic.
func (h *httpRequestExecutor) executeRequestWithRetry(ctx *core.NodeContext,
	config *httpRequestConfig) (*http.Response, error) {
	logger := h.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	retryCount := 0
	retryDelay := 0

	if config.ErrorHandling != nil {
		retryCount = config.ErrorHandling.RetryCount
		retryDelay = config.ErrorHandling.RetryDelay
	}

	httpClient := httpservice.NewHTTPClientWithTimeout(
		time.Duration(config.Timeout) * time.Second)

	var lastErr error
	attempts := retryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			logger.Debug("Retrying HTTP request", log.Int("attempt", attempt), log.Int("maxRetries", retryCount))
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}

		response, err := h.executeRequest(ctx, config, httpClient)
		if err == nil {
			return response, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", attempts, lastErr)
}

// executeRequest executes a single HTTP request.
func (h *httpRequestExecutor) executeRequest(ctx *core.NodeContext, config *httpRequestConfig,
	httpClient httpservice.HTTPClientInterface) (*http.Response, error) {
	logger := h.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// Prepare request body
	var bodyReader io.Reader
	if len(config.Body) > 0 {
		bodyBytes, err := json.Marshal(config.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequest(config.Method, config.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Set default Content-Type for requests with body
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	logger.Debug("Sending HTTP request", log.String("method", config.Method),
		log.MaskedString("url", config.URL))

	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}

	return response, nil
}

// processResponse processes the HTTP response and extracts data based on response mapping.
func (h *httpRequestExecutor) processResponse(ctx *core.NodeContext, config *httpRequestConfig,
	response *http.Response, execResp *common.ExecutorResponse) error {
	logger := h.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	defer func() {
		if err := response.Body.Close(); err != nil {
			logger.Error("Failed to close response body", log.Error(err))
		}
	}()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Debug("Received HTTP response", log.Int("statusCode", response.StatusCode),
		log.String("status", response.Status))

	// Check for error status codes
	if response.StatusCode >= 400 {
		return fmt.Errorf("HTTP request failed with status %d: %s", response.StatusCode, string(bodyBytes))
	}

	// Parse response body as JSON
	var parsedBody map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &parsedBody); err != nil {
			parsedBody = map[string]interface{}{
				"raw": string(bodyBytes),
			}
		}
	}

	// Wrap response body and status in a structure
	responseData := map[string]interface{}{
		"response": map[string]interface{}{
			"data":   parsedBody,
			"status": response.StatusCode,
		},
	}

	// Apply response mapping if configured
	if len(config.ResponseMapping) > 0 {
		for targetKey, sourcePath := range config.ResponseMapping {
			value := h.extractValueFromPath(responseData, sourcePath)
			if value != nil {
				execResp.RuntimeData[targetKey] = fmt.Sprintf("%v", value)
			}
		}
	}

	return nil
}

// extractValueFromPath extracts a value from a nested map using a dot-notation path.
func (h *httpRequestExecutor) extractValueFromPath(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		if part == "" {
			continue
		}
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}

		if current == nil {
			return nil
		}
	}

	return current
}

// handleRequestError handles HTTP request errors and sets the appropriate response status based on the
// failOnError configuration.
func (h *httpRequestExecutor) handleRequestError(execResp *common.ExecutorResponse, config *httpRequestConfig,
	errorMessage string, logger *log.Logger) *common.ExecutorResponse {
	failOnError := false
	if config != nil && config.ErrorHandling != nil {
		failOnError = config.ErrorHandling.FailOnError
	}

	if failOnError {
		logger.Debug("Failing execution due to HTTP request error")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = errorMessage
	} else {
		logger.Debug("Continuing execution despite HTTP request error", log.String("error", errorMessage))
		execResp.Status = common.ExecComplete
	}

	return execResp
}
