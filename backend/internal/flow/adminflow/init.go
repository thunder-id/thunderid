// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package adminflow

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/serverconfig"
)

// Initialize builds the administration flow runner. Only a server that executes flows has one.
func Initialize(serverConfig serverconfig.ServerConfigService, flows providers.FlowProvider,
	executor flowexec.FlowExecServiceInterface) AdministrationFlowRunnerInterface {
	return newAdministrationFlowRunner(serverConfig, flows, executor)
}
