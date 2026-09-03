// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * CLI Session
 *
 * Drives one `npx thunderid ...` invocation attached to a pseudo terminal.
 *
 * A pty is required rather than a pipe. The CLI branches on `ui.Interactive()`, a terminal check
 * on stdin, to decide whether to prompt at all: over a pipe it takes the scripted fallbacks (see
 * `resolvePort` in internal/cli/root.go), so a piped run would assert code paths users never see.
 * The REPL is also a bubbletea program that renders ANSI frames, which only a pty produces.
 */

import * as pty from "node-pty";
import path from "path";
import { Workspace } from "./workspace";
import { freePort } from "./ports";
import { localManifestURL } from "./release-source";
import { Timeouts } from "../constants/timeouts";

/** Key sequences written to the pty to drive the TUI. */
export const Keys = {
  ENTER: "\r",
  ESC: "\x1b",
  DOWN: "\x1b[B",
  CTRL_C: "\x03",
} as const;

/**
 * Admin credentials fed through the environment. setup.sh requires at least one digit and one
 * special character, and supplying both values up front is what makes the CLI skip its
 * interactive credential prompt (see `collectAdminCredentials` in internal/cli/root.go).
 */
export const ADMIN_USERNAME = "admin";
export const ADMIN_PASSWORD = "E2eAdmin#123";

/**
 * Escape sequences the TUI emits: CSI (color, cursor movement, erase), OSC strings, and the short
 * two-byte sequences bubbletea uses for mode switches.
 */
