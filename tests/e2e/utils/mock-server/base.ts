// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { createServer, IncomingMessage, Server, ServerResponse } from "http";

/**
 * Mock HTTP Server base
 *
 * The Node HTTP server lifecycle shared by every mock server in this directory (SMS, Google
 * OIDC, GitHub OAuth): start/stop/getURL/isRunning and the readBody/sendJSON request helpers.
 * Each mock keeps its own request routing and protocol logic - only this generic plumbing,
 * identical across all three, lives here.
 */
export abstract class MockHttpServer {
  protected server: Server | null = null;
  protected readonly port: number;

  /** Log prefix distinguishing this mock's console output, e.g. "[Mock SMS Server]". */
  protected abstract readonly logPrefix: string;

  constructor(port: number) {
    this.port = port;
  }

  /** Route a request. Errors thrown/rejected here are caught and answered with a 500. */
  protected abstract handleRequest(req: IncomingMessage, res: ServerResponse): Promise<void> | void;

  /** Write the 500 response body for an unhandled `handleRequest` error. */
  protected onInternalError(res: ServerResponse): void {
    this.sendJSON(res, 500, { error: "internal_server_error" });
  }

  /** Extra startup logging beyond the generic "Started on ..." line. */
  protected onStarted(): void {}

  protected readBody(req: IncomingMessage): Promise<string> {
    return new Promise((resolve, reject) => {
      let data = "";
      req.setEncoding("utf8");
      req.on("data", (chunk: string) => {
        data += chunk;
      });
      req.on("end", () => resolve(data));
      req.on("error", reject);
    });
  }

  protected sendJSON(res: ServerResponse, status: number, body: unknown): void {
    res.writeHead(status, { "Content-Type": "application/json" });
    res.end(JSON.stringify(body));
  }

  async start(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.server = createServer((req, res) => {
        Promise.resolve(this.handleRequest(req, res)).catch(err => {
          console.error(`${this.logPrefix} Unhandled error:`, err);
          this.onInternalError(res);
        });
      });

      this.server.on("error", (error: Error) => {
        console.error(`${this.logPrefix} Failed to start:`, error);
        reject(error);
      });

      this.server.listen(this.port, () => {
        console.log(`${this.logPrefix} Started on http://localhost:${this.port}`);
        this.onStarted();
        resolve();
      });
    });
  }

  async stop(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (!this.server) {
        resolve();
        return;
      }

      const server = this.server;
      server.close(err => {
        this.server = null;
        if (err) {
          console.error(`${this.logPrefix} Error stopping server:`, err);
          reject(err);
        } else {
          console.log(`${this.logPrefix} Stopped`);
          resolve();
        }
      });
      // Force-close any lingering keep-alive connections, or close()'s callback above would
      // never fire while one stays open.
      server.closeAllConnections();
    });
  }

  getURL(): string {
    return `http://localhost:${this.port}`;
  }

  isRunning(): boolean {
    return this.server !== null && this.server.listening;
  }
}
