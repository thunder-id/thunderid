/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
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

import { createServer } from "node:net";
import { parse } from "./parser.js";
import { add } from "./store.js";

const CRLF = "\r\n";

function reply(socket, code, message) {
  socket.write(`${code} ${message}${CRLF}`);
}

function createSession() {
  return { from: "", rcptTo: [], dataMode: false, dataBuffer: "", authStep: null };
}

function handleLine(socket, session, line) {
  if (session.dataMode) {
    if (line === ".") {
      session.dataMode = false;
      // Undo SMTP dot-stuffing: a line that began with "." was sent with an
      // extra leading dot, so strip one leading dot per line.
      const raw = session.dataBuffer.replace(/^\./gm, "");
      try {
        const parsed = parse(raw);
        if (!parsed.to.length) parsed.to = [...session.rcptTo];
        add(parsed);
        console.log(`[smtp] message received from=${session.from} to=${session.rcptTo.join(",")}`);
        reply(socket, 250, "OK: message queued");
      } catch (err) {
        console.error("[smtp] parse error:", err.message);
        reply(socket, 554, "Transaction failed");
      }
      session.dataBuffer = "";
      session.from = "";
      session.rcptTo = [];
    } else {
      session.dataBuffer += line + "\n";
    }
    return;
  }

  const upper = line.toUpperCase();

  if (upper.startsWith("EHLO") || upper.startsWith("HELO")) {
    const domain = line.slice(5).trim() || "localhost";
    socket.write(`250-mail.local Hello ${domain}${CRLF}`);
    socket.write(`250-SIZE 10485760${CRLF}`);
    socket.write(`250-AUTH PLAIN LOGIN${CRLF}`);
    socket.write(`250 OK${CRLF}`);
    return;
  }

  if (upper.startsWith("AUTH PLAIN")) {
    // Accept any credentials — this is a capture-only dev server.
    reply(socket, 235, "Authentication successful");
    return;
  }

  if (upper.startsWith("AUTH LOGIN")) {
    session.authStep = "username";
    reply(socket, 334, "VXNlcm5hbWU6"); // base64 "Username:"
    return;
  }

  if (session.authStep === "username") {
    session.authStep = "password";
    reply(socket, 334, "UGFzc3dvcmQ6"); // base64 "Password:"
    return;
  }

  if (session.authStep === "password") {
    session.authStep = null;
    reply(socket, 235, "Authentication successful");
    return;
  }

  if (upper.startsWith("MAIL FROM:")) {
    const match = line.match(/MAIL FROM:\s*<?([^>]*)>?/i);
    session.from = match ? match[1].trim() : "";
    reply(socket, 250, "OK");
    return;
  }

  if (upper.startsWith("RCPT TO:")) {
    const match = line.match(/RCPT TO:\s*<?([^>]*)>?/i);
    const addr = match ? match[1].trim() : "";
    if (addr) session.rcptTo.push(addr);
    reply(socket, 250, "OK");
    return;
  }

  if (upper === "DATA") {
    session.dataMode = true;
    session.dataBuffer = "";
    reply(socket, 354, "Start mail input; end with <CRLF>.<CRLF>");
    return;
  }

  if (upper === "RSET") {
    Object.assign(session, createSession());
    reply(socket, 250, "OK");
    return;
  }

  if (upper === "NOOP") {
    reply(socket, 250, "OK");
    return;
  }

  if (upper === "QUIT") {
    reply(socket, 221, "Bye");
    socket.end();
    return;
  }

  reply(socket, 500, "Unrecognized command");
}

export function startSmtp(host, port) {
  const server = createServer(socket => {
    const session = createSession();
    let buffer = "";

    reply(socket, 220, "mail.local ESMTP ready");

    socket.setEncoding("utf8");

    socket.on("data", data => {
      buffer += data;
      let newline;
      while ((newline = buffer.indexOf("\n")) !== -1) {
        const line = buffer.slice(0, newline).replace(/\r$/, "");
        buffer = buffer.slice(newline + 1);
        try {
          handleLine(socket, session, line);
        } catch (err) {
          console.error("[smtp] handler error:", err.message);
          reply(socket, 500, "Internal error");
        }
      }
    });

    socket.on("error", err => {
      if (err.code !== "ECONNRESET") {
        console.error("[smtp] socket error:", err.message);
      }
    });
  });

  server.listen(port, host, () => {
    console.log(`[smtp] SMTP server listening on ${host}:${port}`);
  });

  server.on("error", err => {
    console.error("[smtp] server error:", err.message);
    process.exit(1);
  });

  return server;
}
