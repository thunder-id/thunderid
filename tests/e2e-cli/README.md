# CLI E2E Tests

End-to-end tests for the ThunderID CLI, written with Playwright Test. They drive the same
entry point users run, the npx wrapper at `tools/npx/bin/thunderid.js`, spawned under a
real pseudo terminal.

The release, setup, server, REPL, and sample are real; only the system-browser launch is
stubbed. A run reaches the public release manifest, downloads a real distribution, runs
`setup.sh`, boots the server, drives the REPL, and finally launches the Wayfinder sample
and signs into it in a browser. The point is to catch breakage in the wrapper, the
download and extract path, setup, port negotiation, the REPL and the sample launch as a
single working chain, which no amount of unit testing covers.

## What runs where

The suite is one codebase driven in four ways. Which product and which CLI it points at is what
distinguishes them.

| Lane            | Workflow                       | CLI from          | Product from          | Answers                                        |
| --------------- | ------------------------------- | ----------------- | --------------------- | ---------------------------------------------- |
| 1. CLI gate     | `pr-builder.yml`                | this checkout     | published             | Does a CLI change break the CLI?               |
| 2. Product gate | `pr-builder.yml`                | this checkout     | **this pull request** | Does a product or sample change break the CLI? |
| 3. Release gate | `release-builder.yml`           | release commit    | the release candidate | Are we about to publish a broken pairing?      |
| 4. Nightly      | `nightly-build-validation.yml`  | **published npm** | published             | Do the shipped artifacts still work together?  |

Lane 2 exists because the CLI drives `setup.sh`, `start.sh`, `deployment.yaml`, the bootstrap
resources and the sample bundle, none of which is CLI code. Lane 1 cannot see a change to any of
them: it installs the published release, so it would pass while the change ships broken.

Lane 4 is the only one that installs the CLI from npm, so it is the only one testing what users
actually get. It catches what no pull request can, because nothing in the repository changed.

