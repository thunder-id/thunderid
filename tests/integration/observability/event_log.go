// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package observability verifies, end to end against a running server, that the observability events
// published for authentication flows and token issuance describe the principals involved: which
// entity acted, which entity the credential was issued for, and whether the two differ.
//
// The events are read back from the file sink, which is the only client-visible surface they have.
package observability

import (
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Event data keys asserted by these tests. They mirror event.DataKey in the server, which the tests
// cannot import: the integration suite is a separate module driving the server over HTTP. Spelling
// them out here is deliberate, since the key names are the contract an event consumer reads.
const (
	keyAppID         = "app_id"
	keyClientID      = "client_id"
	keyCorrelationID = "correlation_id"
	keyActorType     = "act_type"
	keyActorSub      = "act_sub"
	keySubject       = "sub"
	keySubjectType   = "sub_type"
	keyIsDelegated   = "is_delegated"
	keyNodeID        = "node_id"
	keyGrantType     = "grant_type"
	keyExecutionID   = "execution_id"
)

// Event types asserted by these tests.
const (
	typeFlowStarted         = "FLOW_STARTED"
	typeFlowCompleted       = "FLOW_COMPLETED"
	typeNodeExecCompleted   = "FLOW_NODE_EXECUTION_COMPLETED"
	typeTokenIssued         = "TOKEN_ISSUED"
	typeTokenIssuanceFailed = "TOKEN_ISSUANCE_FAILED"
)

// Principal type values asserted by these tests. They are the wire values of the token's sub_type
// claim, so "app" the entity category is reported as "application" here.
const (
	principalUser        = "user"
	principalAgent       = "agent"
	principalApplication = "application"
)

// eventLogPath is where the suite points the file sink. It sits under the extracted product home so
// it is cleaned up with the rest of the test distribution.
func eventLogPath() string {
	return testutils.EventLogPath("integration-events.log")
}
