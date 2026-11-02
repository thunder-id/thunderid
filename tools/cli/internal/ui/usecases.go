// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ui

// ConfigInput describes one value the user must supply before the sample runs.
// If Choices is non-empty the TUI renders a list picker; otherwise a text input.
type ConfigInput struct {
	Key          string   // env var key written to the target .env, e.g. "LLM_PROVIDER"
	Label        string   // prompt text shown to the user
	Instructions []string // dimmed lines shown below the label, before the input
	Choices      []Choice // non-empty → list picker; empty → text input
	Secret       bool     // mask text input with EchoPassword
	Optional     bool     // allow Enter with empty value to skip this step
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
		Title:       "Secured Web Application",
		Description: "Tryout user journeys of a secured web application",
		SampleName:  "wayfinder",
		Command:     "/try-consumer",
	},
	{
		Emoji:       "🤖",
		Title:       "Secured AI Agent",
		Description: "Tryout identity security patterns for AI agents and tools",
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
