// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type DisabledIdentityGovernanceProviderTestSuite struct {
	suite.Suite
}

func TestDisabledIdentityGovernanceProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DisabledIdentityGovernanceProviderTestSuite))
}

func (s *DisabledIdentityGovernanceProviderTestSuite) TestDisabledProviderRejectsLifecycleOperations() {
	ctx := context.Background()
	provider := newDisabledIdentityGovernanceProvider()
	for _, operation := range []func(context.Context, string) *tidcommon.ServiceError{
		provider.Unlock, provider.Unsuspend, provider.ValidateSuspend,
	} {
		err := operation(ctx, "e1")
		s.Require().NotNil(err)
		s.Equal(errGovernanceUnavailable.Code, err.Code)
	}

	err := provider.Suspend(ctx, "e1", "")
	s.Require().NotNil(err)
	s.Equal(errGovernanceUnavailable.Code, err.Code)
}

// With governance off there is nothing to stamp, and that is not an error for the caller.
func (s *DisabledIdentityGovernanceProviderTestSuite) TestDisabledProviderRecordsNothing() {
	s.Nil(newDisabledIdentityGovernanceProvider().RecordLogin(context.Background(), "e1"))
}
