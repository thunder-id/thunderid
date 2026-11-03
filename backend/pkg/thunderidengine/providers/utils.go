// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

// GrantedInstead reads alternatives supplied by the authorization engine's typed field.
// Generic external policy context is stripped before this entry is published.
func GrantedInstead(evaluationContext map[string]interface{}) []string {
	substituted, _ := evaluationContext[ContextKeyGrantedInstead].([]string)
	return substituted
}
