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

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thunder-id/thunderid/tools/cli/internal/commands/sample"
	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/health"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
	"github.com/thunder-id/thunderid/tools/cli/internal/utils"
)

// SlashCommand represents a / command available in the REPL.
// Action (sync) or AsyncAction (async tea.Cmd) handles execution; AsyncAction takes priority.
type SlashCommand struct {
	Name        string
	Description string
	Section     string // category label; same value = same group in the completion list
	ComingSoon  bool
	Action      func(baseURL string) ([]string, error)
	AsyncAction func(baseURL string) tea.Cmd
}

var defaultCommands = []SlashCommand{
	{
		Name:        "/open-console",
		Description: "Open the Console in your browser",
		Action: func(baseURL string) ([]string, error) {
			url := baseURL + "/console"
			if err := utils.OpenBrowser(url); err != nil {
				return nil, err
			}
			return []string{Dim("Opening " + url + "...")}, nil
		},
	},
	{
		Name:        "/status",
		Description: "Show server status",
		Action: func(baseURL string) ([]string, error) {
			if health.CheckReady(baseURL) {
				return []string{Green("●") + " " + product.Name + " is running at " + Cyan(baseURL)}, nil
			}
			return []string{Yellow("○") + " " + product.Name + " is not responding"}, nil
		},
	},
	{
		Name:        "/upgrade",
		Description: "Upgrade " + product.Name + " to the latest version",
		AsyncAction: func(_ string) tea.Cmd {
			return func() tea.Msg { return upgradeMsg{} }
		},
	},
	{
		Name:        "/stop",
		Description: "Stop " + product.Name + " and exit",
		Action:      nil, // handled specially in Update
	},
}

// --- bubbletea messages ---

type healthCheckMsg struct{ ready bool }
type cutoverMsg struct{}
type upgradeMsg struct{}
type thunderExitedMsg struct {
	err error
	pid int // PID of the process that exited — used to ignore stale watches
}

// sampleStartedMsg is sent immediately when a try-* command begins.
// It carries the live channels so the model can stream progress.
type sampleStartedMsg struct {
	sampleName string
	progressCh <-chan sample.ProgressEvent
	resultCh   <-chan sample.Result
}

// sampleProgressMsg carries a single progress event from an async try-* operation.
type sampleProgressMsg struct {
	line      string
	overwrite bool // when true, drives the bottom-status line instead of appending to messages
}

// sampleProgressDoneMsg is sent when the progress channel closes (no more lines).
type sampleProgressDoneMsg struct{}

// sampleDoneMsg signals that the try-* operation completed successfully.
type sampleDoneMsg struct {
	proc       *exec.Cmd
	sampleName string
	sampleURL  string
	serverURL  string // confirmed-ready base URL from ResolveBaseURL
	features   []string
}

// sampleErrMsg signals that the try-* operation failed.
type sampleErrMsg struct{ err error }

// usecaseConfigRequestMsg is sent when a use case requires additional config before starting.
type usecaseConfigRequestMsg struct {
	sampleName string
	inputs     []ConfigInput
	envTarget  string
	features   []string
}

// walkthroughPane is one tab in the post-sample walkthrough overlay.
type walkthroughPane struct {
	Title string
	Lines []string // body lines; empty string = blank line
	URL   string   // opened with 'o'
}

func b2cWalkthroughPanes(sampleURL string) []walkthroughPane {
	return []walkthroughPane{
		{
			Title: "Log In",
			URL:   sampleURL,
			Lines: []string{
				"Sign in with the demo consumer account.",
				"",
				"  1  Open the Wayfinder app at " + Cyan(sampleURL),
				"  2  Click Sign in and enter:",
				"",
				"     username  " + Bold("john.doe"),
				"     password  " + Bold("john.doe"),
			},
		},
		{
			Title: "Self Sign-Up",
			URL:   sampleURL,
			Lines: []string{
				"Create a new account via self-registration.",
				"",
				"  1  Open " + Cyan(sampleURL),
				"  2  Click Sign in → Register.",
				"  3  Fill in your details and submit.",
			},
		},
		{
			Title: "View Profile",
			URL:   sampleURL,
			Lines: []string{
				"Explore the user profile page.",
				"",
				"  1  Sign in as " + Bold("john.doe") + " / " + Bold("john.doe"),
				"  2  Click your name in the top-right corner.",
				"  3  Select Profile.",
			},
		},
		{
			Title: "Account Recovery",
			URL:   sampleURL,
			Lines: []string{
				"Trigger the forgot-password flow.",
				"",
				"  1  Open " + Cyan(sampleURL) + " and click Sign in.",
				"  2  Click Forgot password?",
				"  3  Enter your email and follow the instructions.",
				"",
				Dim("  Requires SMTP configured in deployment.yaml."),
			},
		},
		{
			Title: "Onboard Staff",
			URL:   sampleURL,
			Lines: []string{
				"Admin-invite a new internal user.",
				"",
				"  1  Sign in as " + Bold("alex.carter") + " / " + Bold("alex.carter") + Dim("  (Admin)"),
				"  2  Open the Admin panel.",
				"  3  Invite a new user by email.",
			},
		},
	}
}

