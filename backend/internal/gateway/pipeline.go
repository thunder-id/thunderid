// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// adminConsolePath is where a gateway serves its own console, under its base URL.
const adminConsolePath = "/console"

// PipelineStage is one stage a version is deployed and promoted through: a gateway, named.
type PipelineStage struct {
	// Name is what the stage is called.
	Name string `json:"name"`
	// AdminConsoleURL is where the gateway's own console is reached, to manage its users and
	// sessions.
	AdminConsoleURL string `json:"adminConsoleUrl"`
	// Gateway is the gateway the stage runs on.
	Gateway Gateway `json:"gateway"`
}

// Pipeline is the stages a version moves through, in order: a deploy enters at the first, and
// each later one is reached by promoting from the one before it.
type Pipeline struct {
	Stages []PipelineStage `json:"stages"`
}

// pipeline is the deploy pipeline as gateway.pipeline configures it.
type pipeline struct {
	configured []config.GatewayPipelineStage
	logger     *log.Logger
}

// newPipeline reads the pipeline from configuration.
func newPipeline() pipeline {
	p := pipeline{logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayPipeline"))}
	if config.IsServerRuntimeInitialized() {
		p.configured = config.GetServerRuntime().Config.Gateway.Pipeline
	}
	return p
}

// of lays the configured pipeline over the registered gateways. A stage is left out when nothing can
// be deployed to it for certain: the gateway it names is not registered, or two gateways have that
// name, as a gateway declared in a file and one registered through the API can. Without a configured
// pipeline, the default gateway is the one stage.
func (p pipeline) of(ctx context.Context, gateways []Gateway) Pipeline {
	stages, _ := p.resolve(ctx, gateways)
	return Pipeline{Stages: stages}
}

// first is the gateway a deploy enters the pipeline at, if there is one. It is the first configured
// stage's gateway or none: while that gateway is not registered yet, or after it is renamed or removed,
// a deploy must not enter at the next stage instead.
func (p pipeline) first(ctx context.Context, gateways []Gateway) (Gateway, bool) {
	stages, firstResolved := p.resolve(ctx, gateways)
	if len(stages) == 0 || !firstResolved {
		return Gateway{}, false
	}
	return stages[0].Gateway, true
}

// resolve finds each stage's gateway, and reports whether the first stage is among those found.
func (p pipeline) resolve(ctx context.Context, gateways []Gateway) ([]PipelineStage, bool) {
	stages := []PipelineStage{}
	if len(p.configured) == 0 {
		for _, gw := range gateways {
			if gw.IsDefault {
				stages = append(stages, p.stage(gw, config.GatewayPipelineStage{}))
			}
		}
		return stages, true
	}
	byName := make(map[string]Gateway, len(gateways))
	ambiguous := map[string]bool{}
	for _, gw := range gateways {
		if _, seen := byName[gw.Name]; seen {
			ambiguous[gw.Name] = true
		}
		byName[gw.Name] = gw
	}
	firstResolved := false
	for i, configured := range p.configured {
		gw, ok := byName[configured.Gateway]
		switch {
		case !ok:
			p.logger.Warn(ctx, "A pipeline stage names a gateway that is not registered; leaving it out",
				log.String("gateway", configured.Gateway))
			continue
		case ambiguous[configured.Gateway]:
			p.logger.Warn(ctx, "A pipeline stage names a gateway name more than one gateway has; leaving it out",
				log.String("gateway", configured.Gateway))
			continue
		}
		if i == 0 {
			firstResolved = true
		}
		stages = append(stages, p.stage(gw, configured))
	}
	return stages, firstResolved
}

// stage names a gateway's place in the pipeline, falling back to the gateway's own name and console.
func (p pipeline) stage(gw Gateway, configured config.GatewayPipelineStage) PipelineStage {
	stage := PipelineStage{Name: configured.Name, AdminConsoleURL: configured.AdminConsoleURL, Gateway: gw}
	if stage.Name == "" {
		stage.Name = gw.Name
	}
	if stage.AdminConsoleURL == "" {
		stage.AdminConsoleURL = strings.TrimRight(gw.BaseURL, "/") + adminConsolePath
	}
	return stage
}
