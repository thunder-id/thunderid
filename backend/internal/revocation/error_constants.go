// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import "fmt"

// CriteriaLimitExceededError is returned when one change would record more criteria than allowed.
type CriteriaLimitExceededError struct {
	Criteria int
	Max      int
}

// Error implements the error interface.
func (e *CriteriaLimitExceededError) Error() string {
	return fmt.Sprintf("revocation of %d criteria exceeds the limit of %d per change", e.Criteria, e.Max)
}