func agentWalkthroughPanes(sampleURL string) []walkthroughPane {
	return []walkthroughPane{
		{
			Title: "AI Concierge",
			URL:   sampleURL,
			Lines: []string{
				"Chat with the AI travel concierge.",
				"",
				"  1  Open the Wayfinder app at " + Cyan(sampleURL),
				"  2  Click the chat bubble in the bottom-right corner.",
				"  3  Ask about available flights.",
			},
		},
		{
			Title: "Book via Agent",
			URL:   sampleURL,
			Lines: []string{
				"Let the agent book a flight on your behalf.",
				"",
				"  1  Open the chat and ask the concierge to book a flight.",
				"  2  The agent requests user consent — approve the prompt.",
				"  3  The booking is created in your name.",
			},
		},
		{
			Title: "Agent Identity",
			URL:   sampleURL + "/signin-as-agent",
			Lines: []string{
				"Sign in as the AI agent directly.",
				"",
				"  1  Open " + Cyan(sampleURL+"/signin-as-agent"),
				"  2  The gate shows the Agent ID / Secret form.",
				"  3  Enter the agent credentials to authenticate.",
			},
		},
	}
}

// choiceItem wraps a Choice value for use in a bubbletea list.
type choiceItem struct{ choice Choice }

func (c choiceItem) FilterValue() string { return "" }

// choiceDelegate renders single-line choice items.
type choiceDelegate struct{}

func (choiceDelegate) Height() int                             { return 1 }
func (choiceDelegate) Spacing() int                            { return 0 }
func (choiceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (choiceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ci, ok := item.(choiceItem)
	if !ok {
		return
	}
	if index == m.Index() {
		fmt.Fprintln(w, "  "+brandStyle.Render("❯ ")+Bold(ci.choice.Label)) //nolint:errcheck
	} else {
		fmt.Fprintln(w, "    "+Dim(ci.choice.Label)) //nolint:errcheck
	}
}

// --- model ---

type serverStatus int

const (
	statusStarting serverStatus = iota
	statusReady
	statusStopped
)

// ReplModel is the bubbletea model for the interactive REPL.
type ReplModel struct {
	input   textinput.Model
	spinner spinner.Model

	messages []string
	commands []SlashCommand

	status      serverStatus
	version     string
	baseURL     string
	installPath string
	verbose     bool

	showCompletions bool
	completions     []SlashCommand
	selectedComp    int

	proc             *exec.Cmd
	sampleProgressCh <-chan sample.ProgressEvent
	// trySampleStatus holds the current inline-overwrite line (progress bar or
	// "Extracting…") shown in the spinner area at the bottom of the REPL while
	// a try-* operation is running.
	trySampleStatus string
	tryingOut       bool
	quitting        bool
	width           int

	showOnboarding    bool
	onboardingList    list.Model
	onboardingCmdMode bool // true while the slash-command input overlay is active
	checkPort         int  // non-zero overrides health.DefaultPort for health checks
	cutoverRequested  bool // set when the /cutover command is executed
	upgradeRequested  bool // set when the /upgrade command is executed
	newVersion        string

	showWalkthrough  bool
	walkthroughPanes []walkthroughPane
	walkthroughTab   int

	// Generic use-case config collection — active when showUsecaseConfig is true.
	showUsecaseConfig bool
	ucInputs          []ConfigInput
	ucValues          map[string]string
	ucStep            int
	ucList            list.Model
	ucText            textinput.Model
	ucSampleName      string
	ucEnvTarget       string
	ucFeatures        []string
}

// NewReplModel initializes the REPL model.
func NewReplModel(version string, proc *exec.Cmd, installPath string, verbose bool, isFirstRun bool) ReplModel {
	ti := textinput.New()
	ti.Placeholder = "Starting " + product.Name + "..."
	ti.Prompt = "> "
	ti.CharLimit = 256

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrandBlue))

	var commands []SlashCommand
	for _, u := range Usecases {
		u := u
		ip := installPath
		if u.ComingSoon {
			commands = append(commands, SlashCommand{
				Name:        u.Command,
				Description: u.Title + "  · Coming Soon",
				Section:     "Try",
				ComingSoon:  true,
				Action: func(_ string) ([]string, error) {
					return []string{Yellow("⏳") + " " + Bold(u.Title) + " is coming soon."}, nil
				},
			})
		} else if len(u.RequiredConfigs) > 0 {
			commands = append(commands, SlashCommand{
				Name:        u.Command,
				Description: u.Title,
				Section:     "Try",
				AsyncAction: func(_ string) tea.Cmd {
					return func() tea.Msg {
						return usecaseConfigRequestMsg{
							sampleName: u.SampleName,
							inputs:     u.RequiredConfigs,
							envTarget:  u.SampleEnvTarget,
							features:   u.SampleFeatures,
						}
					}
				},
			})
		} else {
			commands = append(commands, SlashCommand{
				Name:        u.Command,
				Description: u.Title,
				Section:     "Try",
				AsyncAction: func(_ string) tea.Cmd {
					return makeTryCmd(u.SampleName, ip, verbose, sample.Options{})
				},
			})
		}
	}
	logCmd := SlashCommand{
		Name:        "/logs",
		Description: "Show recent server logs",
		Action: func(_ string) ([]string, error) {
			logPath := setup.LogFile(installPath)
			data, err := os.ReadFile(logPath)
			if err != nil {
				return nil, fmt.Errorf("could not read logs: %w", err)
			}
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			const maxLines = 30
			if len(lines) > maxLines {
				lines = lines[len(lines)-maxLines:]
			}
			out := make([]string, 0, len(lines)+1)
			out = append(out, Dim(fmt.Sprintf("── last %d lines of %s ──", len(lines), logPath)))
			for _, l := range lines {
				out = append(out, Dim(l))
			}
			return out, nil
		},
	}
	commands = append(commands, logCmd)
	commands = append(commands, defaultCommands...)

	return ReplModel{
		input:          ti,
		spinner:        s,
		commands:       commands,
		version:        version,
		installPath:    installPath,
		verbose:        verbose,
		status:         statusStarting,
		proc:           proc,
		width:          80,
		showOnboarding: isFirstRun,
		onboardingList: newOnboardingList(80),
	}
}

