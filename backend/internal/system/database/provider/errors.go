// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"strings"

	"github.com/lib/pq"
)

// IsUniqueIndexViolation reports whether err is a unique index refusing a write.
//
// The two databases identify the index differently. PostgreSQL names it, so index is matched there.
// SQLite names the indexed columns instead, so sqliteColumns is matched there, written as SQLite
// reports them, for example `GATEWAY.DEPLOYMENT_ID`. The whole column list is matched, so a
// constraint over more columns than these is not mistaken for this one.
func IsUniqueIndexViolation(err error, index, sqliteColumns string) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505" && pqErr.Constraint == index
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed: "+sqliteColumns+" (")
}
