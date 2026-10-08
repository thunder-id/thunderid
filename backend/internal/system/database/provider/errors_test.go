// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
)

func TestIsUniqueIndexViolation(t *testing.T) {
	const index = "idx_gateway_default_deployment"
	const columns = "GATEWAY.DEPLOYMENT_ID"

	cases := map[string]struct {
		err  error
		want bool
	}{
		"postgres names the index":  {&pq.Error{Code: "23505", Constraint: index}, true},
		"postgres, wrapped":         {fmt.Errorf("insert: %w", &pq.Error{Code: "23505", Constraint: index}), true},
		"postgres, another index":   {&pq.Error{Code: "23505", Constraint: "gateway_name_key"}, false},
		"postgres, another failure": {&pq.Error{Code: "40P01", Constraint: index}, false},
		"sqlite names the columns": {
			errors.New("constraint failed: UNIQUE constraint failed: GATEWAY.DEPLOYMENT_ID (2067)"), true},
		"sqlite, a wider constraint": {
			errors.New("constraint failed: UNIQUE constraint failed: GATEWAY.NAME, GATEWAY.DEPLOYMENT_ID (2067)"),
			false},
		"another error": {errors.New("database is down"), false},
		"no error":      {nil, false},
	}
	for name, tc := range cases {
		if got := IsUniqueIndexViolation(tc.err, index, columns); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}
