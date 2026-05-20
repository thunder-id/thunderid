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

package filter

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilterExpression(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAttr  string
		wantOp    Operator
		wantValue interface{}
		wantErr   bool
	}{
		{
			name:      "eq with quoted string",
			input:     `name eq "engineering"`,
			wantAttr:  "name",
			wantOp:    OperatorEq,
			wantValue: "engineering",
		},
		{
			name:      "gt with quoted timestamp",
			input:     `createdAt gt "2024-01-01T00:00:00Z"`,
			wantAttr:  "createdAt",
			wantOp:    OperatorGt,
			wantValue: "2024-01-01T00:00:00Z",
		},
		{
			name:      "lt with quoted timestamp",
			input:     `updatedAt lt "2025-12-31T23:59:59Z"`,
			wantAttr:  "updatedAt",
			wantOp:    OperatorLt,
			wantValue: "2025-12-31T23:59:59Z",
		},
		{
			name:      "eq with unquoted integer",
			input:     `count eq 42`,
			wantAttr:  "count",
			wantOp:    OperatorEq,
			wantValue: int64(42),
		},
		{
			name:      "gt with unquoted integer",
			input:     `size gt 100`,
			wantAttr:  "size",
			wantOp:    OperatorGt,
			wantValue: int64(100),
		},
		{
			name:      "lt with unquoted float",
			input:     `score lt 3.14`,
			wantAttr:  "score",
			wantOp:    OperatorLt,
			wantValue: float64(3.14),
		},
		{
			name:      "eq with boolean true",
			input:     `active eq true`,
			wantAttr:  "active",
			wantOp:    OperatorEq,
			wantValue: true,
		},
		{
			name:      "eq with boolean false",
			input:     `enabled eq false`,
			wantAttr:  "enabled",
			wantOp:    OperatorEq,
			wantValue: false,
		},
		{
			name:      "nested attribute with dot notation",
			input:     `address.city eq "Colombo"`,
			wantAttr:  "address.city",
			wantOp:    OperatorEq,
			wantValue: "Colombo",
		},
		{
			name:    "unsupported operator",
			input:   `name gte "foo"`,
			wantErr: true,
		},
		{
			name:    "missing value",
			input:   `name eq`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   `not-valid`,
			wantErr: true,
		},
		{
			name:    "invalid unquoted token value",
			input:   `name eq foo`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFilterExpression(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantAttr, got.Attribute)
			assert.Equal(t, tc.wantOp, got.Operator)
			assert.Equal(t, tc.wantValue, got.Value)
		})
	}
}

func TestParseFilterGroup(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantClauses int
		wantFirst   FilterExpression
		wantSecond  *FilterExpression
		wantConn    LogicalOperator
		wantErr     bool
	}{
		{
			name:        "single expression",
			input:       `name eq "Engineering"`,
			wantClauses: 1,
			wantFirst:   FilterExpression{Attribute: "name", Operator: OperatorEq, Value: "Engineering"},
		},
		{
			name:        "two clauses with AND",
			input:       `name eq "Engineering" AND handle eq "eng"`,
			wantClauses: 2,
			wantFirst:   FilterExpression{Attribute: "name", Operator: OperatorEq, Value: "Engineering"},
			wantSecond:  &FilterExpression{Attribute: "handle", Operator: OperatorEq, Value: "eng"},
			wantConn:    LogicalAnd,
		},
		{
			name:        "two clauses with OR",
			input:       `name eq "A" OR name eq "B"`,
			wantClauses: 2,
			wantFirst:   FilterExpression{Attribute: "name", Operator: OperatorEq, Value: "A"},
			wantSecond:  &FilterExpression{Attribute: "name", Operator: OperatorEq, Value: "B"},
			wantConn:    LogicalOr,
		},
		{
			name:        "three clauses mixed AND OR",
			input:       `name eq "A" AND createdAt gt "2024" OR handle eq "b"`,
			wantClauses: 3,
			wantFirst:   FilterExpression{Attribute: "name", Operator: OperatorEq, Value: "A"},
		},
		{
			name:        "gt with timestamp",
			input:       `createdAt gt "2024-01-01T00:00:00Z"`,
			wantClauses: 1,
			wantFirst:   FilterExpression{Attribute: "createdAt", Operator: OperatorGt, Value: "2024-01-01T00:00:00Z"},
		},
		{
			name:    "invalid connector",
			input:   `name eq "A" XOR name eq "B"`,
			wantErr: true,
		},
		{
			name:    "malformed second expression",
			input:   `name eq "A" AND bad`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFilterGroup(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Clauses, tc.wantClauses)
			assert.Equal(t, tc.wantFirst.Attribute, got.Clauses[0].Expr.Attribute)
			assert.Equal(t, tc.wantFirst.Operator, got.Clauses[0].Expr.Operator)
			assert.Equal(t, tc.wantFirst.Value, got.Clauses[0].Expr.Value)
			assert.Equal(t, LogicalOperator(""), got.Clauses[0].Connector)

			if tc.wantSecond != nil {
				assert.Equal(t, tc.wantSecond.Attribute, got.Clauses[1].Expr.Attribute)
				assert.Equal(t, tc.wantSecond.Operator, got.Clauses[1].Expr.Operator)
				assert.Equal(t, tc.wantSecond.Value, got.Clauses[1].Expr.Value)
				assert.Equal(t, tc.wantConn, got.Clauses[1].Connector)
			}
		})
	}
}

func TestParseFilterParam(t *testing.T) {
	t.Run("no filter parameter returns nil", func(t *testing.T) {
		q := url.Values{}
		got, err := ParseFilterParam(q)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("valid filter parameter is parsed", func(t *testing.T) {
		q := url.Values{"filter": []string{`name eq "eng"`}}
		got, err := ParseFilterParam(q)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Len(t, got.Clauses, 1)
		assert.Equal(t, "name", got.Clauses[0].Expr.Attribute)
		assert.Equal(t, OperatorEq, got.Clauses[0].Expr.Operator)
		assert.Equal(t, "eng", got.Clauses[0].Expr.Value)
	})

	t.Run("empty filter string returns error", func(t *testing.T) {
		q := url.Values{"filter": []string{"   "}}
		_, err := ParseFilterParam(q)
		assert.Error(t, err)
	})

	t.Run("invalid filter expression returns error", func(t *testing.T) {
		q := url.Values{"filter": []string{"bad filter"}}
		_, err := ParseFilterParam(q)
		assert.Error(t, err)
	})
}
