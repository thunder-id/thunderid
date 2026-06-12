/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package ui

// ConfigInput describes one value the user must supply before the sample runs.
// If Choices is non-empty the TUI renders a list picker; otherwise a text input.
type ConfigInput struct {
	Key     string   // env var key written to the target .env, e.g. "LLM_PROVIDER"
	Label   string   // prompt text shown to the user
	Choices []Choice // non-empty → list picker; empty → text input
	Secret  bool     // mask text input with EchoPassword
}

// Choice is a single option in a ConfigInput list picker.
type Choice struct {
	Value string // stored in the collected config map
	Label string // displayed in the TUI
}

// Usecase describes a try-able auth use case, shared by the onboarding picker and slash commands.
type Usecase struct {
	Emoji           string
	Title           string
	Description     string
	SampleName      string // empty = coming soon
	Command         string // slash command, e.g. "/try-consumer"
	ComingSoon      bool
	RequiredConfigs []ConfigInput // fields to collect before the sample starts; nil = no prompt
	SampleEnvTarget string        // service sub-dir to write collected config into, e.g. "ai-agent"
	SampleFeatures  []string      // feature tags passed to the sample runner, e.g. ["ai"]
}

// Usecases is the canonical list of try-able auth use cases.
var Usecases = []Usecase{
	{
		Emoji:       "👤",
		Title:       "Consumer Login (B2C)",
		Description: "Sign in with email, social providers, passkeys and MFA",
		SampleName:  "wayfinder",
		Command:     "/try-consumer",
	},
	{
		Emoji:       "🤖",
		Title:       "Agent Login (AgentID)",
		Description: "Secure access for AI agents and automated workflows acting on behalf of users",
		SampleName:  "wayfinder",
		Command:     "/try-agentid",
		RequiredConfigs: []ConfigInput{
			{
				Key:   "LLM_PROVIDER",
				Label: "LLM provider for the AI concierge",
				Choices: []Choice{
					{Value: "anthropic", Label: "Anthropic (Claude)"},
					{Value: "gemini", Label: "Gemini"},
				},
			},
			{
				Key:    "LLM_API_KEY",
				Label:  "API key",
				Secret: true,
			},
		},
		SampleEnvTarget: "ai-agent",
		SampleFeatures:  []string{"ai"},
	},
}