// makeTryCmd starts RunAsync and immediately returns sampleStartedMsg so the
// model can begin streaming progress without blocking the event loop.
func makeTryCmd(sampleName, installPath string, verbose bool, opts sample.Options) tea.Cmd {
	return func() tea.Msg {
		progressCh, resultCh := sample.RunAsync(sampleName, installPath, verbose, opts)
		return sampleStartedMsg{sampleName: sampleName, progressCh: progressCh, resultCh: resultCh}
	}
}

// waitForSampleProgress reads one event from the progress channel.
// Returns sampleProgressMsg, or sampleProgressDoneMsg when the channel closes.
func waitForSampleProgress(ch <-chan sample.ProgressEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return sampleProgressDoneMsg{}
		}
		return sampleProgressMsg{line: ev.Line, overwrite: ev.Overwrite}
	}
}

// waitForSampleResult blocks until the result channel delivers its single value.
func waitForSampleResult(sampleName string, ch <-chan sample.Result) tea.Cmd {
	return func() tea.Msg {
		r := <-ch
		if r.Err != nil {
			return sampleErrMsg{err: r.Err}
		}
		return sampleDoneMsg{proc: r.Proc, sampleName: sampleName, sampleURL: r.SampleURL, serverURL: r.ServerURL, features: r.Features}
	}
}

func (m ReplModel) effectivePort() int {
	if m.checkPort > 0 {
		return m.checkPort
	}
	return health.DefaultPort
}

// Init implements tea.Model.
func (m ReplModel) Init() tea.Cmd {
	p := m.effectivePort()
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		func() tea.Msg { return doHealthCheckOn(p) },
		pollHealthCmdOn(p),
		watchProcessCmd(m.proc),
	)
}

func pollHealthCmdOn(port int) tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return doHealthCheckOn(port)
	})
}

func doHealthCheckOn(port int) tea.Msg {
	for _, scheme := range []string{"https", "http"} {
		base := fmt.Sprintf("%s://localhost:%d", scheme, port)
		if health.CheckReady(base) {
			return healthCheckMsg{ready: true}
		}
	}
	return healthCheckMsg{ready: false}
}

func watchProcessCmd(proc *exec.Cmd) tea.Cmd {
	if proc == nil || proc.Process == nil {
		return nil
	}
	pid := proc.Process.Pid
	return func() tea.Msg {
		err := proc.Wait()
		return thunderExitedMsg{err: err, pid: pid}
	}
}

