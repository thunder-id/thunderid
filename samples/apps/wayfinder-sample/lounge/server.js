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

// Skyline Lounge — a standalone OpenID4VP *verifier* kiosk.
//
// It is a separate relying party from Wayfinder (the issuer): a guest presents
// the Wayfinder Sky Pass they hold in their wallet, the lounge verifies it
// through ThunderID's OpenID4VP API, and grants/denies entry based on the verified
// tier. No OAuth client registration is needed — the request object is signed by
// ThunderID's verifier key (x509_san_dns), which the wallet trusts.

import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));

// A dedicated var (not PORT) so the kiosk never collides with the Wayfinder
// backend when both run under `npm run dev` and PORT happens to be set.
const PORT = Number(process.env.LOUNGE_PORT || 8795);
// Reuse the Wayfinder frontend's tunnel URL when a lounge-specific one isn't set,
// so one env var (VITE_THUNDER_BASE_URL) points the whole sample at ThunderID.
const THUNDER_BASE_URL =
  process.env.THUNDER_BASE_URL || process.env.VITE_THUNDER_BASE_URL || "";
const DEFINITION_ID = process.env.SKYPASS_DEFINITION_ID || "wayfinder-skypass";
const ALLOWED_TIERS = (process.env.ALLOWED_TIERS || "Gold,Platinum")
  .split(",")
  .map((s) => s.trim())
  .filter(Boolean);

// Local-dev TLS bypass: ThunderID ships a self-signed cert on localhost. Only
// relax verification when the configured base URL points at localhost.
if (/^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?(\/|$)/.test(THUNDER_BASE_URL)) {
  process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";
  console.warn("[lounge] Local ThunderID detected — TLS verification disabled. Dev only.");
}

function decodeJwtPayload(token) {
  try {
    return JSON.parse(Buffer.from(String(token).split(".")[1], "base64url").toString());
  } catch {
    return null;
  }
}

async function thunderFetch(path, options = {}) {
  if (!THUNDER_BASE_URL) {
    const error = new Error("THUNDER_BASE_URL is not configured.");
    error.statusCode = 500;
    throw error;
  }
  const abort = new AbortController();
  const timeout = setTimeout(() => abort.abort(), 10_000);
  try {
    const response = await fetch(`${THUNDER_BASE_URL}${path}`, {
      ...options,
      headers: { ...options.headers },
      signal: abort.signal,
    });
    const text = await response.text();
    let body;
    try {
      body = text ? JSON.parse(text) : {};
    } catch {
      body = { raw: text };
    }
    if (!response.ok) {
      const error = new Error(body.error || body.message || `ThunderID request failed (${response.status})`);
      error.statusCode = response.status;
      throw error;
    }
    return body;
  } finally {
    clearTimeout(timeout);
  }
}

function sendJson(response, statusCode, body) {
  response.writeHead(statusCode, { "Content-Type": "application/json" });
  response.end(JSON.stringify(body));
}

const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, `http://localhost:${PORT}`);

    if (request.method === "GET" && (url.pathname === "/" || url.pathname === "/index.html")) {
      const html = await readFile(join(__dirname, "public", "index.html"));
      response.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      response.end(html);
      return;
    }

    // Start a verification: ask ThunderID for a presentation request the wallet can scan.
    if (request.method === "POST" && url.pathname === "/api/verify/start") {
      const initiated = await thunderFetch("/openid4vp/initiate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ definition_id: DEFINITION_ID }),
      });
      return sendJson(response, 200, {
        txnId: initiated.txn_id,
        walletUrl: initiated.wallet_url,
      });
    }

    // Poll status; on completion surface the verified claims + the access decision.
    if (request.method === "GET" && url.pathname.startsWith("/api/verify/status/")) {
      const txnId = decodeURIComponent(url.pathname.replace("/api/verify/status/", ""));
      const result = await thunderFetch(`/openid4vp/status/${encodeURIComponent(txnId)}`, { method: "GET" });
      const claims = result.result_token
        ? decodeJwtPayload(result.result_token)?.verified_claims || null
        : null;

      let name = null;
      let tier = null;
      let accessGranted = null;
      if (result.status === "COMPLETED" && claims) {
        tier = claims.tier || null;
        name =
          claims.full_name ||
          [claims.given_name, claims.family_name].filter(Boolean).join(" ") ||
          null;
        accessGranted = tier != null && ALLOWED_TIERS.includes(tier);
      }

      return sendJson(response, 200, {
        status: result.status,
        name,
        tier,
        memberId: claims?.member_id || null,
        accessGranted,
        allowedTiers: ALLOWED_TIERS,
        error: result.error || null,
      });
    }

    sendJson(response, 404, { error: "Not found" });
  } catch (error) {
    sendJson(response, error.statusCode || 500, { error: error.message });
  }
});

server.listen(PORT, () => {
  console.log(`Skyline Lounge verifier listening on http://localhost:${PORT}`);
  console.log(`  ThunderID  : ${THUNDER_BASE_URL || "(set THUNDER_BASE_URL)"}`);
  console.log(`  Definition : ${DEFINITION_ID}`);
  console.log(`  Lounge tiers: ${ALLOWED_TIERS.join(", ")}`);
});
