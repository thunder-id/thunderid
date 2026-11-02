// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailexec

import (
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the authorization details execution service.
func Initialize(
	typeProvider providers.AuthorizationDetailTypeProvider, processorTimeout time.Duration,
) providers.AuthorizationDetailProvider {
	return newAuthorizationDetailExecService(typeProvider, newProcessorClient(processorTimeout))
}
