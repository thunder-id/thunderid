// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package execution holds integration tests for the flow engine itself, as opposed to the tests
// grouped by flow type in the sibling packages. What is exercised here is behaviour the engine
// applies to every flow: execution and resume, expiry, call frames and the call depth limit,
// administration entry points, error branches and the events published along the way.
package execution

import "github.com/thunder-id/thunderid/tests/integration/testutils"

// testServerURL is the base URL every request in this package is issued against.
const testServerURL = testutils.TestServerURL

// flowConfigSection names the flow section of the server configuration.
const flowConfigSection = "flow"

// administrationFlowType is the flow type of the shipped administration flows.
const administrationFlowType = "ADMINISTRATION"