Lanes 1 and 2 both live inside the PR builder as `cli-lint-unit`/`cli-e2e` and `cli-compatibility`.
Lane 2 reuses the distribution that run already built, instead of building a second one. Every job
in `pr-builder.yml`, this pair included, sits behind the `trigger-pr-builder` label (a runner-
saturation workaround, see the workflow's own header), so unlike before, a CLI-only change no
longer gets lint/unit feedback ahead of that label going on.

Lane 4 lives inside Nightly Build Validation rather than its own workflow, as its own `cli-compatibility`
job: one nightly schedule, and one failure report covering both platform packaging and the
published CLI/product pairing, instead of two separate issues and Chat pings for what is really
one "is tonight's build healthy" question.

Two environment variables select the mode:

| Variable                   | Effect                                                                                    |
| -------------------------- | ----------------------------------------------------------------------------------------- |
| `E2E_PRODUCT_DIST`         | Serve this directory of release zips (the shape of `target/dist`) and point the CLI at it |
| `E2E_CLI_SOURCE=published` | Run `npx thunderid@<tag>` instead of building from source; `E2E_CLI_TAG` picks the tag    |

Neither is a test-only backdoor. `E2E_PRODUCT_DIST` works through `--product-version`, the shipped
flag an operator uses for a mirror or to pin a release.

## Running

From this directory:

```bash
pnpm install                                 # first time only
pnpm test                                    # everything, about 2 minutes warm
pnpm run test:no-sample                      # everything except the browser spec
pnpm exec playwright test --project=commands # just the fast, offline command surface
pnpm exec playwright test --project=repl     # the REPL, reusing the shared install
pnpm exec playwright test --ui               # interactive
```

Naming a project also runs the projects it depends on, so `--project=repl` still installs
first. Only `commands` stands alone.

To run against a distribution built from this checkout rather than the published release:

```bash
./build.sh build_frontend && ./build.sh build_backend "$(go env GOOS)" "$(go env GOARCH)"
make package_samples OS=$(go env GOOS) ARCH=$(go env GOARCH)
E2E_PRODUCT_DIST=$PWD/target/dist pnpm --dir tests/e2e-cli test
```

To run against the published CLI, as a user would get it:

```bash
E2E_CLI_SOURCE=published E2E_CLI_TAG=latest pnpm test
```

Requirements:

- network access to `thunderid.dev` and the npm registry
- Go, to build the CLI binary the wrapper resolves (done by `global-setup.ts`)
- Chromium, for the sample spec: `pnpm exec playwright install chromium`
- **ports free**: 8090 for ThunderID, 8899 for the local release server when
  `E2E_PRODUCT_DIST` is set, plus 5173, 8787, 8788, 2525 and 8795 for the Wayfinder sample. The happy path binds 8090 and the conflict specs hold it deliberately.
  If you have a ThunderID instance running locally, stop it first; the specs fail fast
  with a clear message rather than fighting it.

This suite is separate from `tests/e2e` and does not share its configuration. That one
starts a backend on port 8090 through Playwright's `webServer` before the first test; here
the CLI is the thing that installs and starts ThunderID, and it needs that port free.

Windows is not covered yet: the wrapper resolves a `.exe`, setup runs `setup.ps1` instead
of `setup.sh`, and node-pty uses conpty rather than a spawn helper.

## Why a pseudo terminal

The CLI branches on `ui.Interactive()`, a terminal check on stdin, to decide whether to
prompt at all. Over a pipe it takes the scripted fallbacks instead, so a piped run would
assert code paths users never see. `internal/cli/root.go` is explicit about it:

```go
if !ui.Interactive() {
    // A scripted or CI run cannot answer the prompt, so take the port as before
```

The REPL is also a Bubble Tea program rendering ANSI frames, which only a pty produces.

## How it stays isolated

Each spec gets a `Workspace`: a temp `HOME`, which relocates the CLI's
`~/.thunderid/state.json`, and a temp working directory, which relocates the
`./thunderid/vX` install. Every `THUNDERID_*` variable is stripped from the child
environment, so a developer's own shell configuration cannot change what is under test.

Per-spec runtime state stays inside those temp directories, with one exception the specs
cannot avoid: the server binds a real port, and the CLI deliberately leaves it running in
the background after the REPL exits. Teardown frees the port explicitly. Separately,
`global-setup.ts` builds the host binary into the wrapper's own `dist/` directory once per
run, ahead of any spec.

## Layout

Specs are grouped by the part of the CLI they exercise, and each folder is a Playwright
project. The dependency graph in `playwright.config.ts` is what orders them.

```text
tests/
├── commands/       no install, no network. The argument surface that returns early
├── install/        the only spec that downloads. Populates the shared install
├── repl/           warm-starts against the shared install, one session per spec
├── restart/        running again over an existing install
├── port-conflict/  the three answers to a contested port
└── sample/         `try`, ending in a real browser sign-in. Runs last, destructively
```

| Project         | Depends on       | Covers                                                                                                      |
| --------------- | ---------------- | ----------------------------------------------------------------------------------------------------------- |
| `commands`      | nothing          | `--help`, `-h`, `integrate`, `try` with no install                                                          |
| `install`       | nothing          | Manifest, download, extract, `setup.sh`, start, `state.json`, files on disk                                 |
| `repl`          | `install`        | The onboarding picker and command overlay, `/status`, `/logs`, `/open-console`, `/integrate-react`, `/stop` |
| `restart`       | `install`        | Warm start, `--setup`                                                                                       |
| `port-conflict` | `install`        | Alternate port, abort, reclaim                                                                              |
| `sample`        | all of the above | Unknown sample name, and `try wayfinder` through a browser login                                            |

Supporting code:

| Path                         | Role                                                                         |
| ---------------------------- | ---------------------------------------------------------------------------- |
| `global-setup.ts`            | Builds the host binary into the wrapper's `dist/`, clears the shared install |
| `fixtures/shared-install.ts` | Creates and reads back the one install every project reuses                  |
| `fixtures/cli.ts`            | The `repl` fixture: a session already at the prompt, torn down after         |
| `utils/cli-session.ts`       | The pty session: spawn, expect, send keys, exit codes                        |
| `utils/cli-process.ts`       | A non-pty runner, for commands that background work and exit                 |
| `utils/workspace.ts`         | Temp `HOME` and working directory, plus the state file                       |
| `utils/ports.ts`             | Port checks, readiness probes, the port holder                               |
| `utils/sample-ports.ts`      | The sample's five fixed ports, and stopping what it leaves behind            |
| `utils/browser-stub.ts`      | Records what the CLI asks the system browser to open                         |
| `utils/release-source.ts`    | Serves a locally built distribution with a generated manifest                |
| `pages/wayfinder.page.ts`    | A minimal page object for the sample and the gate                            |

### Why one shared install

A distribution is ~85MB extracted. `install` performs it once into a fixed path under
`test-results/`, and everything downstream declares a dependency on that project, which is
both the ordering mechanism and the reason those specs warm-start in seconds. Only two
specs download: `install`, and the first-run case in `port-conflict`, which needs an
install that setup has not yet pinned to a port.

Each spec still starts its own CLI session rather than sharing a live process, so one
wedged REPL cannot take the rest of the file down with it.

## Deliberately not covered

- **`/try-consumer` from inside the REPL.** The command-line `try wayfinder` covers the
  same sample runner; the REPL path additionally drives the walkthrough overlay.
- **`upgrade` performing a real upgrade.** It depends on two releases existing and on
  which one is latest at the time, so it cannot assert anything stable on a PR.
- **The offline fallback** (`root.go` warns and starts the installed version when the
  release server is unreachable). Reaching it needs the releases URL to be injectable;
  today it is a constant in `internal/product`.

## Notes for adding specs

- **Everything is serial, with one worker.** Every spec binds the default port or
  deliberately holds it. That is a property of what is under test, not a tuning knob.
- **A distribution is ~85MB extracted**, and the port-conflict prompt comes _after_ the
  download, so a naive spec per branch pays for its own copy. Both slow files use
  `test.describe.serial` over a shared workspace. Keep it that way unless a case genuinely
  needs a clean machine.
- **Drive slash commands with `runSlash`, never raw keystrokes.** The REPL only routes `/`
  into command mode once it reports ready. Sent earlier it reaches the onboarding picker,
  where the following Enter launches a use case, which hangs the spec on a sample
  download. `runSlash` waits for the overlay before typing.
- **Assertions match the whole buffer**, so a phrase from an earlier frame satisfies a
  later `expect`. Call `reset()` before an interaction that reuses wording already on
  screen; `runSlash` does it for you.
- **`/stop` exits the program**, closing the pty. Use `exitRepl()`, not a raw Ctrl+C. When
  a step is not asserting `/stop` itself, tear down with `shutdown(port)` instead: it exits
  and frees the port without depending on TUI state.
- **Match on short, distinctive phrases.** Output is compared with escape sequences
  stripped and whitespace collapsed, so a phrase broken across a line still matches, but
  the TUI redraws constantly and long strings are fragile.
- The onboarding picker reappears until a use case is actually selected: `onboardingDone`
  is only recorded by `ui.selectOnboarding`. Do not assert it is absent on a second run.

## The sample spec

`tests/sample/try-wayfinder.spec.ts` is the slowest and least ordinary spec here.

- It runs `try wayfinder`, not `try consumer`. `consumer` is the REPL slash command; the
  only sample name the CLI accepts is `wayfinder`, and `unknown-usecase.spec.ts` pins that
  error message down.
- It uses `runCli`, not a pty session. `try` exits as soon as the services are up, and
  closing a pty at that point SIGHUPs the ThunderID server it just backgrounded. A real
  terminal is owned by the shell, so the pty is what would be lying here, not the pipe.
- It runs last and is destructive: `try` rewrites `<install>/config/resources/**` and
  restarts the server, so it must not overlap the projects using the shared install.
- Teardown is entirely ours. `try` installs no signal handler and never calls
  `StopServices`, so it leaves npm and all five services running.
- A cold run pays for an `npm install` across the sample's five-package workspace, so
  budget minutes rather than seconds on a clean machine.

## node-pty, and why this is a separate workspace

`pnpm-workspace.yaml` here marks this directory as its own pnpm workspace root, so it is
installed independently of the repository's workspace and has its own lockfile.

That is not tidiness, it is a requirement. node-pty is a native addon that ships prebuilds
for macOS and Windows only, so on Linux it compiles with node-gyp. As a member of the root
workspace it would force every job that runs `pnpm install --frozen-lockfile` to build a
native addon, and the jobs running in minimal containers have no toolchain: they fail with
`gyp ERR! stack Error: not found: make`. Keeping the suite out of the workspace confines
that requirement to the one job that needs it.

Two consequences worth knowing:

- Versions are pinned in `package.json` rather than taken from the root catalog. When the
  catalog moves `@playwright/test` or `@types/node`, move them here too.
- Running the CLI e2e suite on Linux needs `make` and a C++ toolchain. GitHub's
  `ubuntu-latest` image has both.

`postinstall` runs `scripts/fix-node-pty-permissions.js`. Where node-pty does ship a
prebuilt binary, extraction into the pnpm store drops the mode bits on its `spawn-helper`.
The addon then loads fine but every spawn fails with the opaque `posix_spawnp failed`. The
script restores the executable bit and is a no-op once correct.