// newChoiceList builds a bubbletea list for a set of Choice values.
func newChoiceList(choices []Choice, width int) list.Model {
	items := make([]list.Item, len(choices))
	for i, c := range choices {
		items[i] = choiceItem{c}
	}
	height := len(choices)*choiceDelegate{}.Height() + 2
	l := list.New(items, choiceDelegate{}, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	return l
}

// initUCStep prepares the UI widget for the current config-collection step.
func (m *ReplModel) initUCStep() {
	if m.ucStep >= len(m.ucInputs) {
		return
	}
	inp := m.ucInputs[m.ucStep]
	if len(inp.Choices) > 0 {
		m.ucList = newChoiceList(inp.Choices, m.width)
	} else {
		ti := textinput.New()
		ti.Placeholder = "enter value…"
		ti.Prompt = "  > "
		ti.CharLimit = 512
		if inp.Secret {
			ti.EchoMode = textinput.EchoPassword
		}
		ti.Focus()
		m.ucText = ti
	}
}

// advanceUCStep records value for the current step then moves to the next.
// When all steps are done it clears the config state and returns a makeTryCmd.
func (m *ReplModel) advanceUCStep(value string) tea.Cmd {
	m.ucValues[m.ucInputs[m.ucStep].Key] = value
	m.ucStep++
	if m.ucStep < len(m.ucInputs) {
		m.initUCStep()
		return nil
	}
	m.showUsecaseConfig = false
	m.tryingOut = true
	m.input.Blur()
	opts := sample.Options{
		Config:    m.ucValues,
		EnvTarget: m.ucEnvTarget,
		Features:  m.ucFeatures,
	}
	return makeTryCmd(m.ucSampleName, m.installPath, m.verbose, opts)
}

// Update implements tea.Model.
func (m ReplModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:cyclop,funlen
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.onboardingList.SetSize(msg.Width, onboardingListHeight)

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			m.killThunder()
			return m, tea.Quit
		}

		if m.showOnboarding && m.status == statusReady {
			if m.onboardingCmdMode {
				// ── Slash-command overlay ──────────────────────────────────────
				switch msg.Type {
				case tea.KeyEsc:
					m.onboardingCmdMode = false
					m.input.SetValue("")
					m.input.Blur()
					m.showCompletions = false
					m.selectedComp = 0
				case tea.KeyEnter:
					val := strings.TrimSpace(m.input.Value())
					if m.showCompletions && len(m.completions) > 0 {
						val = m.completions[m.selectedComp].Name
					}
					if val != "" {
						m.showOnboarding = false
						m.onboardingCmdMode = false
						m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
						m.messages = append(m.messages, "> "+val)
						m.input.SetValue("")
						m.showCompletions = false
						m.selectedComp = 0
						if cmd := m.runCommand(val); cmd != nil {
							cmds = append(cmds, cmd)
						}
					}
				case tea.KeyUp:
					if m.showCompletions && m.selectedComp > 0 {
						m.selectedComp--
					}
				case tea.KeyDown:
					if m.showCompletions && m.selectedComp < len(m.completions)-1 {
						m.selectedComp++
					}
				case tea.KeyTab:
					if m.showCompletions && len(m.completions) > 0 {
						m.input.SetValue(m.completions[m.selectedComp].Name)
						m.input.CursorEnd()
					}
				}
			} else {
				// ── Onboarding list navigation ─────────────────────────────────
				if msg.Type == tea.KeyEnter {
					if cmd := m.selectOnboarding(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				} else if msg.Type == tea.KeyRunes && (msg.String() == "/" || msg.String() == "?") {
					m.onboardingCmdMode = true
					m.input.Focus()
					m.input.SetValue("/")
					m.input.CursorEnd()
				} else {
					prevIdx := m.onboardingList.Index()
					var listCmd tea.Cmd
					m.onboardingList, listCmd = m.onboardingList.Update(msg)
					cmds = append(cmds, listCmd)
					if item, ok := m.onboardingList.SelectedItem().(onboardingItem); ok && item.comingSoon {
						m.onboardingList.Select(prevIdx)
					}
				}
			}
		} else if m.showUsecaseConfig {
			// ── Generic use-case config collection ────────────────────────────
			inp := m.ucInputs[m.ucStep]
			if len(inp.Choices) > 0 {
				switch msg.Type {
				case tea.KeyEnter:
					if ci, ok := m.ucList.SelectedItem().(choiceItem); ok {
						if cmd := m.advanceUCStep(ci.choice.Value); cmd != nil {
							cmds = append(cmds, cmd)
						}
					}
				default:
					var listCmd tea.Cmd
					m.ucList, listCmd = m.ucList.Update(msg)
					cmds = append(cmds, listCmd)
				}
			} else {
				switch msg.Type {
				case tea.KeyEnter:
					if val := strings.TrimSpace(m.ucText.Value()); val != "" {
						if cmd := m.advanceUCStep(val); cmd != nil {
							cmds = append(cmds, cmd)
						}
					}
				default:
					var tiCmd tea.Cmd
					m.ucText, tiCmd = m.ucText.Update(msg)
					cmds = append(cmds, tiCmd)
				}
			}
		} else if m.showWalkthrough {
			// ── Walkthrough tab navigation ─────────────────────────────────────
			switch {
			case msg.Type == tea.KeyLeft:
				if m.walkthroughTab > 0 {
					m.walkthroughTab--
				}
			case msg.Type == tea.KeyRight:
				if m.walkthroughTab < len(m.walkthroughPanes)-1 {
					m.walkthroughTab++
				}
			case msg.Type == tea.KeyRunes && msg.String() == "o":
				if pane := m.walkthroughPanes[m.walkthroughTab]; pane.URL != "" {
					utils.OpenBrowser(pane.URL) //nolint:errcheck
				}
			case msg.Type == tea.KeyEsc:
				m.showWalkthrough = false
				m.input.Focus()
			case msg.Type == tea.KeyRunes && msg.String() == "/":
				m.showWalkthrough = false
				m.input.Focus()
				m.input.SetValue("/")
				m.input.CursorEnd()
			}
		} else {
			// ── Regular REPL ───────────────────────────────────────────────────
			switch msg.Type {
			case tea.KeyEnter:
				if m.status != statusReady {
					break
				}
				val := strings.TrimSpace(m.input.Value())
				if val == "" {
					break
				}
				if m.showCompletions && len(m.completions) > 0 {
					val = m.completions[m.selectedComp].Name
				}
				m.messages = append(m.messages, "> "+val)
				m.input.SetValue("")
				m.showCompletions = false
				m.selectedComp = 0
				if cmd := m.runCommand(val); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case tea.KeyUp:
				if m.showCompletions && m.selectedComp > 0 {
					m.selectedComp--
				}
			case tea.KeyDown:
				if m.showCompletions && m.selectedComp < len(m.completions)-1 {
					m.selectedComp++
				}
			case tea.KeyTab:
				if m.showCompletions && len(m.completions) > 0 {
					m.input.SetValue(m.completions[m.selectedComp].Name)
					m.input.CursorEnd()
				}
			}
		}

	case usecaseConfigRequestMsg:
		m.ucInputs = msg.inputs
		m.ucSampleName = msg.sampleName
		m.ucEnvTarget = msg.envTarget
		m.ucFeatures = msg.features

		// Pre-populate from a previous run so the user is not re-prompted.
		sampleDir := sample.SampleDir(m.installPath, msg.sampleName)
		m.ucValues = sample.ReadServiceEnv(sampleDir, msg.envTarget)

		// Advance past any steps that already have a non-empty saved value.
		m.ucStep = 0
		for m.ucStep < len(m.ucInputs) {
			if val, ok := m.ucValues[m.ucInputs[m.ucStep].Key]; ok && val != "" {
				m.ucStep++
			} else {
				break
			}
		}

		if m.ucStep >= len(m.ucInputs) {
			// All values already present — launch immediately without prompting.
			m.tryingOut = true
			m.input.Blur()
			opts := sample.Options{
				Config:    m.ucValues,
				EnvTarget: m.ucEnvTarget,
				Features:  m.ucFeatures,
			}
			cmds = append(cmds, makeTryCmd(m.ucSampleName, m.installPath, m.verbose, opts))
		} else {
			m.showUsecaseConfig = true
			m.input.Blur()
			m.initUCStep()
		}

	case healthCheckMsg:
		if msg.ready {
			if m.status == statusStarting {
				port := m.effectivePort()
				for _, scheme := range []string{"https", "http"} {
					base := fmt.Sprintf("%s://localhost:%d", scheme, port)
					if health.CheckReady(base) {
						m.baseURL = base
						break
					}
				}
				if m.baseURL == "" {
					m.baseURL = fmt.Sprintf("http://localhost:%d", port)
				}
				m.status = statusReady
				if m.showOnboarding {
					// Input stays blurred; user enters command mode explicitly with / or ?
				} else {
					m.input.Focus()
					m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
					m.messages = append(m.messages,
						Green("●")+" "+product.Name+" is running at "+Cyan(m.baseURL),
					)
				}
				if m.newVersion != "" {
					m.messages = append(m.messages,
						Yellow("✦")+" "+Bold(product.Name+" v"+m.newVersion+" is available")+" — type "+Cyan("/upgrade")+" to upgrade",
					)
				}
			}
			// Always keep polling so we can detect crashes via health check.
			cmds = append(cmds, pollHealthCmdOn(m.effectivePort()))
		} else {
			// Only report "stopped responding" when the product was healthy and we
			// are not deliberately restarting it for a try-* operation.
			if m.status == statusReady && !m.tryingOut {
				m.status = statusStopped
				m.input.Blur()
				m.input.Placeholder = product.Name + " stopped. Ctrl+C to exit."
				m.messages = append(m.messages, Red("✗")+" "+product.Name+" stopped responding.")
			}
			if m.status != statusStopped || m.tryingOut {
				cmds = append(cmds, pollHealthCmdOn(m.effectivePort()))
			}
		}

	case thunderExitedMsg:
		// Two independent guards — either one is sufficient to suppress the message:
		// 1. tryingOut: kill was intentional (try-* restart is in progress).
		// 2. PID mismatch: stale watch from a previous proc that was already replaced.
		if m.tryingOut {
			break
		}
		currentPID := 0
		if m.proc != nil && m.proc.Process != nil {
			currentPID = m.proc.Process.Pid
		}
		if msg.pid != currentPID {
			break
		}
		m.status = statusStopped
		m.input.Blur()
		m.input.Placeholder = product.Name + " stopped. Ctrl+C to exit."
		m.messages = append(m.messages, Red("✗")+" "+product.Name+" process exited unexpectedly.")

	case sampleStartedMsg:
		m.sampleProgressCh = msg.progressCh
		cmds = append(cmds,
			waitForSampleProgress(msg.progressCh),
			waitForSampleResult(msg.sampleName, msg.resultCh),
		)

	case sampleProgressMsg:
		if msg.overwrite {
			// Drive the bottom-status line — same role as the \r overwrite in CLI mode.
			m.trySampleStatus = msg.line
		} else {
			// A status line arrived (Stopping…, Writing…, Starting…): clear the
			// bottom progress bar so the spinner shows a neutral state.
			m.trySampleStatus = ""
			m.messages = append(m.messages, "  "+msg.line)
		}
		cmds = append(cmds, waitForSampleProgress(m.sampleProgressCh))

	case sampleProgressDoneMsg:
		// Progress channel closed — result channel will deliver the final outcome.

	case sampleDoneMsg:
		m.tryingOut = false
		m.trySampleStatus = ""
		m.sampleProgressCh = nil
		m.proc = msg.proc
		// The server was confirmed ready by ResolveBaseURL before the sample
		// services started. Mark it ready now so the normal health-check
		// stopped-detection fires immediately if the sample's start.sh kills
		// and fails to restart it, rather than spinning on "Starting…" forever.
		if msg.serverURL != "" {
			m.baseURL = msg.serverURL
			m.status = statusReady
			m.input.Focus()
			m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
		} else {
			m.status = statusStarting
			m.input.Placeholder = "Starting " + product.Name + "..."
		}
		cmds = append(cmds, pollHealthCmdOn(m.effectivePort()))
		m.messages = append(m.messages, Green("✓")+" "+msg.sampleName+" is live at "+Cyan(msg.sampleURL))
		if msg.sampleName == "wayfinder" {
			hasAI := false
			for _, f := range msg.features {
				if f == "ai" {
					hasAI = true
					break
				}
			}
			if hasAI {
				m.walkthroughPanes = agentWalkthroughPanes(msg.sampleURL)
			} else {
				m.walkthroughPanes = b2cWalkthroughPanes(msg.sampleURL)
			}
			m.walkthroughTab = 0
			m.showWalkthrough = true
			m.input.Blur()
		}

	case sampleErrMsg:
		m.tryingOut = false
		m.trySampleStatus = ""
		m.sampleProgressCh = nil
		m.messages = append(m.messages, Red("✗")+" "+msg.err.Error())
		if m.status == statusReady {
			m.input.Focus()
			m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
		}

	case cutoverMsg:
		m.cutoverRequested = true
		m.quitting = true
		return m, tea.Quit

	case upgradeMsg:
		m.killThunder()
		m.upgradeRequested = true
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinCmd)
	}

	var tiCmd tea.Cmd
	m.input, tiCmd = m.input.Update(msg)
	cmds = append(cmds, tiCmd)

	m.updateCompletions()
	return m, tea.Batch(cmds...)
}

