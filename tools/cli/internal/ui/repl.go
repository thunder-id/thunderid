// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/thunder-id/thunderid/tools/cli/internal/commands/integrate"
	"github.com/thunder-id/thunderid/tools/cli/internal/commands/sample"
	"github.com/thunder-id/thunderid/tools/cli/internal/product"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/docs"
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
		Name:        "/status",
		Description: "Show server status",
		Section:     "Server",
		Action: func(baseURL string) ([]string, error) {
			if health.CheckReady(baseURL) {
				return []string{
					Green("●") + " " + product.Name + " is running at " + Cyan(baseURL),
					Green("●") + " Console: " + Cyan(baseURL+"/console"),
				}, nil
			}
			return []string{Yellow("○") + " " + product.Name + " is not responding"}, nil
		},
	},
	{
		Name:        "/stop",
		Description: "Stop " + product.Name + " and exit",
		Section:     "Server",
		Action:      nil, // handled specially in Update
	},
	{
		Name:        "/upgrade",
		Description: "Upgrade " + product.Name + " to the latest version",
		Section:     "Versioning",
		AsyncAction: func(_ string) tea.Cmd {
			return func() tea.Msg { return upgradeMsg{} }
		},
	},
	{
		Name:        "/switch",
		Description: "Switch to another installed version",
		Section:     "Versioning",
		AsyncAction: func(_ string) tea.Cmd {
			return func() tea.Msg { return switchVersionMsg{} }
		},
	},
	{
		Name:        "/open-console",
		Description: "Open the Console in your browser",
		Section:     "Apps",
		Action: func(baseURL string) ([]string, error) {
			url := baseURL + "/console"
			if err := utils.OpenBrowser(url); err != nil {
				return nil, err
			}
			return []string{Dim("Opening " + url + "...")}, nil
		},
	},
}

// defaultInputWidth is the fallback textinput width used before the first
// tea.WindowSizeMsg arrives. Without an explicit width, bubbles' textinput
// truncates its placeholder to a single character.
const defaultInputWidth = 76

// --- bubbletea messages ---

type healthCheckMsg struct{ ready bool }

// logsTailMsg carries newly appended log lines discovered by a follow-mode tick.
// ok is false on an I/O error reading the log; the offset is left unchanged in
// that case so the next tick retries the same read.
type logsTailMsg struct {
	lines  []string
	offset int64
	ok     bool
}
type upgradeMsg struct{}
type switchVersionMsg struct{}
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

// integrateFrameworkMsg triggers fetching and displaying a platform's integration guide.
type integrateFrameworkMsg struct{ framework string }

// guideLoadedMsg carries the result of fetching an integration guide's markdown.
type guideLoadedMsg struct {
	markdown string
	err      error
}

// usecaseConfigRequestMsg is sent when a use case requires additional config before starting.
type usecaseConfigRequestMsg struct {
	sampleName string
	inputs     []ConfigInput
	envTarget  string
	features   []string
}

// sampleTryRequestMsg starts a use case that needs no configuration. The slash
// commands are built before the REPL knows which port the product ended up on,
// so the run is dispatched from Update, where the model can supply it.
type sampleTryRequestMsg struct {
	sampleName string
	features   []string
}

// walkthroughPane is one tab in the post-sample walkthrough overlay.
type walkthroughPane struct {
	Title string
	Lines []string // body lines; empty string = blank line
	URL   string   // opened with 'o'
}

// mailInboxURL is the SMTP test-inbox UI shipped with the Wayfinder sample.
const mailInboxURL = "http://localhost:8788"

// b2cWalkthroughPanes mirrors the Console's Secured Web Application tryout
// scenarios (welcome.applicationTryout.scenarios.* in the frontend i18n
// locale) so the CLI and Console walk users through the same journeys.
func b2cWalkthroughPanes(sampleURL, consoleURL string) []walkthroughPane {
	return []walkthroughPane{
		{
			Title: "Sign-In",
			URL:   sampleURL,
			Lines: []string{
				"Sign in with the test user account to explore " + product.Name + " Sign in experience.",
				"",
				"  1  Open the Wayfinder app at " + Cyan(sampleURL) + ".",
				"  2  Click Sign in and use the credentials below.",
				"",
				"     " + Dim("username") + "  " + Highlight("john.doe"),
				"     " + Dim("password") + "  " + Highlight("john.doe"),
				"",
				"  " + Bold("View Profile"),
				"  " + Dim("Explore the self-service profile page - view account details, edit attributes, and change your password."),
				"",
				"  3  Click the username in the top-right corner and select Profile.",
				"  4  View account details, edit profile attributes, or change your password. The page calls " + product.Name +
					" directly with your session token.",
			},
		},
		{
			Title: "Self Sign-Up",
			URL:   sampleURL,
			Lines: []string{
				"Register a new customer account and see " + product.Name + " assign the Traveler role automatically on completion.",
				"",
				"  1  Open " + Cyan(sampleURL) + " and click Sign in.",
				"  2  On the " + product.Name + " page, click Sign up.",
				"  3  Fill in below sample details and click Continue.",
				"",
				"     " + Dim("username") + "  " + Highlight("emma.wilson"),
				"     " + Dim("password") + "  " + Highlight("emma.wilson"),
				"",
				"  4  Fill in the registration form using these sample details and click Continue.",
				"",
				"     " + Dim("email         ") + "  " + Highlight("emma.wilson@example.com"),
				"     " + Dim("first name    ") + "  " + Highlight("Emma"),
				"     " + Dim("last name     ") + "  " + Highlight("Wilson"),
				"     " + Dim("mobile number ") + "  " + Highlight("+15550148812"),
				"",
				"  5  " + product.Name +
					" will create a Customer user and assign the Traveler role. The browser returns to the Wayfinder app signed in as " +
					"the new user.",
			},
		},
		{
			Title: "Account Recovery",
			URL:   sampleURL,
			Lines: []string{
				"Walk through the password recovery flow - John forgets his password and resets it via email.",
				"",
				"  1  Open " + Cyan(sampleURL) + " and click Sign in.",
				"  2  On the " + product.Name + " sign-in page, click Forgot password?",
				"  3  Enter " + Bold("john.doe") + " as the username and submit.",
				"  4  " + product.Name + " sends a recovery email to John's registered address. Open it from the inbox at " +
					Cyan(mailInboxURL) + ".",
				"  5  Click the reset link in the email and set a new password.",
				"  6  Sign in again with the new credentials.",
			},
		},
		{
			Title: "Staff Sign-Up",
			URL:   consoleURL,
			Lines: []string{
				"Invite and onboard two new staff members entirely from the " + product.Name +
					" Console: Sam Rivera (Support) and Maya Patel (DestinationsAdmin). The admin picks the staff role and sends the " +
					"invitation, and the matching role is attached automatically when the invitee completes their profile.",
				"",
				"  1  Sign in to the " + product.Name + " Console at " + Cyan(consoleURL) + " as your admin user.",
				"  2  Navigate to Users and select Add User.",
				"  3  Select Staff as the user type.",
				"  4  Pick Support as the role, enter Sam Rivera's email (" + Bold("sam.rivera@example.com") +
					"), and click Send invitation. An invite link is emailed to Sam.",
				"  5  Open Sam's invitation email from the inbox at " + Cyan(mailInboxURL) +
					" and open the link. The browser opens a Complete Your Profile page.",
				"  6  Fill in the additional attributes and submit. Sam's account is now active with the Support role attached.",
				"  7  Repeat the flow for Maya Patel (email " + Bold("maya.patel@example.com") +
					"), picking DestinationsAdmin as the role.",
			},
		},
	}
}

