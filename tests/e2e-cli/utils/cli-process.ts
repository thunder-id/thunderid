// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Runs the CLI without a pseudo terminal, for commands that neither prompt nor render a TUI.
 *
 * `try` is the case this exists for. It prints a summary and exits, leaving the ThunderID server
 * and the sample's services running in the background. Under a pty the CLI is the only process on
 * the terminal, so its exit closes the pty and the resulting SIGHUP takes the server down with
 * it. In a real terminal the shell owns the pty and the server survives, so the pty is what would
 * be lying here, not the plain pipe.
 *
 * Everything interactive still belongs in CliSession: over a pipe the CLI takes its scripted
 * fallbacks (see ui.Interactive), which is only correct when there is nothing to answer.
 */

import { spawn } from "child_process";
import { ADMIN_PASSWORD, ADMIN_USERNAME, cliCommand } from "./cli-session";
import { localManifestURL } from "./release-source";
import { Workspace } from "./workspace";

export interface CliResult {
  /** Combined stdout and stderr, with escape sequences stripped. */
  output: string;
  exitCode: number;
}

const ANSI = /\x1b\[[0-9;?]*[\x20-\x2f]*[\x40-\x7e]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g;

/**
 * Runs the CLI to completion.
 *
 * The child is left in this process's group on purpose: anything it backgrounds has to outlive
 * it, which is the whole point of the commands that use this.
 */
export function runCli(workspace: Workspace, args: string[], timeout: number): Promise<CliResult> {
  return new Promise((resolve, reject) => {
    const env: Record<string, string> = {};
    for (const [key, value] of Object.entries(process.env)) {
      if (value === undefined || key === "HOME" || key.startsWith("THUNDERID_")) continue;
      env[key] = value;
    }
    env.HOME = workspace.home;
    env.THUNDERID_ADMIN_USERNAME = ADMIN_USERNAME;
    env.THUNDERID_ADMIN_PASSWORD = ADMIN_PASSWORD;
    const manifest = localManifestURL();
    if (manifest) {
      env.THUNDERID_PRODUCT_VERSION = manifest;
    }

    const launch = cliCommand();
    const child = spawn(launch.command, [...launch.args, ...args], { cwd: workspace.dir, env });

    let output = "";
    child.stdout.on("data", chunk => (output += chunk));
    child.stderr.on("data", chunk => (output += chunk));

    const timer = setTimeout(() => {
      child.kill("SIGKILL");
      reject(new Error(`\`thunderid ${args.join(" ")}\` did not finish within ${timeout}ms\n\n${tail(output)}`));
    }, timeout);

    child.once("error", error => {
      clearTimeout(timer);
      reject(error);
    });
    child.once("close", code => {
      clearTimeout(timer);
      resolve({ output: output.replace(ANSI, ""), exitCode: code ?? -1 });
    });
  });
}

function tail(output: string, lines = 40): string {
  const kept = output
    .replace(ANSI, "")
    .split("\n")
    .filter(line => line.trim() !== "");
  return kept.slice(-lines).join("\n");
}