func (m *ReplModel) updateCompletions() {
	val := m.input.Value()
	if val == "/" {
		m.completions = m.commands
		m.showCompletions = true
		if m.selectedComp >= len(m.completions) {
			m.selectedComp = 0
		}
		return
	}
	if !strings.HasPrefix(val, "/") {
		m.showCompletions = false
		m.completions = nil
		return
	}
	filter := strings.ToLower(strings.TrimSpace(val))
	var matches []SlashCommand
	for _, c := range m.commands {
		if strings.HasPrefix(strings.ToLower(c.Name), filter) {
			matches = append(matches, c)
		}
	}
	m.completions = matches
	m.showCompletions = len(matches) > 0
	if m.selectedComp >= len(matches) {
		m.selectedComp = 0
	}
}

func (m *ReplModel) runCommand(val string) tea.Cmd {
	if val == "/stop" {
		m.killThunder()
		return tea.Quit
	}
	if m.tryingOut {
		m.messages = append(m.messages, Yellow("⏳")+" Please wait — setup is in progress.")
		return nil
	}
	for _, c := range m.commands {
		if c.Name != val {
			continue
		}
		if c.AsyncAction != nil {
			m.tryingOut = true
			m.input.Blur()
			return c.AsyncAction(m.baseURL)
		}
		if c.Action != nil {
			lines, err := c.Action(m.baseURL)
			m.messages = append(m.messages, lines...)
			if err != nil {
				m.messages = append(m.messages, Red("✗")+" "+err.Error())
			}
		}
		return nil
	}
	if !strings.HasPrefix(val, "/") {
		return nil
	}
	m.messages = append(m.messages, Yellow("?")+" Unknown command. "+Dim("Type / to see available commands."))
	return nil
}

