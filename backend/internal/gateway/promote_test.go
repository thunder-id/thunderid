// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PromoteTestSuite struct {
	suite.Suite
	f *versionFixture
}

func TestPromoteTestSuite(t *testing.T) {
	suite.Run(t, new(PromoteTestSuite))
}

// SetupTest administers two gateways, gw-1 and gw-2, under one key.
func (s *PromoteTestSuite) SetupTest() {
	s.f = newVersionFixture(s.T())
	second := s.f.gateways.gateways[0]
	second.ID, second.Name = "gw-2", "prod"
	s.f.gateways.gateways = append(s.f.gateways.gateways, second)
}

// A promotion applies the version the source runs to the target, leaving out what the source does not
// hold, and records the source as where each resource came from.
func (s *PromoteTestSuite) TestPromoteSendsWhatTheSourceRuns() {
	s.f.capture(s.T(), "ou-a", "ou-b")
	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: &[]string{"organization_unit/ou-a"}})
	s.Require().Nil(svcErr)

	result, svcErr := s.f.svc.Promote(context.Background(),
		PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: "gw-2"})

	s.Require().Nil(svcErr)
	s.Equal("gw-2", result.GatewayID)
	s.Equal(unit("ou-a"), s.f.client.last().Content, "what the source holds back was promoted")
	applied, svcErr := s.f.svc.GetApplied(context.Background(), "gw-2")
	s.Require().Nil(svcErr)
	s.Equal(s.f.hash(1), applied.Applied)
	master, svcErr := s.f.svc.GetMaster(context.Background(), "gw-2")
	s.Require().Nil(svcErr)
	s.Require().Len(master.Entries, 1)
	s.Equal("gw-1", master.Entries[0].Origin)
}

// A promotion to the source itself, or from a source that runs nothing, is refused before anything is
// sent.
func (s *PromoteTestSuite) TestPromoteRefusesItselfAndASourceThatRunsNothing() {
	s.f.capture(s.T(), "ou-a")

	_, svcErr := s.f.svc.Promote(context.Background(),
		PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: "gw-1"})
	s.Equal(&ErrorPromoteToItself, svcErr)
	_, svcErr = s.f.svc.Promote(context.Background(),
		PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: "gw-2"})
	s.Equal(&ErrorSourceRunsNothing, svcErr)
	s.Empty(s.f.client.sent)
}

// A promotion sends a removal, or an addition or update the source holds deployed with the version's
// content; not a resource the source refused or holds in an earlier form, and nothing the target holds
// back.
func (s *PromoteTestSuite) TestPromoteSelectsWhatTheSourceHolds() {
	role := func(id string) string { return "resource_type: role\nid: " + id + "\nname: " + id }
	source := &Master{GatewayID: "dev", Entries: []MasterEntry{
		{Key: "role/r1", State: EntryFailed, ContentHash: contentHash(role("r1"))},
		{Key: "role/r3", State: EntryDeployed, ContentHash: contentHash(role("r3"))},
		{Key: "role/r5", State: EntryDeployed, ContentHash: contentHash(role("r5-earlier"))},
		{Key: "role/r6", State: EntryDeployed, ContentHash: contentHash(role("r6"))},
	}}
	version := &Version{Hash: "bbb",
		Resources: joinBundle([]string{role("r1"), role("r2"), role("r3"), role("r5"), role("r6")})}
	target := &Diff{Changes: []Change{
		{Key: "role/r1", Change: ChangeAdded},
		{Key: "role/r2", Change: ChangeUnchanged},
		{Key: "role/r3", Change: ChangeUpdated},
		{Key: "role/r4", Change: ChangeDeleted},
		{Key: "role/r5", Change: ChangeAdded},
		{Key: "role/r6", Change: ChangeUpdated, HeldBack: true},
		{Key: "role/r7", Change: ChangeDeleted, HeldBack: true},
	}}

	s.Equal([]string{"role/r3", "role/r4"}, promoteSelection(source, version, target))
}
