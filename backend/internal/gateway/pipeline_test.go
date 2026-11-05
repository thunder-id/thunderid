// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// pipelineOf is a pipeline configured with the given stages.
func pipelineOf(stages ...config.GatewayPipelineStage) pipeline {
	return pipeline{configured: stages, logger: log.GetLogger()}
}

var pipelineGateways = []Gateway{
	{ID: "gw-dev", Name: "dev", BaseURL: "https://dev.test/", IsDefault: true},
	{ID: "gw-prod", Name: "prod", BaseURL: "https://prod.test"},
}

// Without a configured pipeline, the default gateway is the one environment, named after it, with the
// console it serves.
func TestPipelineIsTheDefaultGatewayWhenNoneIsConfigured(t *testing.T) {
	p := pipelineOf().of(context.Background(), pipelineGateways)

	require.Len(t, p.Stages, 1)
	assert.Equal(t, "gw-dev", p.Stages[0].Gateway.ID)
	assert.Equal(t, "dev", p.Stages[0].Name)
	assert.Equal(t, "https://dev.test/console", p.Stages[0].AdminConsoleURL)
}

// No default gateway and no pipeline is no environment, not an error.
func TestPipelineIsEmptyWithoutADefaultGateway(t *testing.T) {
	p := pipelineOf().of(context.Background(), []Gateway{{ID: "gw-1", Name: "one"}})

	assert.NotNil(t, p.Stages)
	assert.Empty(t, p.Stages)
	_, ok := pipelineOf().first(context.Background(), nil)
	assert.False(t, ok)
}

// A configured pipeline is taken in its own order, with the names and consoles it gives, and leaves
// out a stage whose gateway is not registered.
func TestPipelineFollowsTheConfiguredOrder(t *testing.T) {
	p := pipelineOf(
		config.GatewayPipelineStage{Gateway: "prod"},
		config.GatewayPipelineStage{Gateway: "missing", Name: "Staging"},
		config.GatewayPipelineStage{Gateway: "dev", Name: "Development", AdminConsoleURL: "https://admin.dev.test"},
	).of(context.Background(), pipelineGateways)

	require.Len(t, p.Stages, 2)
	assert.Equal(t, PipelineStage{Name: "prod", AdminConsoleURL: "https://prod.test/console",
		Gateway: pipelineGateways[1]}, p.Stages[0])
	assert.Equal(t, PipelineStage{Name: "Development", AdminConsoleURL: "https://admin.dev.test",
		Gateway: pipelineGateways[0]}, p.Stages[1])

	first, ok := pipelineOf(config.GatewayPipelineStage{Gateway: "prod"}).first(context.Background(),
		pipelineGateways)
	assert.True(t, ok)
	assert.Equal(t, "gw-prod", first.ID)
}

// The service reads the pipeline over the registered gateways, without their keys, and reports a store
// it cannot read.
func TestGetPipelineReadsTheRegisteredGateways(t *testing.T) {
	store := &fakeStore{gateways: []Gateway{{ID: "gw-1", Name: "dev", BaseURL: "https://dev.test", Key: "sealed",
		IsDefault: true}}}
	svc := newService(store)

	p, svcErr := svc.GetPipeline(context.Background())

	require.Nil(t, svcErr)
	require.Len(t, p.Stages, 1)
	assert.Empty(t, p.Stages[0].Gateway.Key)

	store.err = errors.New("database is down")
	_, svcErr = svc.GetPipeline(context.Background())
	assert.Equal(t, &tidcommon.InternalServerError, svcErr)
}

// Auto apply deploys to the first environment of a configured pipeline, not the default gateway.
func TestAutoApplyAppliesToTheFirstConfiguredEnvironment(t *testing.T) {
	f := autoApplyFixture(t, true, false)
	svc := f.svc.(*versionService)
	svc.pipeline = pipelineOf(config.GatewayPipelineStage{Gateway: "dev"})

	svc.applyToFirstEnvironment(context.Background())

	assert.Len(t, f.client.sent, 1)
}
