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

// Package common provides shared ThunderID engine models and service errors.
package common

import (
	"encoding/json"
	"regexp"
)

// i18nParamPattern matches "{{param(name)}}" placeholders with optional whitespace.
// The captured group is the parameter name, which is looked up in I18nMessage.Params.
var i18nParamPattern = regexp.MustCompile(`{{\s*param\(\s*(\w+)\s*\)\s*}}`)

// substituteParams replaces "{{param(name)}}" placeholders in template with the
// corresponding values from params. Unmatched placeholders are left untouched, and the
// template is returned unchanged when there are no params.
func substituteParams(template string, params map[string]string) string {
	if len(params) == 0 {
		return template
	}
	return i18nParamPattern.ReplaceAllStringFunc(template, func(match string) string {
		name := i18nParamPattern.FindStringSubmatch(match)[1]
		if value, ok := params[name]; ok {
			return value
		}
		return match
	})
}

// I18nMessage represents a translatable message with a key and default value.
// DefaultValue may contain "{{param(name)}}" placeholders that are substituted from
// Params, keeping the key a parameterized template for translators.
type I18nMessage struct {
	Key          string            `json:"key"`
	DefaultValue string            `json:"defaultValue"`
	Params       map[string]string `json:"params,omitempty"`
}

// String returns the default value of the message with its params substituted.
// This is useful for logging or when translation is not available.
func (m I18nMessage) String() string {
	return substituteParams(m.DefaultValue, m.Params)
}

// MarshalJSON serializes the message with its params substituted into defaultValue, so
// consumers that only read the fallback still get a ready human-readable string, while
// params remains available for key-based translation.
func (m I18nMessage) MarshalJSON() ([]byte, error) {
	type alias I18nMessage
	return json.Marshal(alias{
		Key:          m.Key,
		DefaultValue: substituteParams(m.DefaultValue, m.Params),
		Params:       m.Params,
	})
}

// IsEmpty returns true if the message has no key set.
func (m I18nMessage) IsEmpty() bool {
	return m.Key == ""
}

// ServiceError defines a service error structure with i18n support.
// This is the new error type that should be used for services being migrated to i18n.
// Translatable fields use core.Message instead of plain strings.
type ServiceError struct {
	Code             string           `json:"code"`
	Type             ServiceErrorType `json:"type"`
	Error            I18nMessage      `json:"error"`
	ErrorDescription I18nMessage      `json:"error_description,omitempty"`
}

// CustomServiceError creates a new service error based on an existing error with a custom description.
// The caller must supply a complete I18nMessage with both Key and DefaultValue so that the
// translation system has a unique key to resolve, not the base error's generic key.
func CustomServiceError(svcError ServiceError, errorDesc I18nMessage) *ServiceError {
	err := &ServiceError{
		Type:             svcError.Type,
		Code:             svcError.Code,
		Error:            svcError.Error,
		ErrorDescription: svcError.ErrorDescription,
	}
	if !errorDesc.IsEmpty() {
		err.ErrorDescription = errorDesc
	}
	return err
}

// WithParams returns a copy of the error whose ErrorDescription carries the given
// substitution params. Use it when the base error's description is a "{{param(name)}}"
// template defined in an error_constants.go file.
func (e ServiceError) WithParams(params map[string]string) *ServiceError {
	err := e
	err.ErrorDescription.Params = params
	return &err
}

// Operator represents a comparison operator in a filter expression.
type Operator string

const (
	// OperatorEq represents the equality operator.
	OperatorEq Operator = "eq"
	// OperatorGt represents the greater-than operator.
	OperatorGt Operator = "gt"
	// OperatorLt represents the less-than operator.
	OperatorLt Operator = "lt"
	// OperatorContains represents the case-insensitive substring (contains) operator.
	OperatorContains Operator = "co"
)

// FilterExpression holds a parsed filter expression from an API request.
// Value is typed as string, int64, float64, or bool depending on the literal.
type FilterExpression struct {
	Attribute string
	Operator  Operator
	Value     interface{}
}

// LogicalOperator is the connector between consecutive filter clauses.
type LogicalOperator string

const (
	// LogicalAnd requires both the preceding and the current clause to be true.
	LogicalAnd LogicalOperator = "AND"
	// LogicalOr requires either the preceding or the current clause to be true.
	LogicalOr LogicalOperator = "OR"
)

// FilterClause pairs a logical connector with a single FilterExpression.
// The Connector field on the first clause in a FilterGroup is always ignored.
type FilterClause struct {
	Connector LogicalOperator
	Expr      FilterExpression
}

// FilterGroup holds one or more clauses evaluated with their logical connectors.
// AND has higher precedence than OR, matching standard SQL behavior.
type FilterGroup struct {
	Clauses []FilterClause
}
