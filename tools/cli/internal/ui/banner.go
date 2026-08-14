// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package ui provides terminal rendering helpers: banner, styled messages, and interactive prompts.
package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	huhspinner "charm.land/huh/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
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

// maxBoxWidth caps the bordered boxes so they do not overflow a standard terminal, and
// terminalWidth reports how much room there actually is.
const maxBoxWidth = 100

// terminalWidth returns the current terminal width, or fallbackWidth when stdout is not
// a terminal (a pipe, a CI log) and has no width to report. It is a variable so tests
// can render at a chosen width.
var terminalWidth = func() int {
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	return fallbackWidth
}

// fallbackWidth is the classic terminal width, used when the real one is unknown.
const fallbackWidth = 80

// boxWidth returns the width a bordered box may render at: the terminal minus the
// border and padding it draws around its content, capped at maxBoxWidth.
func boxWidth(horizontalChrome int) int {
	width := terminalWidth()
	if width > maxBoxWidth {
		width = maxBoxWidth
	}
	width -= horizontalChrome
	if width < 20 {
		width = 20
	}
	return width
}

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

// fitBox renders content in a bordered box sized to the terminal, wrapping long lines
// instead of letting the border run off the edge. horizontalChrome is the width the
// style spends on its own border and padding.
func fitBox(style lipgloss.Style, horizontalChrome int, content string) string {
	return style.Width(boxWidth(horizontalChrome)).Render(content)
}

// Box chrome widths: 2 border columns plus the style's horizontal padding.
const (
	noteChrome  = 2 + 1*2
	introChrome = 2 + 4*2
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

// slogan is shown under the product name in the banner.
const slogan = "Auth for Modern Apps and Agents"

// BannerString returns the styled ASCII art banner, or a compact variant when the
// terminal is too narrow for the art and its box: the art is a fixed width, so on a
// small terminal the border would wrap and every line would break.
func BannerString() string {
	logoWidth := 2 + len(thunderLines[0]) + len(idLines[0])
	if terminalWidth() < logoWidth+introChrome {
		return fitBox(noteBoxStyle, noteChrome,
			brandStyle.Render("⚡ "+product.Name)+"\n"+greyStyle.Render(slogan))
	}

	var lines []string
	for i, t := range thunderLines {
		line := "  " + brandStyle.Render(t) + greyStyle.Render(idLines[i])
		lines = append(lines, line)
	}
	banner := strings.Join(lines, "\n")

	centeredSlogan := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorGrey)).
		Width(logoWidth).
		Align(lipgloss.Center).
		Render(slogan)

	return introBoxStyle.Render(banner + "\n\n" + centeredSlogan)
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
	fmt.Println(fitBox(noteBoxStyle, noteChrome, content))
}

// Success prints a green success box.
func Success(msg string) {
	fmt.Println(fitBox(successBoxStyle, noteChrome, greenStyle.Render("✓ ")+msg))
}

// Outro prints a dimmed note box.
func Outro(msg string) {
	fmt.Println(fitBox(noteBoxStyle, noteChrome, greyStyle.Render(msg)))
}

// Warn prints a yellow warning box.
func Warn(msg string) {
	fmt.Println(fitBox(noteBoxStyle, noteChrome, YellowStyle.Render("⚠ "+msg)))
}

// Fatal prints a red error box.
func Fatal(msg string) {
	fmt.Println(fitBox(errorBoxStyle, noteChrome, redStyle.Render("✗ ")+msg))
}

// FatalDetail prints a failure whose message carries captured output (a setup
// transcript, a log tail) after a blank line: the detail is printed as plain lines and
// only the summary goes in the error box, which would otherwise be too wide to read.
func FatalDetail(msg string) {
	if idx := strings.Index(msg, "\n\n"); idx != -1 {
		detail := strings.TrimSpace(msg[idx+2:])
		if detail != "" {
			fmt.Println()
			for _, line := range strings.Split(detail, "\n") {
				fmt.Println("  " + line)
			}
			fmt.Println()
		}
		msg = strings.TrimSpace(msg[:idx])
	}
	Fatal(msg)
}

// SpinnerTheme renders the blocking spinners (setup, startup) in the product colors.
func SpinnerTheme() huhspinner.Theme {
	return huhspinner.ThemeFunc(func(bool) *huhspinner.Styles {
		return &huhspinner.Styles{
			Spinner: lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrandBlue)).PaddingLeft(2),
			Title:   lipgloss.NewStyle(),
		}
	})
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
// altPort is the next free port the caller pre-computed as an alternative; pass 0
// when the install cannot move ports, and only freeing the port or aborting is
// offered. holders, when known, are listed so the operator sees what would be
// stopped. Returns the chosen action and the port to use.
func PromptPortConflict(port, altPort int, holders []setup.PortHolder) (PortConflictChoice, int) {
	title := redStyle.Render(fmt.Sprintf("Port %d is already in use", port))
	body := greyStyle.Render(fmt.Sprintf("%s cannot start because another process is using port %d.", product.Name, port))
	if held := holderLines(holders); held != "" {
		body += "\n" + held
	}
	if altPort <= 0 {
		body += "\n" + greyStyle.Render(fmt.Sprintf("This install is already set up on port %d and cannot be moved.", port))
	}
	fmt.Println(fitBox(noteBoxStyle, noteChrome, title+"\n\n"+body))

	var choice PortConflictChoice
	if err := huh.NewSelect[PortConflictChoice]().
		Title("How would you like to proceed?").
		Options(portConflictOptions(port, altPort)...).
		Value(&choice).
		Run(); err != nil {
		return AbortSetup, port
	}
	if choice == UseAlternatePort {
		return choice, altPort
	}
	return choice, port
}