// agentWalkthroughPanes mirrors the Console's Secured AI Agent tryout
// scenarios (welcome.aiAgentsTryout.scenarios.* in the frontend i18n
// locale) so the CLI and Console walk users through the same journeys.
func agentWalkthroughPanes(sampleURL string) []walkthroughPane {
	return []walkthroughPane{
		{
			Title: "Protect the Agent",
			URL:   sampleURL,
			Lines: []string{
				"See scope-based access control in action - John can use the AI concierge, but Jane cannot.",
				"",
				"  1  Open " + Cyan(sampleURL) + " and sign in as John Doe.",
				"",
				"     " + Green("✓") + " John has access to chat with the Wayfinder chat agent",
				"",
				"     " + Dim("username") + "  " + Highlight("john.doe"),
				"     " + Dim("password") + "  " + Highlight("john.doe"),
				"",
				"  2  Open the chat widget (bottom-right corner) and send any message. The concierge responds — John's token carries the " +
					Bold("agent:access") + " scope.",
				"  3  Sign out and sign in as Jane Smith.",
				"",
				"     " + Red("✗") + " Jane does not have access to chat with the Wayfinder chat agent",
				"",
				"     " + Dim("username") + "  " + Highlight("jane.smith"),
				"     " + Dim("password") + "  " + Highlight("jane.smith"),
				"",
				"  4  Open the chat. Since Jane does not have the Wayfinder Chat User role, the chat agent will not be accessible and " +
					"the widget will show an error message instead.",
			},
		},
		{
			Title: "Browse with Agent",
			URL:   sampleURL,
			Lines: []string{
				"Watch the agent use its own Machine-to-Machine (M2M) token to call read-only tools - no user consent popup required.",
				"",
				"  1  Sign in as John at " + Cyan(sampleURL) + " and open the chat widget.",
				"",
				"     " + Dim("username") + "  " + Highlight("john.doe"),
				"     " + Dim("password") + "  " + Highlight("john.doe"),
				"",
				"  2  Ask a browsing question in the chat:",
				"",
				"     " + Dim(`"What flights are there from Colombo to Singapore?"`),
				"",
				"  3  The agent calls the Wayfinder MCP server with its own M2M token (client_credentials grant). No popup appears.",
				"  4  You can also try asking for flight deals — the agent calls the recommend_bookings tool, which requires the " +
					Bold("booking:recommend") + " scope, granted to the Wayfinder Concierge via its Recommender role.",
				"",
				"     " + Dim(`"Suggest a few flight deals."`),
			},
		},
		{
			Title: "Book on Behalf",
			URL:   sampleURL,
			Lines: []string{
				"Trigger the on-behalf-of consent flow - the agent pauses, asks for your permission, and only proceeds after you approve.",
				"",
				"  1  Sign in as John at " + Cyan(sampleURL) + " and open the chat widget.",
				"",
				"     " + Dim("username") + "  " + Highlight("john.doe"),
				"     " + Dim("password") + "  " + Highlight("john.doe"),
				"",
				"  2  Ask the agent to book something, for example:",
				"",
				"     " + Dim(`"Book flight 2"`),
				"",
				"  3  The agent returns a consent request. A popup opens - sign in as John and select which booking permissions to " +
					"grant (" + Bold("booking:read") + ", " + Bold("booking:create") + ", " + Bold("booking:cancel") + ").",
				"  4  Click Authorize. The agent retries the action using John's context token, and the booking confirmation appears in " +
					"the chat shortly after.",
				"  5  To see the rejection path, repeat the flow but deny " + Bold("booking:create") +
					" in the consent screen. The agent returns a 403.",
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
	adminCreds  *setup.AdminCredentials

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
	height          int

	// body scrolls everything above the input line, so output longer than the
	// terminal stays reachable and the input is never pushed off screen.
	body      viewport.Model
	bodySized bool

	showOnboarding    bool
	onboardingList    list.Model
	onboardingCmdMode bool   // true while the slash-command input overlay is active
	checkPort         int    // non-zero overrides health.DefaultPort for health checks
	upgradeRequested  bool   // set when the /upgrade command is executed
	switchRequested   bool   // set when the /use command is executed
	newVersion        string // non-empty shows a persistent upgrade-available notice below the banner

	nodeWarning string // non-empty shows a persistent Node.js version notice below the banner

	// showAllOnEmpty shows the full command list on an empty prompt for
	// returning users (who skip the first-run onboarding picker). It clears
	// the first time the user types anything.
	showAllOnEmpty bool

	showWalkthrough  bool
	walkthroughPanes []walkthroughPane
	walkthroughTab   int

	// Sample dev-port conflict — active when showPortConflict is true. Holds the launch
	// waiting for the user to approve stopping the processes on the sample's ports.
	showPortConflict bool
	pcHolders        []setup.PortHolder
	pcSampleName     string
	pcOpts           sample.Options
	pcStop           bool // highlighted answer: true stops the holders, false cancels

	// showNotice displays noticeMessage as a standalone dismissible page — used
	// to surface why a /switch or /upgrade attempt didn't go through, since it
	// would otherwise be a plain print lost the instant the REPL redraws.
	showNotice    bool
	noticeMessage string

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
	ucLaunch          func(values map[string]string) (string, sample.Options)

	// Integration guide viewer — active when showGuide is true. Content is the
	// platform's thunderid.dev quickstart, fetched on demand and glamour-rendered.
	showGuide     bool
	guideLoading  bool
	guideLabel    string // display label, e.g. "React"
	guideDocURL   string // human-facing page, opened with 'o'
	guideViewport viewport.Model

	// Log follow mode — active while tailingLogs is true. tailLogOffset is the
	// byte offset up to which the file has already been read and appended.
	// tailLogLines holds the followed output separately from m.messages, capped
	// at maxTailLogLines so a long-running or noisy log stream can't grow the
	// REPL's rendered history — and the memory behind it — without bound.
	tailingLogs   bool
	tailLogPath   string
	tailLogOffset int64
	tailLogLines  []string
}

// maxTailLogLines bounds how many followed log lines are retained for
// rendering at once. Older lines are dropped as new ones arrive.
const maxTailLogLines = 500

// appendTailLogLine records a followed log line, dropping the oldest once the
// buffer exceeds maxTailLogLines.
func (m *ReplModel) appendTailLogLine(line string) {
	m.tailLogLines = append(m.tailLogLines, line)
	if len(m.tailLogLines) > maxTailLogLines {
		m.tailLogLines = m.tailLogLines[len(m.tailLogLines)-maxTailLogLines:]
	}
}

// NewReplModel initializes the REPL model.
func NewReplModel(
	version string, proc *exec.Cmd, installPath string,
	verbose bool, isFirstRun bool, creds *setup.AdminCredentials,
) ReplModel {
	ti := textinput.New()
	ti.Placeholder = "Starting " + product.Name + "..."
	ti.Prompt = "> "
	ti.CharLimit = 256
	ti.SetWidth(defaultInputWidth)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrandBlue))

	var commands []SlashCommand
	for _, u := range Usecases {
		u := u
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
					return func() tea.Msg {
						return sampleTryRequestMsg{
							sampleName: u.SampleName,
							features:   u.SampleFeatures,
						}
					}
				},
			})
		}
	}
	for _, p := range integrate.Platforms {
		p := p
		commands = append(commands, SlashCommand{
			Name:        "/integrate-" + p.Key,
			Description: "Add " + product.Name + " auth to your " + p.Label + " app",
			Section:     "Integrate",
			AsyncAction: func(_ string) tea.Cmd {
				return func() tea.Msg { return integrateFrameworkMsg{framework: p.Key} }
			},
		})
	}

	logCmd := SlashCommand{
		Name:        "/logs",
		Description: "Follow recent server logs (Esc to stop)",
		Section:     "Server",
		Action:      nil, // handled specially in runCommand, like /stop
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
		adminCreds:     creds,
		status:         statusStarting,
		proc:           proc,
		width:          80,
		height:         24,
		showOnboarding: isFirstRun,
		onboardingList: newOnboardingList(80),
		body:           newOutputViewport(),
		guideViewport:  viewport.New(),
	}
}

