/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const SCHEME_PATTERN = /^([a-zA-Z][a-zA-Z0-9+.-]*):/;
const HTTP_AUTHORITY_PATTERN = /^https?:\/\/([^/?#]*)/i;
// A non-empty host (bracketed IPv6 or any run of non-space, non-colon chars)
// followed by an optional, purely numeric port — mirroring Go's url.Parse.
const HTTP_HOST_PORT_PATTERN = /^(\[[^\]]+\]|[^\s:]+)(:\d*)?$/;

/**
 * Checks whether a value is valid for use as a resource logo URI.
 *
 * Mirrors the backend `IsValidLogoURI` allowlist so the client never commits a
 * value the server will reject:
 * - `http`/`https` are accepted only with a non-empty host and, if present, a
 *   numeric port (so malformed authorities like `http://host:abc` are rejected
 *   client-side instead of failing on the server).
 * - `data`, `blob` and `emoji` schemes are always accepted.
 * - A value with no scheme is accepted only when it is an absolute path
 *   (starts with `/`).
 * - Every other scheme (e.g. `javascript`, `file`, `ftp`) is rejected.
 *
 * @param value - The candidate logo URI.
 * @returns `true` when the value is an acceptable logo URI.
 *
 * @public
 */
export default function isValidLogoUri(value: string): boolean {
  if (!value) return false;

  const schemeMatch = SCHEME_PATTERN.exec(value);

  if (!schemeMatch) {
    return value.startsWith('/');
  }

  switch (schemeMatch[1].toLowerCase()) {
    case 'http':
    case 'https': {
      const authorityMatch = HTTP_AUTHORITY_PATTERN.exec(value);
      if (!authorityMatch) return false;
      const authority: string = authorityMatch[1].split('@').pop() ?? '';
      return HTTP_HOST_PORT_PATTERN.test(authority);
    }
    case 'data':
    case 'blob':
    case 'emoji':
      return true;
    default:
      return false;
  }
}