// portConflictOptions lists the answers to a port-in-use prompt. The alternate-port
// answer appears only when the caller supplied one.
func portConflictOptions(port, altPort int) []huh.Option[PortConflictChoice] {
	options := []huh.Option[PortConflictChoice]{
		huh.NewOption(fmt.Sprintf("Kill the process on port %d and continue", port), KillAndUsePort),
	}
	if altPort > 0 {
		options = append(options, huh.NewOption(fmt.Sprintf("Use port %d instead", altPort), UseAlternatePort))
	}
	return append(options, huh.NewOption("Abort", AbortSetup))
}

// ConfirmStopPortHolders lists the processes occupying the ports a sample app needs
// and asks whether to stop them. The sample services bind fixed ports written into
// their generated config, so moving them is not an option: the choice is stop or
// cancel. Returns false when the operator declines; a non-interactive run has nobody to
// ask, so it warns and continues like the port-conflict path in cli.resolvePort.
func ConfirmStopPortHolders(holders []setup.PortHolder) bool {
	title := redStyle.Render("Ports needed by the sample app are in use")
	body := greyStyle.Render("The sample services bind these fixed ports.")
	if held := holderLines(holders); held != "" {
		body += "\n" + held
	}
	fmt.Println(fitBox(noteBoxStyle, noteChrome, title+"\n\n"+body))

	if !Interactive() {
		// A scripted or CI run cannot answer, and huh would fail immediately. Continue
		// as before rather than reporting a cancellation nobody chose; the box above
		// already named what is being stopped.
		Warn("Not running interactively, so the processes holding these ports are being stopped.")
		return true
	}

	var stop bool
	if err := huh.NewSelect[bool]().
		Title("How would you like to proceed?").
		Options(
			huh.NewOption("Stop these processes and continue", true),
			huh.NewOption("Cancel, leave them running", false),
		).
		Value(&stop).
		Run(); err != nil {
		return false
	}
	return stop
}

// holderLines renders one indented line per port holder, or "" when none are known.
func holderLines(holders []setup.PortHolder) string {
	lines := make([]string, 0, len(holders))
	for _, h := range holders {
		lines = append(lines, greyStyle.Render("  "+h.String()))
	}
	return strings.Join(lines, "\n")
}

// Interactive reports whether stdin is a terminal, so callers can skip a prompt in
// scripted and CI runs instead of asking a question nothing can answer. A CI run
// usually gets /dev/null on stdin, which is a character device but no terminal, so
// the terminal itself is what has to be tested for.
func Interactive() bool {
	return term.IsTerminal(os.Stdin.Fd())
}

// PromptAdminCredentials interactively collects the admin username and password
// before setup runs. The defaults (defaultUsername and generatedPassword) are shown
// as the values that will be used if the operator provides nothing, mirroring
// setup.sh; neither field is prefilled with editable text. Pressing Enter on the
// password field accepts the generated one, in which case usedGenerated is true.
//
// It returns ok=false (a no-op) when stdin is not an interactive terminal, so
// scripted and CI runs fall through to setup's own defaults.
func PromptAdminCredentials(defaultUsername, generatedPassword string) (username, password string, usedGenerated, ok bool) {
	if !Interactive() {
		return "", "", false, false
	}
	var userInput, passInput string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Configure the default admin user").
				Description("Press Enter to accept the shown default."),
			huh.NewInput().
				Title("Admin username").
				Description("Default: "+defaultUsername).
				Placeholder(defaultUsername).
				Value(&userInput),
			huh.NewInput().
				Title("Admin password").
				Description("Press Enter to use the generated password: "+generatedPassword).
				Placeholder("leave blank to use the generated password").
				EchoMode(huh.EchoModePassword).
				Value(&passInput),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", false, false
	}
	username = strings.TrimSpace(userInput)
	if username == "" {
		username = defaultUsername
	}
	if passInput == "" {
		return username, generatedPassword, true, true
	}
	return username, passInput, false, true
}

// PrintCredentialsFallback prints generated admin credentials to stdout when the
// REPL that normally displays them will not be reached (e.g. startup failed). Setup
// does not regenerate the password on a later run, so this is the last chance to
// surface it. No-op when no credentials were generated.
func PrintCredentialsFallback(creds *setup.AdminCredentials) {
	if creds == nil {
		return
	}
	fmt.Println("  " + Bold("Admin credentials"))
	fmt.Println("    Username: " + Cyan(creds.Username))
	fmt.Println("    Password: " + Cyan(creds.Password))
	fmt.Println("    " + Dim("Sign in to the Console with these credentials once it is running."))
	fmt.Println()
}

// PromptUpgrade shows the "new version available" banner and asks the user what to do.
// Returns the chosen action, or StartCurrent if the prompt is cancelled.
func PromptUpgrade(currentVersion, newVersion string) UpgradeChoice {
	title := YellowStyle.Render("✦ " + product.Name + " v" + newVersion + " is available")
	body := greyStyle.Render("You have v" + currentVersion + " installed.\nUpgrade for the latest features and security fixes.")
	fmt.Println(fitBox(noteBoxStyle, noteChrome, title+"\n\n"+body))

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