// newOutputViewport builds the scrollable output region. The default key map binds
// plain letters and space, which belong to the command input, so scrolling is bound to
// keys the input does not use.
func newOutputViewport() viewport.Model {
	vp := viewport.New()
	vp.SoftWrap = true
	vp.KeyMap = viewport.KeyMap{
		PageUp:   key.NewBinding(key.WithKeys("pgup")),
		PageDown: key.NewBinding(key.WithKeys("pgdown")),
		Up:       key.NewBinding(key.WithKeys("shift+up")),
		Down:     key.NewBinding(key.WithKeys("shift+down")),
	}
	return vp
}

// scrollKeys are the keys that move the output region instead of reaching the
// focused input or list.
var scrollKeys = map[string]bool{"pgup": true, "pgdown": true, "shift+up": true, "shift+down": true}

// syncBody re-flows the scrollable region: the viewport takes the height the pinned
// footer leaves, and follows new output unless the user has scrolled up.
func (m *ReplModel) syncBody() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	height := m.height - lipgloss.Height(m.footer())
	if height < minBodyHeight {
		height = minBodyHeight
	}
	follow := !m.bodySized || m.body.AtBottom()
	m.body.SetWidth(m.width)
	m.body.SetHeight(height)
	m.body.SetContent(m.bodyContent())
	m.bodySized = true
	if follow {
		m.body.GotoBottom()
	}
}

// portHolders is a seam so tests can drive the port-conflict overlay without
// depending on what happens to be listening on the developer's machine.
var portHolders = setup.PortHolders

// launchTry starts a sample run, or opens the port-conflict overlay first when
// something already holds one of the sample's dev ports: the run frees those ports
// before it starts, and the holder may be an unrelated app.
func (m *ReplModel) launchTry(sampleName string, opts sample.Options) tea.Cmd {
	if holders := portHolders(sample.ServicePorts(opts.Features)...); len(holders) > 0 {
		// runCommand latches tryingOut before dispatching the try. Release it while the
		// overlay waits: a cancel returns to the prompt, and a stale latch would reject
		// every later command as setup in progress.
		m.tryingOut = false
		m.showPortConflict = true
		m.pcHolders = holders
		m.pcSampleName = sampleName
		m.pcOpts = opts
		m.pcStop = true
		m.input.Blur()
		return nil
	}
	m.tryingOut = true
	m.input.Blur()
	return makeTryCmd(sampleName, m.installPath, m.verbose, opts)
}

// closePortConflict dismisses the port-conflict overlay. refocus hands the prompt back
// to the user, which every path except the approved launch does.
func (m *ReplModel) closePortConflict(refocus bool) {
	m.showPortConflict = false
	m.pcHolders = nil
	if !refocus {
		return
	}
	m.input.Focus()
	if m.status == statusReady {
		m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
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
		pollHealthCmdOn(p, healthPollFastInterval),
		watchProcessCmd(m.proc),
	)
}

// healthPollFastInterval is used while waiting for the product to become
// ready (or to recover), where sub-second responsiveness matters.
const healthPollFastInterval = time.Second

