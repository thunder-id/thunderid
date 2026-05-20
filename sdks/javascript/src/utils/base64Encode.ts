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

import * as jose from 'jose';

/**
 * Encodes a string to standard base64 using `jose` (already a package dependency).
 *
 * `jose.base64url.encode` is environment-agnostic (browser, Node.js, Deno, Bun,
 * edge/service-worker runtimes). It produces base64url output, which is then
 * converted to standard base64 by restoring the `+`/`/` characters and adding
 * `=` padding.
 *
 * @param value - The UTF-8 string to encode.
 * @returns The standard base64-encoded string (with `+`, `/`, and `=` padding).
 *
 * @example
 * ```typescript
 * base64Encode('clientId:clientSecret'); // "Y2xpZW50SWQ6Y2xpZW50U2VjcmV0"
 * ```
 */
const base64Encode = (value: string): string => {
  const b64url: string = jose.base64url.encode(new TextEncoder().encode(value));
  const rem: number = b64url.length % 4;
  const padded: string = rem === 0 ? b64url : b64url + '='.repeat(4 - rem);

  return padded.replace(/-/g, '+').replace(/_/g, '/');
};

export default base64Encode;
