/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {ThunderIDError} from '../errors/thunderid-error';
import {ErrorCode} from '../errors/error-codes';

/**
 * Validate a `returnTo` / redirect URL supplied by the client.
 *
 * Rules (defence-in-depth against open-redirect attacks):
 * 1. Must be a non-empty string.
 * 2. Must start with `/` (relative path only — no protocol, no host).
 * 3. Must NOT start with `//` (protocol-relative URL — resolves as absolute).
 * 4. Must NOT contain a `\` after the leading `/` (browser normalises `\` to `/`).
 * 5. Must NOT contain a `%2F` or `%5C` in the first two chars after `/`
 *    (encoded slashes/backslashes that bypass rule 3/4 after URL decoding).
 *
 * Returns the validated URL as-is on success, or throws `ThunderIDError`
 * with `ErrorCode.OpenRedirectBlocked` on failure.
 *
 * @example
 * ```ts
 * const safe = validateReturnUrl('/dashboard');          // '/dashboard'
 * validateReturnUrl('//evil.com');                        // throws
 * validateReturnUrl('https://evil.com');                  // throws
 * validateReturnUrl('/\\evil.com');                       // throws
 * ```
 */
export function validateReturnUrl(url: unknown): string {
  if (typeof url !== 'string' || url.trim() === '') {
    throw new ThunderIDError('returnTo must be a non-empty string.', ErrorCode.OpenRedirectBlocked, {statusCode: 400});
  }

  const trimmed: string = url.trim();

  // Must start with exactly one slash
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) {
    throw new ThunderIDError(
      `Open redirect blocked: returnTo "${trimmed}" must be a relative path starting with a single "/".`,
      ErrorCode.OpenRedirectBlocked,
      {statusCode: 400},
    );
  }

  // Reject backslash after the leading slash
  if (trimmed.length > 1 && trimmed[1] === '\\') {
    throw new ThunderIDError(
      `Open redirect blocked: returnTo "${trimmed}" contains a backslash.`,
      ErrorCode.OpenRedirectBlocked,
      {statusCode: 400},
    );
  }

  // Reject encoded protocol-relative or absolute indicators in the first segment
  const decoded: string = decodeURIComponent(trimmed.slice(1, 5).toLowerCase());
  if (decoded.startsWith('/') || decoded.startsWith('\\')) {
    throw new ThunderIDError(
      `Open redirect blocked: returnTo "${trimmed}" contains an encoded redirect sequence.`,
      ErrorCode.OpenRedirectBlocked,
      {statusCode: 400},
    );
  }

  return trimmed;
}

/**
 * Safe variant of `validateReturnUrl` that returns a fallback instead of throwing.
 *
 * @example
 * ```ts
 * const url = safeReturnUrl(query.returnTo, '/dashboard');
 * ```
 */
export function safeReturnUrl(url: unknown, fallback: string = '/'): string {
  try {
    return validateReturnUrl(url);
  } catch {
    return fallback;
  }
}
