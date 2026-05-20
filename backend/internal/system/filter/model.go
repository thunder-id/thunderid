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

// Package filter provides common types and parsing utilities for API filter expressions.
package filter

// Operator represents a comparison operator in a filter expression.
type Operator string

const (
	// OperatorEq represents the equality operator.
	OperatorEq Operator = "eq"
	// OperatorGt represents the greater-than operator.
	OperatorGt Operator = "gt"
	// OperatorLt represents the less-than operator.
	OperatorLt Operator = "lt"
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