// healthPollSteadyInterval is used once the product is known to be ready.
// Its only remaining purpose is crash detection, which doesn't need
// sub-second granularity and shouldn't spam the access log with checks.
const healthPollSteadyInterval = 5 * time.Second

func pollHealthCmdOn(port int, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(_ time.Time) tea.Msg {
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
		ti.SetWidth(defaultInputWidth)
		if inp.Secret {
			ti.EchoMode = textinput.EchoPassword
		}
		ti.Focus()
		m.ucText = ti
	}
}

// advanceUCStep records value for the current step then moves to the next.
// When all steps are done it clears the config state and starts the run.
func (m *ReplModel) advanceUCStep(value string) tea.Cmd {
	m.ucValues[m.ucInputs[m.ucStep].Key] = value
	m.ucStep++
	if m.ucStep < len(m.ucInputs) {
		m.initUCStep()
		return nil
	}
	m.showUsecaseConfig = false
	return m.launchTry(m.ucLaunch(m.ucValues))
}

// Update implements tea.Model. It delegates to update and then re-flows the
// scrollable region, so every state change is reflected in one place.
func (m ReplModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	updated, ok := next.(ReplModel)
	if !ok {
		return next, cmd
	}
	updated.syncBody()
	return updated, cmd
}

func (m ReplModel) update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:cyclop,funlen
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.onboardingList.SetSize(msg.Width, onboardingListHeight)
		inputWidth := clamp(msg.Width-4, 20, 200)
		m.input.SetWidth(inputWidth)
		m.ucText.SetWidth(inputWidth)
		// Reserve 3 rows for the header/separator/hint chrome renderGuide draws around
		// the viewport. render() gives the guide the whole terminal (no shared body/footer
		// split), so this is the only reservation needed to fit within msg.Height.
		m.guideViewport.SetWidth(msg.Width)
		m.guideViewport.SetHeight(clamp(msg.Height-3, 5, 1000))

	// Bracketed paste arrives as its own message rather than as key presses, so the
	// overlay inputs need it routed explicitly — the command input already receives
	// it from the catch-all at the end of Update. Without this, pasting into the API
	// key prompt does nothing, and the value is too long to retype comfortably.
	case tea.PasteMsg:
		var tiCmd tea.Cmd
		if m.showUsecaseConfig && m.ucStep < len(m.ucInputs) && len(m.ucInputs[m.ucStep].Choices) == 0 {
			m.ucText, tiCmd = m.ucText.Update(msg)
		}
		cmds = append(cmds, tiCmd)

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			m.killThunderID()
			return m, tea.Quit
		}

		// Scrolling the output must work in every mode, and must not reach the
		// focused input or list underneath.
		if scrollKeys[msg.String()] {
			var vpCmd tea.Cmd
			m.body, vpCmd = m.body.Update(msg)
			cmds = append(cmds, vpCmd)
			return m, tea.Batch(cmds...)
		}

		if m.showPortConflict {
			// ── Sample dev-port conflict ───────────────────────────────────────
			switch msg.String() {
			case "up", "down", "left", "right", "tab":
				m.pcStop = !m.pcStop
			case "enter":
				if m.pcStop {
					sampleName, opts := m.pcSampleName, m.pcOpts
					m.closePortConflict(false)
					m.tryingOut = true
					m.input.Blur()
					cmds = append(cmds, makeTryCmd(sampleName, m.installPath, m.verbose, opts))
					break
				}
				held := portsSummary(m.pcHolders)
				m.closePortConflict(true)
				m.messages = append(m.messages,
					Yellow("○")+" Cancelled. The processes on "+held+" were left running.")
			case "esc":
				m.closePortConflict(true)
			}
			return m, tea.Batch(cmds...)
		}

		if m.showNotice {
			// ── Standalone notice page ──────────────────────────────────────────
			switch msg.String() {
			case "esc":
				m.showNotice = false
				m.noticeMessage = ""
				m.input.Focus()
			case "/":
				m.showNotice = false
				m.noticeMessage = ""
				m.input.Focus()
				m.input.SetValue("/")
				m.input.CursorEnd()
				m.updateCompletions()
				return m, tea.Batch(cmds...)
			}
		} else if m.showOnboarding && m.status == statusReady {
			if m.onboardingCmdMode {
				// ── Slash-command overlay ──────────────────────────────────────
				switch msg.String() {
				case "esc":
					m.onboardingCmdMode = false
					m.input.SetValue("")
					m.input.Blur()
					m.showCompletions = false
					m.selectedComp = 0
				case "enter":
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
				case "up":
					if m.showCompletions && m.selectedComp > 0 {
						m.selectedComp--
					}
				case "down":
					if m.showCompletions && m.selectedComp < len(m.completions)-1 {
						m.selectedComp++
					}
				case "tab":
					if m.showCompletions && len(m.completions) > 0 {
						m.input.SetValue(m.completions[m.selectedComp].Name)
						m.input.CursorEnd()
					}
				}
			} else {
				// ── Onboarding list navigation ─────────────────────────────────
				if msg.String() == "enter" {
					if cmd := m.selectOnboarding(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				} else if msg.String() == "/" || msg.String() == "?" {
					m.onboardingCmdMode = true
					m.input.Focus()
					m.input.SetValue("/")
					m.input.CursorEnd()
					m.updateCompletions()
					return m, tea.Batch(cmds...)
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
				switch msg.String() {
				case "enter":
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
				switch msg.String() {
				case "enter":
					val := strings.TrimSpace(m.ucText.Value())
					if val != "" || m.ucInputs[m.ucStep].Optional {
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
			case msg.String() == "left":
				if m.walkthroughTab > 0 {
					m.walkthroughTab--
				}
			case msg.String() == "right":
				if m.walkthroughTab < len(m.walkthroughPanes)-1 {
					m.walkthroughTab++
				}
			case msg.String() == "o":
				if pane := m.walkthroughPanes[m.walkthroughTab]; pane.URL != "" {
					utils.OpenBrowser(pane.URL) //nolint:errcheck
				}
			case msg.String() == "esc":
				m.showWalkthrough = false
				m.input.Focus()
			case msg.String() == "/":
				m.showWalkthrough = false
				m.input.Focus()
				m.input.SetValue("/")
				m.input.CursorEnd()
				m.updateCompletions()
				return m, tea.Batch(cmds...)
			}
		} else if m.showGuide {
			// ── Integration guide viewer ────────────────────────────────────────
			switch msg.String() {
			case "o":
				if m.guideDocURL != "" {
					utils.OpenBrowser(m.guideDocURL) //nolint:errcheck
				}
			case "esc":
				m.showGuide = false
				m.input.Focus()
			case "/":
				m.showGuide = false
				m.input.Focus()
				m.input.SetValue("/")
				m.input.CursorEnd()
				m.updateCompletions()
				return m, tea.Batch(cmds...)
			default:
				var vpCmd tea.Cmd
				m.guideViewport, vpCmd = m.guideViewport.Update(msg)
				cmds = append(cmds, vpCmd)
			}
		} else if m.tailingLogs {
			// ── Log follow mode ──────────────────────────────────────────────────
			if msg.String() == "esc" {
				m.tailingLogs = false
				m.input.Focus()
				m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
			}
		} else {
			// ── Regular REPL ───────────────────────────────────────────────────
			switch msg.String() {
			case "enter":
				if m.status != statusReady {
					break
				}
				val := strings.TrimSpace(m.input.Value())
				if m.showCompletions && len(m.completions) > 0 {
					val = m.completions[m.selectedComp].Name
				}
				if val == "" {
					break
				}
				m.showAllOnEmpty = false
				m.messages = append(m.messages, "> "+val)
				m.input.SetValue("")
				m.showCompletions = false
				m.selectedComp = 0
				if cmd := m.runCommand(val); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case "up":
				if m.showCompletions && m.selectedComp > 0 {
					m.selectedComp--
				}
			case "down":
				if m.showCompletions && m.selectedComp < len(m.completions)-1 {
					m.selectedComp++
				}
			case "tab":
				if m.showCompletions && len(m.completions) > 0 {
					m.input.SetValue(m.completions[m.selectedComp].Name)
					m.input.CursorEnd()
				}
			}
		}

	case sampleTryRequestMsg:
		cmds = append(cmds, m.launchTry(msg.sampleName, sample.Options{
			Features: msg.features, Port: m.effectivePort(),
		}))

	case usecaseConfigRequestMsg:
		m.ucInputs = msg.inputs
		m.ucSampleName = msg.sampleName
		m.ucEnvTarget = msg.envTarget
		m.ucFeatures = msg.features

		// Capture msg fields for the launch closure.
		sampleName, envTarget, features := msg.sampleName, msg.envTarget, msg.features
		port := m.effectivePort()
		m.ucLaunch = func(values map[string]string) (string, sample.Options) {
			return sampleName, sample.Options{
				Config: values, EnvTarget: envTarget, Features: features, Port: port,
			}
		}

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
			cmds = append(cmds, m.launchTry(m.ucLaunch(m.ucValues)))
		} else {
			m.showUsecaseConfig = true
			m.input.Blur()
			m.initUCStep()
		}

	case integrateFrameworkMsg:
		for _, p := range integrate.Platforms {
			if p.Key != msg.framework {
				continue
			}
			// runCommand latches tryingOut for every AsyncAction, but that flag
			// exists to block input during a sample launch, not a guide fetch.
			m.tryingOut = false
			m.guideLoading = true
			m.guideLabel = p.Label
			m.guideDocURL = docs.SiteURL(p.Slug)
			m.input.Blur()
			slug := p.Slug
			cmds = append(cmds, func() tea.Msg {
				markdown, err := docs.FetchGuide(slug)
				return guideLoadedMsg{markdown: markdown, err: err}
			})
			break
		}

	case guideLoadedMsg:
		m.guideLoading = false
		if msg.err != nil {
			m.messages = append(m.messages,
				Red("✗")+" Could not load the "+m.guideLabel+" guide: "+msg.err.Error(),
				Dim("  Open it directly: ")+Cyan(m.guideDocURL),
			)
			m.input.Focus()
			break
		}
		rendered := msg.markdown
		if r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(clamp(m.width-4, 20, 100)),
		); err == nil {
			if out, err := r.Render(msg.markdown); err == nil {
				rendered = out
			}
		}
		m.guideViewport.SetContent(rendered)
		m.guideViewport.GotoTop()
		m.showGuide = true

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
				m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
				m.showAllOnEmpty = true
				if !m.showOnboarding && !m.showNotice {
					// Onboarding and the notice page own input focus until dismissed.
					m.input.Focus()
				}
			}
			// Ready is confirmed — back off to a slower cadence. Sub-second polling
			// is only needed while waiting for the product to come up; once it has,
			// polling every second forever just floods the access log with health
			// checks for no added benefit to crash detection.
			cmds = append(cmds, pollHealthCmdOn(m.effectivePort(), healthPollSteadyInterval))
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
				cmds = append(cmds, pollHealthCmdOn(m.effectivePort(), healthPollFastInterval))
			}
		}

	case logsTailMsg:
		if m.tailingLogs {
			if msg.ok {
				for _, l := range msg.lines {
					m.appendTailLogLine(l)
				}
				m.tailLogOffset = msg.offset
			}
			cmds = append(cmds, tailLogsTickCmd(m.tailLogPath, m.tailLogOffset))
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
		pollInterval := healthPollFastInterval
		if m.status == statusReady {
			pollInterval = healthPollSteadyInterval
		}
		cmds = append(cmds, pollHealthCmdOn(m.effectivePort(), pollInterval))
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
				m.walkthroughPanes = b2cWalkthroughPanes(msg.sampleURL, m.baseURL)
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

	// Both exits below replace the running product, so the sample must not survive:
	// it would keep its ports and point at a base URL that is gone.
	case upgradeMsg:
		sample.StopServices()
		m.upgradeRequested = true
		m.quitting = true
		return m, tea.Quit

	case switchVersionMsg:
		sample.StopServices()
		m.switchRequested = true
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
	if val == "" {
		m.completions = nil
		m.showCompletions = false
		if m.showAllOnEmpty {
			m.completions = m.commands
			m.showCompletions = true
			if m.selectedComp >= len(m.completions) {
				m.selectedComp = 0
			}
		}
		return
	}
	m.showAllOnEmpty = false
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

// tailLogsTickCmd schedules the next follow-mode read. It reads whatever was
// appended to path since offset and reports back via logsTailMsg.
func tailLogsTickCmd(path string, offset int64) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		lines, newOffset, ok := setup.TailFileFollow(path, offset)
		return logsTailMsg{lines: lines, offset: newOffset, ok: ok}
	})
}

