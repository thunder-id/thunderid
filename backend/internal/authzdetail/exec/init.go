// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailexec

import (
	"time"

	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the authorization details execution service. Processors are called through
// a client from clients, which refuses private addresses unless the deployment allows them.
func Initialize(
	typeProvider providers.AuthorizationDetailTypeProvider, processorTimeout time.Duration,
	clients httpservice.OutboundClientProviderInterface,
) providers.AuthorizationDetailProvider {
	return newAuthorizationDetailExecService(typeProvider, newProcessorClient(processorTimeout, clients))
}
