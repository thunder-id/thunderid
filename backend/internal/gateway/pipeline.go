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

// PipelineStage is one environment a version is deployed and promoted through: a gateway, named.
type PipelineStage struct {
	// Name is what the environment is called.
	Name string `json:"name"`
	// AdminConsoleURL is where the environment's own console is reached, to manage its users and
	// sessions.
	AdminConsoleURL string `json:"adminConsoleUrl"`
	// Gateway is the gateway the environment runs on.
	Gateway Gateway `json:"gateway"`
}

// Pipeline is the environments a version moves through, in order: a deploy enters at the first, and
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

// of lays the configured pipeline over the registered gateways. A stage naming a gateway that is not
// registered is left out, since nothing can be deployed to it. Without a configured pipeline, the
// default gateway is the one environment.
func (p pipeline) of(ctx context.Context, gateways []Gateway) Pipeline {
	stages := []PipelineStage{}
	if len(p.configured) == 0 {
		for _, gw := range gateways {
			if gw.IsDefault {
				stages = append(stages, p.stage(gw, config.GatewayPipelineStage{}))
			}
		}
		return Pipeline{Stages: stages}
	}
	byName := make(map[string]Gateway, len(gateways))
	for _, gw := range gateways {
		byName[gw.Name] = gw
	}
	for _, configured := range p.configured {
		gw, ok := byName[configured.Gateway]
		if !ok {
			p.logger.Warn(ctx, "A pipeline stage names a gateway that is not registered; leaving it out",
				log.String("gateway", configured.Gateway))
			continue
		}
		stages = append(stages, p.stage(gw, configured))
	}
	return Pipeline{Stages: stages}
}

// first is the gateway a deploy enters the pipeline at, if there is one.
func (p pipeline) first(ctx context.Context, gateways []Gateway) (Gateway, bool) {
	stages := p.of(ctx, gateways).Stages
	if len(stages) == 0 {
		return Gateway{}, false
	}
	return stages[0].Gateway, true
}

// stage names a gateway's environment, falling back to the gateway's own name and console.
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
