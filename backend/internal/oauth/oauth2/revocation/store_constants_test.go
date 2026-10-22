// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type StoreConstantsTestSuite struct {
	suite.Suite
}

func TestStoreConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(StoreConstantsTestSuite))
}

// The criterion upsert is one statement for both engines, so its text must be valid PostgreSQL:
// positional parameters, the unique key as the conflict target, and the boundary guard comparing the
// stored cutoff with the incoming one.
func (s *StoreConstantsTestSuite) TestCriterionUpsert_PostgresStatement() {
	q := queryInsertRevocationCriterion
	s.Empty(q.PostgresQuery)
	s.Empty(q.SQLiteQuery)
	for i := 1; i <= 7; i++ {
		s.Contains(q.Query, fmt.Sprintf("$%d", i))
	}
	s.Contains(q.Query, `ON CONFLICT (DEPLOYMENT_ID, CRITERION_TYPE, CRITERION_VALUE) DO UPDATE SET `)
	s.Contains(q.Query, `OR (excluded.REASON IN (`+boundaryReasonSQLList()+`) `+
		`AND "REVOCATION_CRITERIA".REVOKED_AT > excluded.REVOKED_AT) `)
	s.NotContains(q.Query, "?")
}
