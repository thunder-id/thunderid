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

// Package ui provides terminal rendering helpers: banner, styled messages, and interactive prompts.
package ui

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

const (
	colorBrandBlue = product.ColorElectricBlue
	colorGrey      = "#808080"
	colorGreen     = "#22C55E"
	colorRed       = "#EF4444"
	colorCyan      = "#06B6D4"
	colorYellow    = "#EAB308"
)

// CyanStyle renders text in cyan.
var CyanStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorCyan))

// YellowStyle renders text in yellow.
var YellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorYellow))

var (
	brandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrandBlue))
	greyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGrey))
	greenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen))
	redStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed))
	boldStyle  = lipgloss.NewStyle().Bold(true)

	introBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBrandBlue)).
			Padding(1, 4)

	noteBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGrey)).
			Padding(0, 1)

	successBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGreen)).
			Padding(0, 1)

	errorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorRed)).
			Padding(0, 1)
)

var thunderLines = []string{
	` _____ _                     _           `,
	`|_   _| |                   | |          `,
	`  | | | |__  _   _ _ __   __| | ___ _ __ `,
	`  | | | '_ \| | | | '_ \ / _` + "`" + ` |/ _ \ '__|`,
	`  | | | | | | |_| | | | | (_| |  __/ |   `,
	`  \_/ |_| |_|\__,_|_| |_|\__,_|\___|_|   `,
}

var idLines = []string{
	` ___________ `,
	`|_   _|  _  \`,
	`  | | | | | |`,
	`  | | | | | |`,
	` _| |_| |/ / `,
	` \___/|___/  `,
}

// BannerString returns the styled ASCII art banner as a string.
func BannerString() string {
	logoWidth := 2 + len(thunderLines[0]) + len(idLines[0])

	var lines []string
	for i, t := range thunderLines {
		line := "  " + brandStyle.Render(t) + greyStyle.Render(idLines[i])
		lines = append(lines, line)
	}
	banner := strings.Join(lines, "\n")

	slogan := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorGrey)).
		Width(logoWidth).
		Align(lipgloss.Center).
		Render("Auth for Modern Apps and Agents")

	return introBoxStyle.Render(banner + "\n\n" + slogan)
}

// PrintBanner writes the styled banner to stdout.
func PrintBanner() {
	fmt.Println(BannerString())
}

// StatusBoxString returns a bordered box showing the backend and console URLs
// for a running server, styled to match the rest of the intro/banner chrome.
func StatusBoxString(baseURL string) string {
	dot := greenStyle.Render("●")
	label := lipgloss.NewStyle().Foreground(lipgloss.Color(colorGrey)).Width(9)

	rows := lipgloss.JoinVertical(lipgloss.Left,
		dot+" "+label.Render("Backend")+CyanStyle.Render(baseURL),
		dot+" "+label.Render("Console")+CyanStyle.Render(baseURL+"/console"),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorGreen)).
		Padding(0, 1).
		Render(rows)
}

// Note prints a bordered note box with title and body.
func Note(title, body string) {
	content := boldStyle.Render(title) + "\n" + greyStyle.Render(body)
	fmt.Println(noteBoxStyle.Render(content))
}

// Success prints a green success box.
func Success(msg string) {
	fmt.Println(successBoxStyle.Render(greenStyle.Render("✓ ") + msg))
}

// Outro prints a dimmed note box.
func Outro(msg string) {
	fmt.Println(noteBoxStyle.Render(greyStyle.Render(msg)))
}

// Warn prints a yellow warning box.
func Warn(msg string) {
	fmt.Println(noteBoxStyle.Render(YellowStyle.Render("⚠ " + msg)))
}

// Fatal prints a red error box.
func Fatal(msg string) {
	fmt.Println(errorBoxStyle.Render(redStyle.Render("✗ ") + msg))
}

// Bold returns a bold-rendered string.
func Bold(s string) string {
	return boldStyle.Render(s)
}

// Dim returns a grey-rendered string.
func Dim(s string) string {
	return greyStyle.Render(s)
}

// Cyan returns a cyan-rendered string.
func Cyan(s string) string {
	return CyanStyle.Render(s)
}

// Green returns a green-rendered string.
func Green(s string) string {
	return greenStyle.Render(s)
}

// Yellow returns a yellow-rendered string.
func Yellow(s string) string {
	return YellowStyle.Render(s)
}

// Red returns a red-rendered string.
func Red(s string) string {
	return redStyle.Render(s)
}

// UpgradeChoice represents the user's response to the upgrade prompt.
type UpgradeChoice int

const (
	// UpgradeNow instructs the CLI to download and apply the upgrade immediately.
	UpgradeNow UpgradeChoice = iota
	// StartCurrent skips the upgrade and starts the currently installed version.
	StartCurrent
	// SkipRelease marks the new version as skipped and starts the current version.
	SkipRelease
)

// PortConflictChoice represents the user's response to a port-in-use prompt.
type PortConflictChoice int

const (
	// KillAndUsePort kills the process on the default port and continues.
	KillAndUsePort PortConflictChoice = iota
	// UseAlternatePort starts ThunderID on an alternate port instead.
	UseAlternatePort
	// AbortSetup exits without starting ThunderID.
	AbortSetup
)

// PromptPortConflict shows a port-in-use warning and asks how to proceed.
// altPort is the next free port the caller pre-computed as an alternative.
// Returns the chosen action and the port to use.
func PromptPortConflict(port, altPort int) (PortConflictChoice, int) {
	title := redStyle.Render(fmt.Sprintf("Port %d is already in use", port))
	body := greyStyle.Render(fmt.Sprintf("%s cannot start because another process is using port %d.", product.Name, port))
	fmt.Println(noteBoxStyle.Render(title + "\n\n" + body))

	var choice PortConflictChoice
	if err := huh.NewSelect[PortConflictChoice]().
		Title("How would you like to proceed?").
		Options(
			huh.NewOption(fmt.Sprintf("Kill the process on port %d and continue", port), KillAndUsePort),
			huh.NewOption(fmt.Sprintf("Use port %d instead", altPort), UseAlternatePort),
			huh.NewOption("Abort", AbortSetup),
		).
		Value(&choice).
		Run(); err != nil {
		return AbortSetup, port
	}
	if choice == UseAlternatePort {
		return choice, altPort
	}
	return choice, port
}

// PromptUpgrade shows the "new version available" banner and asks the user what to do.
// Returns the chosen action, or StartCurrent if the prompt is cancelled.
func PromptUpgrade(currentVersion, newVersion string) UpgradeChoice {
	title := YellowStyle.Render("✦ " + product.Name + " v" + newVersion + " is available")
	body := greyStyle.Render("You have v" + currentVersion + " installed.\nUpgrade for the latest features and security fixes.")
	fmt.Println(noteBoxStyle.Render(title + "\n\n" + body))

	var choice UpgradeChoice
	if err := huh.NewSelect[UpgradeChoice]().
		Title("What would you like to do?").
		Options(
			huh.NewOption("Upgrade now", UpgradeNow),
			huh.NewOption("Start v"+currentVersion+" (upgrade later)", StartCurrent),
			huh.NewOption("Skip v"+newVersion, SkipRelease),
		).
		Value(&choice).
		Run(); err != nil {
		return StartCurrent
	}
	return choice
}