const ANSI = /\x1b\[[0-9;?]*[\x20-\x2f]*[\x40-\x7e]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[()][A-Za-z0-9]|\x1b[=>78]/g;

const WHITESPACE = /\s+/g;

/** How many times, and how long, to try opening the command overlay before giving up. */
const OVERLAY_ATTEMPTS = 6;
const OVERLAY_WAIT = 5_000;

/** Normalizes a phrase the same way session output is normalized, so the two are comparable. */
function normalize(text: string): string {
  return text.replace(ANSI, "").replace(WHITESPACE, " ");
}

export interface SessionOptions {
  /** Pre-supply the admin credentials, suppressing the interactive credential prompt. */
  preconfigured?: boolean;
  /** Prepended to PATH, so a stub can shadow a real executable the CLI shells out to. */
  pathPrefix?: string;
}

export class CliSession {
  private readonly proc: pty.IPty;
  private raw = "";
  private code: number | null = null;
  private readonly exited: Promise<number>;

  constructor(
    private readonly workspace: Workspace,
    args: string[] = [],
    options: SessionOptions = {}
  ) {
    const env: Record<string, string> = {};
    for (const [key, value] of Object.entries(process.env)) {
      // HOME is redirected at the workspace and every THUNDERID_* variable is dropped, so local
      // shell configuration cannot change what is under test.
      if (value === undefined || key === "HOME" || key.startsWith("THUNDERID_")) continue;
      env[key] = value;
    }
    env.HOME = workspace.home;
    env.TERM = "xterm-256color";

    // Point the CLI at a locally served distribution when one is configured. This is the same
    // knob an operator uses for a mirror, so the suite exercises the shipped feature rather than
    // a test-only backdoor.
    const manifest = localManifestURL();
    if (manifest) {
      env.THUNDERID_PRODUCT_VERSION = manifest;
    }
    if (options.preconfigured) {
      env.THUNDERID_ADMIN_USERNAME = ADMIN_USERNAME;
      env.THUNDERID_ADMIN_PASSWORD = ADMIN_PASSWORD;
    }
    if (options.pathPrefix) {
      env.PATH = `${options.pathPrefix}${path.delimiter}${env.PATH ?? ""}`;
    }

    const launch = cliCommand();
    this.proc = pty.spawn(launch.command, [...launch.args, ...args], {
      name: "xterm-256color",
      // A wide window keeps the TUI from wrapping the phrases assertions match on.
      cols: 160,
      rows: 50,
      cwd: workspace.dir,
      env,
    });

    this.proc.onData(chunk => {
      this.raw += chunk;
    });
    this.exited = new Promise<number>(resolve => {
      this.proc.onExit(({ exitCode }) => {
        this.code = exitCode;
        resolve(exitCode);
      });
    });
  }

  /** Everything received so far, stripped of escape sequences and whitespace-normalized. */
  get output(): string {
    return normalize(this.raw);
  }

  /** True once the process has exited. */
  get hasExited(): boolean {
    return this.code !== null;
  }

  /**
   * Drops the output captured so far.
   *
   * Assertions match everything the session has printed, so a phrase from an earlier frame would
   * satisfy a later wait. Resetting marks a fresh starting point before a new interaction.
   */
  reset(): void {
    this.raw = "";
  }

  /** Resolves true once phrase appears, or false when timeout expires. Never throws. */
  async waitFor(phrase: string, timeout: number = Timeouts.SHORT): Promise<boolean> {
    const needle = normalize(phrase);
    const deadline = Date.now() + timeout;
    for (;;) {
      if (this.output.includes(needle)) return true;
      if (this.hasExited) {
        // One last look: the phrase may have arrived in the same breath as the exit.
        return this.output.includes(needle);
      }
      if (Date.now() >= deadline) return false;
      await sleep(100);
    }
  }

  /** Waits for phrase, throwing with the recent output when it does not arrive. */
  async expect(phrase: string, timeout: number = Timeouts.SHORT): Promise<void> {
    if (await this.waitFor(phrase, timeout)) return;
    throw new Error(`Timed out after ${timeout}ms waiting for ${JSON.stringify(phrase)}\n\n${this.tail()}`);
  }

  /** Writes keystrokes, pausing so the TUI can process and redraw before the next assertion. */
  async send(...keys: string[]): Promise<void> {
    for (const key of keys) {
      this.proc.write(key);
      await sleep(150);
    }
  }

  /**
   * Opens the REPL's slash-command overlay, leaving it open.
   *
   * The REPL only routes "/" into command mode once its own status has flipped to ready, and that
   * lags the server answering its readiness endpoint. A "/" sent before then is swallowed by the
   * onboarding picker, where a following Enter would launch a use case instead of running a
   * command, so this retries rather than assuming the first keystroke lands.
   */
  async openCommandOverlay(): Promise<void> {
    for (let attempt = 1; attempt <= OVERLAY_ATTEMPTS; attempt++) {
      // Esc first on a retry: it clears any half-open state so a second "/" cannot land in an
      // input that already holds one.
      if (attempt > 1) await this.send(Keys.ESC);

      this.reset();
      await this.send("/");
      // Every overlay lists the built-in commands, so /status's description proves it is up.
      if (await this.waitFor("Show server status", OVERLAY_WAIT)) return;
    }
    throw new Error(`Command overlay did not open after ${OVERLAY_ATTEMPTS} attempts\n\n${this.tail()}`);
  }

  /** Runs a slash command in the REPL. */
  async runSlash(command: string): Promise<void> {
    await this.openCommandOverlay();
    this.reset();
    await this.send(command.replace(/^\//, ""), Keys.ENTER);
  }

  /** The port from the CLI's "started on port N" line. */
  startedPort(): number {
    const match = /started on port (\d+)/.exec(this.output);
    if (!match) throw new Error(`No start line in output\n\n${this.tail()}`);
    return Number(match[1]);
  }

  /** Waits for the process to exit and returns its exit code. */
  async waitForExit(timeout: number = Timeouts.SHORT): Promise<number> {
    const result = await Promise.race([this.exited, sleep(timeout).then(() => "timeout" as const)]);
    if (result === "timeout") {
      throw new Error(`Process did not exit within ${timeout}ms\n\n${this.tail()}`);
    }
    return result;
  }

  /**
   * Leaves the REPL and returns the exit code.
   *
   * Ctrl+C is best effort: /stop already ends the program, which closes the pty, so writing then
   * would fail rather than tell us anything about the CLI.
   */
  async exitRepl(timeout: number = Timeouts.SHORT): Promise<number> {
    if (!this.hasExited) this.proc.write(Keys.CTRL_C);
    return this.waitForExit(timeout);
  }

  /**
   * Ends the session and makes sure port is free for whatever runs next. The CLI backgrounds the
   * server deliberately, so leaving the REPL is not on its own enough.
   */
  async shutdown(port: number): Promise<void> {
    await this.exitRepl();
    await freePort(port);
  }

  /** Terminates the session, tolerating one that already exited. Registered as test cleanup. */
  async dispose(): Promise<void> {
    if (this.hasExited) return;
    this.proc.write(Keys.CTRL_C);
    const raced = await Promise.race([this.exited, sleep(10_000).then(() => "timeout" as const)]);
    if (raced === "timeout") {
      this.proc.kill("SIGKILL");
      await this.exited;
    }
  }

  /** The last non-empty lines of output, for failure messages. */
  tail(lines = 60): string {
    const kept = this.raw
      .replace(ANSI, "")
      .replace(/\r/g, "\n")
      .split("\n")
      .map(line => line.trimEnd())
      .filter(line => line.trim() !== "");
    return `Last output:\n${kept.slice(-lines).join("\n")}`;
  }
}

/**
 * The wrapper under test. Tests drive the npx entry point rather than the Go binary so the
 * wrapper's own platform resolution and exit-code forwarding are covered too.
 */
export function npxEntry(): string {
  return path.resolve(__dirname, "..", "..", "..", "tools", "npx", "bin", "thunderid.js");
}

/**
 * Whether to drive the CLI built from this checkout, or the one published to npm.
 *
 * The published mode is the only one that tests what users actually get: the wrapper as
 * packaged, and the binaries the release workflow built. Everything else builds from source, so
 * a PR is tested against its own code.
 */
export function publishedTag(): string | null {
  if (process.env.E2E_CLI_SOURCE?.trim() !== "published") return null;
  return process.env.E2E_CLI_TAG?.trim() || "latest";
}

/** The command that launches the CLI, honouring the source mode. */
export function cliCommand(): { command: string; args: string[] } {
  const tag = publishedTag();
  if (tag) {
    // --yes so a cold runner does not stop on the install prompt.
    return { command: "npx", args: ["--yes", `thunderid@${tag}`] };
  }
  return { command: "node", args: [npxEntry()] };
}

export function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}
