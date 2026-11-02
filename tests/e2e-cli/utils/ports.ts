// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Port helpers
 *
 * The CLI negotiates over a real TCP port and backgrounds the server it starts, so tests have to
 * inspect and release ports directly rather than relying on a process exiting.
 */

import { execFile, spawn, ChildProcess } from "child_process";
import https from "https";
import http from "http";
import net from "net";
import { promisify } from "util";
import { sleep } from "./cli-session";
import { Timeouts } from "../constants/timeouts";

const execFileAsync = promisify(execFile);

/** The port a fresh setup binds, matching health.DefaultPort. */
export const DEFAULT_PORT = 8090;

export async function isPortInUse(port: number): Promise<boolean> {
  return new Promise(resolve => {
    const server = net.createServer();
    server.once("error", () => resolve(true));
    server.once("listening", () => server.close(() => resolve(false)));
    server.listen(port, "127.0.0.1");
  });
}

/**
 * Whether something is accepting connections on port.
 *
 * This is a different question from isPortInUse, which asks whether the port can be bound. A
 * server listening on `localhost` may resolve to ::1 only, leaving a 127.0.0.1 bind free: the
 * Wayfinder sample's API does exactly that. Use this to assert a service is up, and isPortInUse
 * to assert a port is available.
 */
export async function isPortAccepting(port: number): Promise<boolean> {
  const results = await Promise.all([connects("127.0.0.1", port), connects("::1", port)]);
  return results.some(Boolean);
}

function connects(host: string, port: number): Promise<boolean> {
  return new Promise(resolve => {
    const socket = net.connect({ host, port });
    const settle = (result: boolean) => {
      socket.destroy();
      resolve(result);
    };
    socket.setTimeout(2000);
    socket.once("connect", () => settle(true));
    socket.once("timeout", () => settle(false));
    socket.once("error", () => settle(false));
  });
}

/**
 * Throws when something already holds port. The happy-path specs bind the default port, so an
 * unrelated instance on it would fail them in a way that looks like a product bug rather than a
 * dirty machine.
 */
export async function requirePortFree(port: number): Promise<void> {
  if (await isPortInUse(port)) {
    throw new Error(`Port ${port} is already in use; stop whatever is listening on it and re-run`);
  }
}

/**
 * Stops whatever is listening on port.
 *
 * The CLI starts the server in the background on purpose: it outlives the REPL, so exiting the
 * CLI is not enough to release the port for the next spec.
 */
export async function freePort(port: number): Promise<void> {
  // SIGTERM first. The server owns SQLite files and start.sh installs a cleanup trap, so killing
  // it outright leaves a dirty write-ahead log for the next spec that starts on this install.
  if (!(await signalHolders(port, "SIGTERM"))) return;
  if (await waitForPortFree(port, 10_000)) return;

  await signalHolders(port, "SIGKILL");
  if (!(await waitForPortFree(port, 10_000))) {
    throw new Error(`Port ${port} is still held after SIGTERM and SIGKILL`);
  }
}

/** Signals every process listening on port. Returns false when nothing was listening. */
async function signalHolders(port: number, signal: "SIGTERM" | "SIGKILL"): Promise<boolean> {
  let stdout = "";
  try {
    ({ stdout } = await execFileAsync("lsof", ["-ti", `tcp:${port}`]));
  } catch {
    return false; // nothing listening
  }
  let signalled = false;
  for (const field of stdout.split(/\s+/).filter(Boolean)) {
    const pid = Number(field);
    if (!Number.isInteger(pid)) continue;
    try {
      process.kill(pid, signal);
      signalled = true;
    } catch {
      // already gone
    }
  }
  return signalled;
}

/**
 * Frees port only when one was actually bound. A spec that failed before the server started has
 * no port to release, and expressing that as a guard here keeps it out of the test body.
 */
export async function releasePortIfBound(port: number): Promise<void> {
  if (!port) return;
  await freePort(port);
}

/** Polls until nothing holds port, or timeout expires. */
export async function waitForPortFree(port: number, timeout = 15_000): Promise<boolean> {
  const deadline = Date.now() + timeout;
  for (;;) {
    if (!(await isPortInUse(port))) return true;
    if (Date.now() >= deadline) return false;
    await sleep(200);
  }
}

/**
 * Whether the server answers its readiness endpoint on port, over either scheme, mirroring
 * internal/services/health. The distribution serves TLS with a self-signed certificate, so
 * verification is skipped exactly as the CLI skips it.
 */
export async function isReady(port: number): Promise<boolean> {
  for (const scheme of ["https", "http"] as const) {
    if (await probe(scheme, port)) return true;
  }
  return false;
}

function probe(scheme: "https" | "http", port: number): Promise<boolean> {
  return new Promise(resolve => {
    const client = scheme === "https" ? https : http;
    const req = client.request(
      {
        host: "127.0.0.1",
        port,
        path: "/health/readiness",
        method: "GET",
        timeout: 5000,
        ...(scheme === "https" ? { rejectUnauthorized: false } : {}),
      },
      res => {
        res.resume();
        resolve(res.statusCode === 200);
      }
    );
    req.on("error", () => resolve(false));
    req.on("timeout", () => {
      req.destroy();
      resolve(false);
    });
    req.end();
  });
}

/** Polls until the server answers on port, or timeout expires. */
export async function waitForReady(port: number, timeout: number = Timeouts.SHORT): Promise<boolean> {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (await isReady(port)) return true;
    await sleep(500);
  }
  return false;
}

/**
 * A process that squats on a port until it is stopped, used to provoke the CLI's port-conflict
 * handling.
 *
 * The holder has to be a separate process: the CLI's "kill the process on this port" branch looks
 * the owner up with lsof and terminates it, which would take the test runner itself down if the
 * runner held the port.
 */
export class PortHolder {
  private constructor(
    private readonly proc: ChildProcess,
    private exited = false
  ) {}

  static async start(port: number): Promise<PortHolder> {
    // Accept and immediately drop connections, so a probe sees an open port that never answers a
    // readiness check, which is what an unrelated process squatting on the port looks like.
    const script = `
      const net = require("net");
      const server = net.createServer(socket => socket.destroy());
      server.listen(${port}, "127.0.0.1", () => console.log("listening"));
    `;
    const proc = spawn(process.execPath, ["-e", script], { stdio: ["ignore", "pipe", "inherit"] });
    const holder = new PortHolder(proc);
    proc.on("exit", () => {
      holder.exited = true;
    });

    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`Port holder did not bind port ${port}`)), 10_000);
      proc.stdout?.once("data", () => {
        clearTimeout(timer);
        resolve();
      });
      proc.once("exit", () => {
        clearTimeout(timer);
        reject(new Error(`Port holder exited before binding port ${port}`));
      });
    });
    return holder;
  }

  /** Whether the holding process is still running. */
  get alive(): boolean {
    return !this.exited;
  }

  async stop(): Promise<void> {
    if (this.exited) return;
    this.proc.kill("SIGKILL");
    await new Promise<void>(resolve => this.proc.once("exit", () => resolve()));
  }
}