func (m *ReplModel) killThunder() {
	if m.proc == nil || m.proc.Process == nil {
		return
	}
	// SIGTERM lets start.sh's cleanup trap kill ThunderID and the consent server
	// before exiting. SIGKILL would bypass the trap and leave port 9090 occupied,
	// causing the next invocation to fail.
	m.proc.Process.Signal(syscall.SIGTERM) //nolint:errcheck
	time.Sleep(time.Second)
}

func renderCompletions(m ReplModel) string {
	if !m.showCompletions || len(m.completions) == 0 {
		return ""
	}
	var b strings.Builder
	separator := Dim(strings.Repeat("─", clamp(m.width-2, 20, 80)))
	b.WriteString(separator + "\n")
	const nameW = 24
	lastSection := ""
	for i, c := range m.completions {
		if c.Section != lastSection {
			if i > 0 {
				b.WriteString("\n")
			}
			if c.Section != "" {
				b.WriteString("  " + Dim(c.Section) + "\n")
			}
			lastSection = c.Section
		}
		var namePart, descPart string
		indicator := "  "
		if c.ComingSoon {
			namePart = Dim(fmt.Sprintf("%-*s", nameW, c.Name))
			descPart = Dim(c.Description)
		} else if i == m.selectedComp {
			indicator = "▶ "
			namePart = lipgloss.NewStyle().Foreground(lipgloss.Color(colorCyan)).Bold(true).Render(fmt.Sprintf("%-*s", nameW, c.Name))
			descPart = lipgloss.NewStyle().Foreground(lipgloss.Color(colorCyan)).Render(c.Description)
		} else {
			namePart = Dim(fmt.Sprintf("%-*s", nameW, c.Name))
			descPart = Dim(c.Description)
		}
		b.WriteString("  " + indicator + namePart + "  " + descPart + "\n")
	}
	b.WriteString(separator + "\n")
	return b.String()
}

