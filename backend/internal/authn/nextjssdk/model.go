// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package nextjssdk

// integrateNextJSSDKInput represents the input for the integrate_nextjs_sdk tool.
type integrateNextJSSDKInput struct {
	ServerURL string `json:"server_url,omitempty" jsonschema:"URL of the server, which is the SDK base URL"`
}

// integrateNextJSSDKOutput represents the output for the integrate_nextjs_sdk tool.
type integrateNextJSSDKOutput struct {
	Instructions string `json:"instructions"`
	CodeSnippets string `json:"code_snippets"`
}
