// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package reactsdk

// integrateReactSDKInput represents the input for the integrate_react_sdk tool.
type integrateReactSDKInput struct {
	ServerURL string `json:"server_url,omitempty" jsonschema:"URL of the server, which is the SDK base URL"`
}

// integrateReactSDKOutput represents the output for the integrate_react_sdk tool.
type integrateReactSDKOutput struct {
	Instructions string `json:"instructions"`
	CodeSnippets string `json:"code_snippets"`
}
