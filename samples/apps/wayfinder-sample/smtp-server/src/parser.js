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

function decodeQuotedPrintable(str) {
  return str
    .replace(/=\r?\n/g, "")
    .replace(/=([0-9A-Fa-f]{2})/g, (_, hex) => String.fromCharCode(parseInt(hex, 16)));
}

function decodeBase64(str) {
  return Buffer.from(str.replace(/\s+/g, ""), "base64").toString("utf8");
}

function decodeEncodedWord(str) {
  // RFC 2047 encoded words: =?charset?encoding?text?=
  return str.replace(/=\?([^?]+)\?([BbQq])\?([^?]*)\?=/g, (_, charset, enc, text) => {
    const buf = enc.toUpperCase() === "B"
      ? Buffer.from(text.replace(/\s+/g, ""), "base64")
      : Buffer.from(text.replace(/_/g, " ").replace(/=([0-9A-Fa-f]{2})/g, (__, h) => String.fromCharCode(parseInt(h, 16))), "binary");
    return buf.toString("utf8");
  });
}

function parseHeaders(headerBlock) {
  const headers = {};
  const lines = headerBlock.replace(/\r\n([ \t])/g, " $1").split(/\r?\n/);

  for (const line of lines) {
    const colon = line.indexOf(":");
    if (colon === -1) continue;
    const key = line.slice(0, colon).trim().toLowerCase();
    const value = line.slice(colon + 1).trim();
    if (key in headers) {
      if (!Array.isArray(headers[key])) headers[key] = [headers[key]];
      headers[key].push(value);
    } else {
      headers[key] = value;
    }
  }

  return headers;
}

function extractAddress(raw) {
  if (!raw) return "";
  const decoded = decodeEncodedWord(raw);
  const match = decoded.match(/<([^>]+)>/);
  return match ? match[1] : decoded.trim();
}

function extractAddressDisplay(raw) {
  if (!raw) return "";
  return decodeEncodedWord(raw).trim();
}

function getContentType(headers) {
  const ct = (Array.isArray(headers["content-type"]) ? headers["content-type"][0] : headers["content-type"]) || "";
  const parts = ct.split(";").map(s => s.trim());
  const type = parts[0].toLowerCase();
  const boundary = (parts.find(p => p.startsWith("boundary=")) || "")
    .replace(/^boundary=/, "").replace(/^"|"$/g, "");
  const charset = (parts.find(p => p.toLowerCase().startsWith("charset=")) || "utf-8")
    .replace(/^charset=/i, "").replace(/^"|"$/g, "").toLowerCase();
  return { type, boundary, charset };
}

function getTransferEncoding(headers) {
  const enc = (Array.isArray(headers["content-transfer-encoding"])
    ? headers["content-transfer-encoding"][0]
    : headers["content-transfer-encoding"]) || "";
  return enc.trim().toLowerCase();
}

function decodePart(body, encoding) {
  if (encoding === "base64") return decodeBase64(body);
  if (encoding === "quoted-printable") return decodeQuotedPrintable(body);
  return body;
}

function parseParts(rawBody, boundary) {
  const delimiter = `--${boundary}`;
  const parts = rawBody.split(new RegExp(`\\r?\\n?${escapeRegex(delimiter)}(?:--)?\\r?\\n?`));
  return parts.slice(1).filter(p => p.trim() && !p.trim().startsWith("--"));
}

function escapeRegex(str) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function extractTextFromPart(part) {
  const sep = part.match(/\r?\n\r?\n/);
  if (!sep) return { type: "", text: "" };

  const headerBlock = part.slice(0, sep.index);
  const body = part.slice(sep.index + sep[0].length);
  const headers = parseHeaders(headerBlock);
  const { type, boundary } = getContentType(headers);
  const enc = getTransferEncoding(headers);

  if (type.startsWith("multipart/") && boundary) {
    return extractMultipart(body, boundary, type);
  }

  return { type, text: decodePart(body, enc) };
}

function extractMultipart(rawBody, boundary, multipartType) {
  const parts = parseParts(rawBody, boundary);
  let html = "";
  let text = "";

  for (const part of parts) {
    const { type, text: content, html: nestedHtml } = extractTextFromPart(part);
    if (type === "text/html") html = content;
    else if (type === "text/plain") text = content;
    else if (type.startsWith("multipart/")) {
      // nested multipart — preserve both alternatives it resolved
      if (!text && content) text = content;
      if (!html && nestedHtml) html = nestedHtml;
    }
  }

  return { type: multipartType, text, html };
}

export function parse(raw) {
  const sep = raw.match(/\r?\n\r?\n/);
  if (!sep) {
    return { from: "", to: [], subject: "(no subject)", date: "", html: "", text: raw, headers: {}, raw };
  }

  const headerBlock = raw.slice(0, sep.index);
  const body = raw.slice(sep.index + sep[0].length);
  const headers = parseHeaders(headerBlock);

  const fromRaw = Array.isArray(headers["from"]) ? headers["from"][0] : (headers["from"] || "");
  const toRaw = Array.isArray(headers["to"]) ? headers["to"].join(", ") : (headers["to"] || "");
  const subject = decodeEncodedWord(Array.isArray(headers["subject"]) ? headers["subject"][0] : (headers["subject"] || "(no subject)"));
  const date = Array.isArray(headers["date"]) ? headers["date"][0] : (headers["date"] || "");

  const from = extractAddressDisplay(fromRaw);
  const to = toRaw.split(",").map(a => extractAddress(a.trim())).filter(Boolean);

  const { type, boundary } = getContentType(headers);
  const enc = getTransferEncoding(headers);

  let html = "";
  let text = "";

  if (type.startsWith("multipart/") && boundary) {
    const result = extractMultipart(body, boundary, type);
    html = result.html || "";
    text = result.text || "";
  } else if (type === "text/html") {
    html = decodePart(body, enc);
  } else {
    text = decodePart(body, enc);
  }

  return { from, to, subject, date, html, text, headers, raw };
}