func renderWalkthrough(m ReplModel) string {
	if len(m.walkthroughPanes) == 0 {
		return ""
	}
	var b strings.Builder

	var tabParts []string
	for i, p := range m.walkthroughPanes {
		if i == m.walkthroughTab {
			tabParts = append(tabParts, lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorCyan)).
				Bold(true).
				Underline(true).
				Render(p.Title))
		} else {
			tabParts = append(tabParts, Dim(p.Title))
		}
	}
	b.WriteString("  " + strings.Join(tabParts, Dim("  ·  ")) + "\n")
	b.WriteString("  " + Dim(strings.Repeat("─", clamp(m.width-4, 20, 76))) + "\n\n")

	pane := m.walkthroughPanes[m.walkthroughTab]
	for _, line := range pane.Lines {
		b.WriteString("  " + line + "\n")
	}

	b.WriteString("\n")
	hint := Dim("  ← → switch tabs")
	if pane.URL != "" {
		hint += Dim("  •  o open in browser")
	}
	hint += Dim("  •  esc dismiss  •  / for commands")
	b.WriteString(hint + "\n")

	return b.String()
}

// View implements tea.Model.
func (m ReplModel) View() string {
	if m.quitting {
		return Dim("Stopping " + product.Name + "...\n")
	}

	var b strings.Builder

	b.WriteString(BannerString() + "\n")

	statusPart := ""
	switch m.status {
	case statusStarting:
		statusPart = m.spinner.View() + " Starting..."
	case statusReady:
		statusPart = Green("●") + " Running at " + Cyan(m.baseURL)
	case statusStopped:
		statusPart = Red("○") + " Stopped"
	}

	b.WriteString(Bold("⚡ "+product.Name+" v"+m.version) + "  " + statusPart + "\n")
	b.WriteString(Dim(strings.Repeat("─", clamp(m.width-2, 20, 80))) + "\n\n")

	if m.showOnboarding && m.status == statusReady {
		if m.onboardingCmdMode {
			// Slash-command overlay: show completions and input, no list.
			b.WriteString(renderCompletions(m))
			b.WriteString(m.input.View())
			b.WriteString("\n\n" + Dim("  esc back to use-case picker"))
		} else {
			// List mode with custom hint replacing the list's built-in help.
			b.WriteString(strings.TrimRight(m.onboardingList.View(), "\n"))
			b.WriteString("\n" + Dim("  ↑/k up  •  ↓/j down  •  / commands"))
		}
		return b.String()
	}

	if m.showUsecaseConfig {
		inp := m.ucInputs[m.ucStep]
		b.WriteString("  " + Bold(inp.Label) + "\n\n")
		if len(inp.Choices) > 0 {
			b.WriteString(m.ucList.View())
			b.WriteString("\n" + Dim("  ↑/↓ select  •  Enter to continue"))
		} else {
			b.WriteString(m.ucText.View() + "\n")
			b.WriteString("\n" + Dim("  Enter to continue"))
		}
		return b.String()
	}

	for _, msg := range m.messages {
		b.WriteString("  " + msg + "\n")
	}
	if len(m.messages) > 0 {
		b.WriteString("\n")
	}

	if m.showWalkthrough {
		b.WriteString(renderWalkthrough(m))
		return b.String()
	}

	b.WriteString(renderCompletions(m))

	switch {
	case m.tryingOut && m.trySampleStatus != "":
		b.WriteString(m.spinner.View() + " " + m.trySampleStatus)
	case m.tryingOut:
		b.WriteString(m.spinner.View() + Dim(" Please wait… (Ctrl+C to abort)"))
	case m.status == statusStarting:
		b.WriteString(m.spinner.View() + Dim(" Starting "+product.Name+"…"))
	default:
		b.WriteString(m.input.View())
	}
	return b.String()
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// RunREPL starts the interactive REPL and blocks until the user exits.
// newVersion, if non-empty, causes a banner to appear prompting the user to /upgrade.
// Returns upgradeRequested=true when the user ran /upgrade.
func RunREPL(
	version string, proc *exec.Cmd, installPath string,
	verbose, isFirstRun bool, newVersion string,
) (upgradeRequested bool, err error) {
	m := NewReplModel(version, proc, installPath, verbose, isFirstRun)
	m.newVersion = newVersion
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, runErr := p.Run()
	if rm, ok := finalModel.(ReplModel); ok {
		return rm.upgradeRequested, runErr
	}
	return false, runErr
}

// RunStagingREPL runs the REPL connected to a staging instance on stagingPort.
// It injects a /cutover command; when the user runs it the REPL exits and
// cutoverRequested=true is returned so the caller can perform the cut-over.
func RunStagingREPL(version string, proc *exec.Cmd, installPath string, verbose bool, stagingPort int) (cutoverRequested bool, err error) {
	m := NewReplModel(version, proc, installPath, verbose, false)
	m.checkPort = stagingPort
	m.commands = append([]SlashCommand{
		{
			Name:        "/cutover",
			Description: "Cut over to this version and restart on the default port",
			AsyncAction: func(_ string) tea.Cmd {
				return func() tea.Msg { return cutoverMsg{} }
			},
		},
	}, m.commands...)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, runErr := p.Run()
	if rm, ok := finalModel.(ReplModel); ok {
		return rm.cutoverRequested, runErr
	}
	return false, runErr
}
