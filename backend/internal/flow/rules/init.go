// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package rules is the rule engine of DECISION nodes. It parses a decision block, compiles it into an
// immutable plan, and evaluates the plan with three-valued logic, so that missing data is never
// treated as false.
package rules

// Initialize creates the rule engine with the default limits.
func Initialize() EngineInterface {
	return newRuleEngine(defaultLimits())
}
