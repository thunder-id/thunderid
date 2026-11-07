// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type PipelineTestSuite struct {
	suite.Suite
}

func TestPipelineTestSuite(t *testing.T) {
	suite.Run(t, new(PipelineTestSuite))
}

// pipelineOf is a pipeline configured with the given stages.
func pipelineOf(stages ...config.GatewayPipelineStage) pipeline {
	return pipeline{configured: stages, logger: log.GetLogger()}
}

var pipelineGateways = []Gateway{
	{ID: "gw-dev", Name: "dev", BaseURL: "https://dev.test/", IsDefault: true},
	{ID: "gw-prod", Name: "prod", BaseURL: "https://prod.test"},
}

// Without a configured pipeline, the default gateway is the one stage, named after it, with the
// console it serves.
func (s *PipelineTestSuite) TestPipelineIsTheDefaultGatewayWhenNoneIsConfigured() {
	p := pipelineOf().of(context.Background(), pipelineGateways)

	s.Require().Len(p.Stages, 1)
	s.Equal("gw-dev", p.Stages[0].Gateway.ID)
	s.Equal("dev", p.Stages[0].Name)
	s.Equal("https://dev.test/console", p.Stages[0].AdminConsoleURL)
}

// No default gateway and no pipeline is no stage, not an error.
func (s *PipelineTestSuite) TestPipelineIsEmptyWithoutADefaultGateway() {
	p := pipelineOf().of(context.Background(), []Gateway{{ID: "gw-1", Name: "one"}})

	s.NotNil(p.Stages)
	s.Empty(p.Stages)
	_, ok := pipelineOf().first(context.Background(), nil)
	s.False(ok)
}

// A configured pipeline is taken in its own order, with the names and consoles it gives, and leaves
// out a stage whose gateway is not registered.
func (s *PipelineTestSuite) TestPipelineFollowsTheConfiguredOrder() {
	p := pipelineOf(
		config.GatewayPipelineStage{Gateway: "prod"},
		config.GatewayPipelineStage{Gateway: "missing", Name: "Staging"},
		config.GatewayPipelineStage{Gateway: "dev", Name: "Development", AdminConsoleURL: "https://admin.dev.test"},
	).of(context.Background(), pipelineGateways)

	s.Require().Len(p.Stages, 2)
	s.Equal(PipelineStage{Name: "prod", AdminConsoleURL: "https://prod.test/console",
		Gateway: pipelineGateways[1]}, p.Stages[0])
	s.Equal(PipelineStage{Name: "Development", AdminConsoleURL: "https://admin.dev.test",
		Gateway: pipelineGateways[0]}, p.Stages[1])

	first, ok := pipelineOf(config.GatewayPipelineStage{Gateway: "prod"}).first(context.Background(),
		pipelineGateways)
	s.True(ok)
	s.Equal("gw-prod", first.ID)
}

// The service reads the pipeline over the registered gateways, without their keys, and reports a store
// it cannot read.
func (s *PipelineTestSuite) TestGetPipelineReadsTheRegisteredGateways() {
	store := &fakeStore{gateways: []Gateway{{ID: "gw-1", Name: "dev", BaseURL: "https://dev.test", Key: "sealed",
		IsDefault: true}}}
	svc := newService(store, newPipeline())

	p, svcErr := svc.GetPipeline(context.Background())

	s.Require().Nil(svcErr)
	s.Require().Len(p.Stages, 1)
	s.Empty(p.Stages[0].Gateway.Key)

	store.err = errors.New("database is down")
	_, svcErr = svc.GetPipeline(context.Background())
	s.Equal(&tidcommon.InternalServerError, svcErr)
}

// Auto apply deploys to the first gateway of a configured pipeline, not the default gateway.
func (s *PipelineTestSuite) TestAutoApplyAppliesToTheFirstConfiguredGateway() {
	f := autoApplyFixture(s.T(), true, false)
	svc := f.svc.(*versionService)
	store := svc.gateways.(*fakeStore)
	store.gateways = append(store.gateways, Gateway{ID: "gw-2", Name: "prod", BaseURL: "https://prod.test",
		Key: store.gateways[0].Key, IsDefault: true})
	svc.pipeline = pipelineOf(
		config.GatewayPipelineStage{Gateway: store.gateways[0].Name},
		config.GatewayPipelineStage{Gateway: "prod"},
	)

	svc.applyToFirstGateway(context.Background(), nil)

	s.Equal([]string{store.gateways[0].ID}, f.client.gatewayIDs)
}

// While the first stage's gateway cannot be found, auto apply deploys nowhere rather than to the next
// stage.
func (s *PipelineTestSuite) TestAutoApplyWaitsForTheFirstStagesGateway() {
	f := autoApplyFixture(s.T(), true, true)
	svc := f.svc.(*versionService)
	svc.pipeline = pipelineOf(
		config.GatewayPipelineStage{Gateway: "not-registered-yet"},
		config.GatewayPipelineStage{Gateway: svc.gateways.(*fakeStore).gateways[0].Name},
	)

	svc.applyToFirstGateway(context.Background(), nil)

	s.Empty(f.client.sent)
}

// The first stage is the first configured one or none: while its gateway is not registered, the next
// stage is still shown, but a deploy does not enter there.
func (s *PipelineTestSuite) TestFirstIsNoneWhileTheFirstStagesGatewayIsMissing() {
	p := pipelineOf(
		config.GatewayPipelineStage{Gateway: "missing"},
		config.GatewayPipelineStage{Gateway: "prod"},
	)

	s.Len(p.of(context.Background(), pipelineGateways).Stages, 1)
	_, ok := p.first(context.Background(), pipelineGateways)
	s.False(ok)
}

// A name two gateways have, as a declared and a registered gateway can, names neither: the stage is
// left out rather than deploying to whichever came last.
func (s *PipelineTestSuite) TestAStageNamingTwoGatewaysIsLeftOut() {
	gateways := append([]Gateway{{ID: "gw-dev-2", Name: "dev", BaseURL: "https://dev2.test"}},
		pipelineGateways...)
	p := pipelineOf(
		config.GatewayPipelineStage{Gateway: "dev"},
		config.GatewayPipelineStage{Gateway: "prod"},
	)

	stages := p.of(context.Background(), gateways).Stages
	s.Require().Len(stages, 1)
	s.Equal("gw-prod", stages[0].Gateway.ID)
	_, ok := p.first(context.Background(), gateways)
	s.False(ok)
}