// startLogTail prints the last 30 lines of the current log file and switches
// the REPL into follow mode, appending new lines as they are written until
// the user presses Esc.
func (m *ReplModel) startLogTail() tea.Cmd {
	logPath := setup.LatestLogFile(m.installPath)
	lines, offset, ok := setup.TailFileFollow(logPath, 0)
	if !ok {
		m.messages = append(m.messages, Red("✗")+" could not read logs at "+logPath)
		return nil
	}
	const maxLines = 30
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	m.messages = append(m.messages, Dim(fmt.Sprintf("── following %s (Esc to stop) ──", logPath)))
	for _, l := range lines {
		m.appendTailLogLine(l)
	}
	m.tailingLogs = true
	m.tailLogPath = logPath
	m.tailLogOffset = offset
	m.input.Blur()
	m.input.Placeholder = "Following logs — press Esc to stop"
	return tailLogsTickCmd(logPath, offset)
}

func (m *ReplModel) runCommand(val string) tea.Cmd {
	if val == "/logs" {
		return m.startLogTail()
	}
	if val == "/stop" {
		if err := m.stopThunderID(true); err != nil {
			m.messages = append(m.messages, Red("✗")+" Could not stop "+product.Name+": "+err.Error())
			return nil
		}
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

// killThunderID stops what this CLI started and leaves an instance it merely attached to
// running, which is what exiting the session should do.
func (m *ReplModel) killThunderID() {
	// This path cannot fail: it only signals a handle the CLI owns.
	_ = m.stopThunderID(false)
}

// stopThunderID stops the sample and the product. When the CLI started the product it
// holds a handle to the launcher and signals that. When it attached to an instance
// started by an earlier run there is no handle, so an explicit stop falls back to the
// process listening on the session's port — without that fallback an orphaned server
// could never be stopped through the CLI. An undeliverable signal takes that same
// fallback. It only runs when the product answers on the port, so an unrelated listener
// is never terminated.
func (m *ReplModel) stopThunderID(explicit bool) error {
	// Stop the sample first: its backend talks to the product, so stopping the
	// dependant before the product avoids error spew in the sample log.
	sample.StopServices()

	if m.proc != nil && m.proc.Process != nil {
		// SIGTERM lets start.sh's cleanup trap stop ThunderID cleanly before exiting.
		// SIGKILL would bypass the trap and leave the port occupied, causing the next
		// invocation to fail.
		if err := m.proc.Process.Signal(syscall.SIGTERM); err == nil {
			time.Sleep(time.Second)
			return nil
		}
		// The signal was undeliverable (SIGTERM is unsupported on Windows, or the
		// launcher is already gone), so no trap ran for this stop and the server it
		// backgrounded can still be listening. Reporting success here would quit the
		// session leaving that server up, so fall through to the port stop regardless
		// of explicit: we own this process, an implicit exit still has to clean it up.
	} else if !explicit {
		return nil
	}
	port := m.effectivePort()
	if !productOnPort(port) {
		return nil // nothing of ours is listening
	}
	return stopPort(port)
}

// stopTimeout bounds an explicit stop of an attached instance.
const stopTimeout = 15 * time.Second

// productOnPort and stopPort are variables so tests can exercise the attached-stop
// path without signaling whatever holds that port on the developer's machine.
var (
	productOnPort = health.IsReady
	stopPort      = func(port int) error { return setup.FreePort(port, stopTimeout) }
)

// minCompletionRows is the floor for the scrollable completion window, even
// on a very short terminal.
const minCompletionRows = 6

// completionRow is one rendered line of the completion list. itemIndex is the
// index into m.completions for a command row, or -1 for headers/spacers —
// only command rows count toward keeping the selection in view.
type completionRow struct {
	text      string
	itemIndex int
}

// renderCompletions draws the / command list, scrolling the window so the
// selected item stays visible when the list is taller than the terminal.
// available is the number of terminal rows left for this block (including
// its own separators and scroll indicators), as computed by the caller from
// what has already been rendered above it.
func renderCompletions(m ReplModel, available int) string {
	if !m.showCompletions || len(m.completions) == 0 {
		return ""
	}
	const nameW = 24
	rows, selectedRow := buildCompletionRows(m, nameW)
	window, start := completionWindow(rows, selectedRow, available)

	var b strings.Builder
	separator := Dim(strings.Repeat("─", BannerWidth()))
	b.WriteString(separator + "\n")
	if start > 0 {
		b.WriteString("  " + Dim("↑ more above") + "\n")
	}
	for _, r := range window {
		b.WriteString(r.text + "\n")
	}
	if start+len(window) < len(rows) {
		b.WriteString("  " + Dim("↓ more below") + "\n")
	}
	b.WriteString(separator + "\n")
	return b.String()
}

// buildCompletionRows expands m.completions into display rows, inserting section
// headers and spacer rows between sections. Returns the rows and the index within
// them of the currently selected command, for completionWindow to scroll around.
func buildCompletionRows(m ReplModel, nameW int) (rows []completionRow, selectedRow int) {
	lastSection := ""
	for i, c := range m.completions {
		if c.Section != lastSection {
			if i > 0 {
				rows = append(rows, completionRow{itemIndex: -1})
			}
			if c.Section != "" {
				rows = append(rows, completionRow{text: "  " + Dim(c.Section), itemIndex: -1})
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
		rows = append(rows, completionRow{text: "  " + indicator + namePart + "  " + descPart, itemIndex: i})
	}

	for idx, r := range rows {
		if r.itemIndex == m.selectedComp {
			selectedRow = idx
			break
		}
	}
	return rows, selectedRow
}

// completionWindow returns the slice of rows that fits within available terminal
// rows, scrolled so selectedRow stays visible, plus that slice's start index in
// rows — the same start index renderCompletions needs to decide whether to draw
// the "more above" indicator, and a click handler needs to map a clicked row back
// to a command.
func completionWindow(rows []completionRow, selectedRow, available int) (window []completionRow, start int) {
	maxRows := clamp(available-2, minCompletionRows, len(rows))
	if len(rows) > maxRows {
		// Reserve room for the "more above"/"more below" indicator lines.
		maxRows = clamp(maxRows-2, minCompletionRows, len(rows))
	}
	if len(rows) > maxRows {
		start = selectedRow - maxRows/2
		if start < 0 {
			start = 0
		}
		if start+maxRows > len(rows) {
			start = len(rows) - maxRows
		}
	}
	end := start + maxRows
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], start
}

// completionsAvailable returns the terminal rows left for the / command dropdown
// in whichever footer context is currently showing it. Both footer() (to render
// it) and the mouse-click handler (to map a click back to a row) need this same
// value, so it lives here once rather than as inline arithmetic in two places.
func (m ReplModel) completionsAvailable() int {
	if m.onboardingCmdMode {
		// Reserve 3 rows below for the input line and the trailing hint.
		return m.height - 3
	}
	// Reserve 2 rows below for the input/spinner line.
	return m.height - 2
}

// renderNotice draws a standalone page for m.noticeMessage — used instead of
// dropping it into the scrolling message log so it can't be missed or pushed
// off-screen, with its own dismiss hint.
func renderNotice(m ReplModel) string {
	var b strings.Builder
	b.WriteString(Dim(strings.Repeat("─", clamp(m.width-4, 20, 76))) + "\n\n")
	b.WriteString("  " + m.noticeMessage + "\n\n")
	b.WriteString(Dim("  esc dismiss  •  / for commands") + "\n")
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

// renderGuide draws the fetched integration guide (glamour-rendered markdown)
// inside a scrollable viewport, with a header naming the platform and doc URL.
// It replaces the REPL's entire screen (see render()), so its chrome is exactly
// the 3 rows guideViewport's height is sized against: header, separator, hint.
func renderGuide(m ReplModel) string {
	var b strings.Builder
	b.WriteString("  " + Dim(m.guideLabel+" Integration Guide") + "  " + Dim("·") + "  " + Cyan(m.guideDocURL) + "\n")
	b.WriteString("  " + Dim(strings.Repeat("─", clamp(m.width-4, 20, 76))) + "\n")
	b.WriteString(m.guideViewport.View() + "\n")
	b.WriteString(Dim("  ↑/↓ scroll  •  o open in browser  •  esc back  •  / for commands"))
	return b.String()
}

// renderPortConflict names the processes holding the sample's dev ports. Those ports
// are pinned in the sample's generated config, so the only choices, rendered by
// portConflictChoices in the pinned footer, are stopping the holders or canceling.
func renderPortConflict(m ReplModel) string {
	var b strings.Builder
	b.WriteString("  " + Bold("Ports needed by the "+m.pcSampleName+" sample are in use") + "\n\n")
	for _, h := range m.pcHolders {
		b.WriteString("  " + Dim(h.String()) + "\n")
	}
	b.WriteString("\n  " + Dim("Starting the sample frees these ports, which stops whatever holds them.") + "\n")
	return b.String()
}

// portConflictChoices renders the answer the user is about to give.
func portConflictChoices(m ReplModel) string {
	var b strings.Builder
	for i, opt := range []string{"Stop these processes and continue", "Cancel, leave them running"} {
		if (i == 0) == m.pcStop {
			b.WriteString("  " + Cyan("> "+opt) + "\n")
		} else {
			b.WriteString("    " + Dim(opt) + "\n")
		}
	}
	b.WriteString("\n" + Dim("  ↑/↓ select  •  Enter to confirm  •  esc cancel"))
	return b.String()
}

// portsSummary lists the ports held by holders as a comma-separated string.
func portsSummary(holders []setup.PortHolder) string {
	seen := make(map[int]bool, len(holders))
	var ports []string
	for _, h := range holders {
		if seen[h.Port] {
			continue
		}
		seen[h.Port] = true
		ports = append(ports, strconv.Itoa(h.Port))
	}
	return strings.Join(ports, ", ")
}

// credentialsBox renders a bordered box with the generated admin credentials, so
// they stay visible on the alternate screen for the whole session. Returns an empty
// string when no credentials were generated this run.
func (m ReplModel) credentialsBox() string {
	if m.adminCreds == nil {
		return ""
	}
	label := lipgloss.NewStyle().Foreground(lipgloss.Color(colorGrey)).Width(10)
	rows := lipgloss.JoinVertical(lipgloss.Left,
		Bold("Admin credentials"),
		label.Render("Username")+Cyan(m.adminCreds.Username),
		label.Render("Password")+Cyan(m.adminCreds.Password),
		Dim("Sign in to the Console with these credentials."),
	)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBrandBlue)).
		Padding(0, 1).
		Render(rows)
}

// View implements tea.Model.
func (m ReplModel) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	// Mouse reporting is deliberately left off (MouseModeNone, the zero value):
	// enabling it captures every click and drag for the app, which blocks the
	// terminal's own click-drag text selection unless the user holds a
	// modifier key. Losing mouse-wheel scroll and click-to-select is the
	// tradeoff for letting text (admin credentials, integration guides, etc.)
	// be selected and copied normally. Scrolling remains available via
	// PgUp/PgDn and shift+up/down (see scrollHint).
	return v
}

// minBodyHeight keeps a usable slice of output visible on very short terminals.
const minBodyHeight = 3

// render builds the REPL view: a scrolling output region above a pinned footer that
// always keeps the input (or the current spinner) on screen.
func (m ReplModel) render() string {
	if m.quitting {
		return Dim("Stopping " + product.Name + "...\n")
	}
	// The guide viewer takes the whole terminal instead of sharing it with the
	// scrollable body: splitting the height between the two would force the body
	// below its minimum on short terminals, overflowing the screen.
	if m.showGuide {
		return renderGuide(m)
	}
	footer := m.footer()
	if !m.bodySized {
		// No window size yet (first frame): render flat rather than guessing a height.
		return m.bodyContent() + footer
	}
	return m.body.View() + "\n" + footer
}

// bodyPreamble builds the banner, server status and credentials box shown at
// the top of the scrollable region, before whatever mode-specific content
// bodyContent appends after it. It is factored out so a click handler can
// count exactly how many rows precede body content it needs to hit-test
// (e.g. the onboarding list) without duplicating this logic and drifting out
// of sync with what's actually rendered.
func (m ReplModel) bodyPreamble() string {
	var b strings.Builder

	b.WriteString(BannerString(m.version) + "\n\n")

	if m.nodeWarning != "" {
		b.WriteString(fitBox(noteBoxStyle, noteChrome, Yellow("⚠ "+m.nodeWarning)) + "\n\n")
	}

	if m.newVersion != "" && m.status == statusReady {
		b.WriteString(Yellow("✦") + " " + Bold(product.Name+" v"+m.newVersion+" is available") + " — type " + Cyan("/upgrade") +
			" to upgrade\n\n")
	}

	switch m.status {
	case statusStarting:
		b.WriteString(m.spinner.View() + " Starting...\n")
	case statusReady:
		b.WriteString(StatusBoxString(m.baseURL) + "\n")
	case statusStopped:
		b.WriteString(Red("○") + " Stopped\n")
	}
	if box := m.credentialsBox(); box != "" {
		b.WriteString(box + "\n")
	}
	b.WriteString(Dim(strings.Repeat("─", BannerWidth())) + "\n\n")
	return b.String()
}

// bodyContent builds the scrollable region: the banner, server status and everything
// the session has printed so far.
func (m ReplModel) bodyContent() string {
	var b strings.Builder
	b.WriteString(m.bodyPreamble())

	if m.showNotice {
		b.WriteString(renderNotice(m))
		return b.String()
	}

	if m.showOnboarding && m.status == statusReady {
		if !m.onboardingCmdMode {
			b.WriteString(strings.TrimRight(m.onboardingList.View(), "\n"))
		}
		return b.String()
	}

	if m.showPortConflict {
		b.WriteString(renderPortConflict(m))
		return b.String()
	}

	if m.showUsecaseConfig {
		inp := m.ucInputs[m.ucStep]
		b.WriteString("  " + Bold(inp.Label) + "\n\n")
		for _, line := range inp.Instructions {
			b.WriteString("  " + Dim(line) + "\n")
		}
		if len(inp.Instructions) > 0 {
			b.WriteString("\n")
		}
		return b.String()
	}

	for _, msg := range m.messages {
		b.WriteString("  " + msg + "\n")
	}
	if len(m.messages) > 0 {
		b.WriteString("\n")
	}

	for _, l := range m.tailLogLines {
		b.WriteString("  " + Dim(l) + "\n")
	}
	if len(m.tailLogLines) > 0 {
		b.WriteString("\n")
	}

	// The walkthrough carries its own navigation hints, so it scrolls with the output.
	// The guide is rendered in the footer instead, since it scrolls in its own viewport.
	if m.showWalkthrough {
		b.WriteString(renderWalkthrough(m))
	}
	return b.String()
}

// footer builds the pinned region below the scrolling output: the completion list and
// whatever the user types into or waits on.
func (m ReplModel) footer() string {
	var b strings.Builder

	if m.showNotice {
		return ""
	}

	if m.showOnboarding && m.status == statusReady {
		if m.onboardingCmdMode {
			b.WriteString(renderCompletions(m, m.completionsAvailable()))
			b.WriteString(m.input.View())
			b.WriteString("\n\n" + Dim("  esc back to use-case picker"))
		} else {
			b.WriteString("\n" + Dim("  ↑/k up  •  ↓/j down  •  / commands"))
		}
		return b.String()
	}

	if m.showPortConflict {
		return portConflictChoices(m)
	}

	if m.showUsecaseConfig {
		inp := m.ucInputs[m.ucStep]
		if len(inp.Choices) > 0 {
			b.WriteString(m.ucList.View())
			b.WriteString("\n" + Dim("  ↑/↓ select  •  Enter to continue"))
		} else {
			b.WriteString(m.ucText.View() + "\n")
			if inp.Optional {
				b.WriteString("\n" + Dim("  Enter to continue  •  leave empty to skip"))
			} else {
				b.WriteString("\n" + Dim("  Enter to continue"))
			}
		}
		return b.String()
	}

	if m.guideLoading {
		b.WriteString(m.spinner.View() + Dim(" Fetching "+m.guideLabel+" guide…"))
		return b.String()
	}

	if m.showWalkthrough {
		return Dim("  " + scrollHint)
	}

	// While a try-* sample is downloading/starting, its progress replaces the command
	// menu and input line entirely rather than being appended below them.
	if m.tryingOut {
		if m.trySampleStatus != "" {
			b.WriteString(m.spinner.View() + " " + m.trySampleStatus)
		} else {
			b.WriteString(m.spinner.View() + Dim(" Please wait… (Ctrl+C to abort)"))
		}
		return b.String()
	}

	b.WriteString(renderCompletions(m, m.completionsAvailable()))

	if m.status == statusStarting {
		b.WriteString(m.spinner.View() + Dim(" Starting "+product.Name+"…"))
	} else {
		b.WriteString(m.input.View())
		b.WriteString("\n" + Dim("  "+scrollHint))
	}
	return b.String()
}

// scrollHint documents the keys that move the output region.
const scrollHint = "PgUp/PgDn scroll  •  shift+↑/↓ line"

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
// nodeWarning, if non-empty, is shown below the banner for the life of the session.
// port overrides the default health-check port when non-zero.
// notice, if non-empty, is shown as the first message — used to surface why a prior
// /switch or /upgrade attempt in this same process didn't go through, since printing
// it directly would be hidden the instant this REPL's alternate screen takes over.
// Returns upgradeRequested=true when the user ran /upgrade, switchRequested=true when /use.
func RunREPL(
	version string, proc *exec.Cmd, installPath string,
	verbose, isFirstRun bool, newVersion, nodeWarning string, port int, creds *setup.AdminCredentials,
	notice string,
) (upgradeRequested, switchRequested bool, err error) {
	m := NewReplModel(version, proc, installPath, verbose, isFirstRun, creds)
	m.newVersion = newVersion
	m.nodeWarning = nodeWarning
	if notice != "" {
		m.showNotice = true
		m.noticeMessage = notice
		m.input.Blur()
	}
	if port > 0 {
		m.checkPort = port
	}
	p := tea.NewProgram(m)
	finalModel, runErr := p.Run()
	if rm, ok := finalModel.(ReplModel); ok {
		return rm.upgradeRequested, rm.switchRequested, runErr
	}
	return false, false, runErr
}
