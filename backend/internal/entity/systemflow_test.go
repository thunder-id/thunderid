// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// With system flows off, as on a control plane, only the change runs.
func (s *ServiceTestSuite) TestRevokeAround_WithSystemFlowsOff() {
	svc := s.svc.(*entityService)
	ran := false

	s.NoError(svc.revokeAround(s.ctx, "e-off", systemFlowEntityDeletePre, systemFlowRevocationPost,
		func() error { ran = true; return nil }))
	s.True(ran)
}
