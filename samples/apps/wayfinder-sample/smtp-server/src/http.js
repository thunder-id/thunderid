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

import { createServer } from "node:http";
import { readFileSync, existsSync, statSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join, dirname, extname, resolve, relative, isAbsolute } from "node:path";
import { list, get, setRead, remove, clear, unreadCount } from "./store.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const DIST_DIR = resolve(join(__dirname, "../dist"));

const MIME = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript",
  ".mjs": "text/javascript",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".json": "application/json",
};

function serveFile(res, filePath) {
  const ext = extname(filePath).toLowerCase();
  const contentType = MIME[ext] || "application/octet-stream";
  const isText = contentType.startsWith("text/") || contentType.includes("javascript") ||
    contentType.includes("json") || contentType.includes("svg");
  const content = readFileSync(filePath, isText ? "utf8" : null);
  res.writeHead(200, { "Content-Type": contentType });
  res.end(content);
}

function serveIndex(res) {
  const indexPath = join(DIST_DIR, "index.html");
  if (!existsSync(indexPath)) {
    res.writeHead(503, { "Content-Type": "text/html; charset=utf-8" });
    res.end(
      "<html><body style=\"font-family:sans-serif;padding:40px\">" +
      "<p>UI not built. Run <code>npm run build</code> in the smtp-server directory.</p>" +
      "</body></html>"
    );
    return;
  }
  serveFile(res, indexPath);
}

function send(res, status, body, contentType = "application/json") {
  const payload = contentType === "application/json" ? JSON.stringify(body) : body;
  res.writeHead(status, { "Content-Type": contentType });
  res.end(payload);
}

function idFromPath(pathname, prefix) {
  try {
    return decodeURIComponent(pathname.slice(prefix.length));
  } catch {
    return null;
  }
}

export function startHttp(host, port) {
  const server = createServer((req, res) => {
    const { method, url } = req;
    let pathname;
    try {
      // Fixed base — we only need the path, so the Host header is irrelevant
      // and a missing/malformed one can't crash the parse.
      pathname = new URL(url, "http://localhost").pathname;
    } catch {
      return send(res, 400, { error: "Bad request" });
    }

    // JSON API
    if (pathname === "/health" && method === "GET") {
      return send(res, 200, { status: "ok" });
    }

    if (method === "GET" && pathname === "/api/messages") {
      return send(res, 200, { unread: unreadCount(), messages: list() });
    }

    if (method === "GET" && pathname.startsWith("/api/messages/") &&
        !pathname.endsWith("/read") && !pathname.endsWith("/unread")) {
      const id = idFromPath(pathname, "/api/messages/");
      if (id === null) return send(res, 400, { error: "Bad request" });
      const msg = get(id);
      if (!msg) return send(res, 404, { error: "Not found" });
      setRead(id, true);
      return send(res, 200, { message: msg });
    }

    if (method === "POST" && pathname.startsWith("/api/messages/") && pathname.endsWith("/read")) {
      const rawId = idFromPath(pathname, "/api/messages/");
      if (rawId === null) return send(res, 400, { error: "Bad request" });
      const msg = setRead(rawId.replace(/\/read$/, ""), true);
      if (!msg) return send(res, 404, { error: "Not found" });
      return send(res, 200, { message: msg });
    }

    if (method === "POST" && pathname.startsWith("/api/messages/") && pathname.endsWith("/unread")) {
      const rawId = idFromPath(pathname, "/api/messages/");
      if (rawId === null) return send(res, 400, { error: "Bad request" });
      const msg = setRead(rawId.replace(/\/unread$/, ""), false);
      if (!msg) return send(res, 404, { error: "Not found" });
      return send(res, 200, { message: msg });
    }

    if (method === "DELETE" && pathname.startsWith("/api/messages/") && pathname !== "/api/messages/") {
      const id = idFromPath(pathname, "/api/messages/");
      if (id === null) return send(res, 400, { error: "Bad request" });
      const ok = remove(id);
      if (!ok) return send(res, 404, { error: "Not found" });
      return send(res, 200, { deleted: id });
    }

    if (method === "DELETE" && (pathname === "/api/messages" || pathname === "/api/messages/")) {
      const count = clear();
      return send(res, 200, { cleared: count });
    }

    // Static files from Vite build output
    if (method === "GET") {
      if (pathname === "/" || pathname === "/index.html") {
        return serveIndex(res);
      }

      const filePath = resolve(join(DIST_DIR, pathname));
      // Guard against path traversal (cross-platform: avoid string-prefix checks
      // that break on Windows backslashes).
      const rel = relative(DIST_DIR, filePath);
      if (rel.startsWith("..") || isAbsolute(rel)) {
        return send(res, 403, { error: "Forbidden" });
      }

      if (existsSync(filePath) && !statSync(filePath).isDirectory()) {
        try {
          return serveFile(res, filePath);
        } catch {
          return send(res, 500, { error: "Read error" });
        }
      }

      // SPA fallback
      return serveIndex(res);
    }

    return send(res, 404, { error: "Not found" });
  });

  server.listen(port, host, () => {
    console.log(`[http] Inbox UI listening on http://${host}:${port}`);
  });

  server.on("error", err => {
    console.error("[http] server error:", err.message);
    process.exit(1);
  });

  return server;
}
