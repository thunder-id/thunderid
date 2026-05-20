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

import {ErrorCode} from './error-codes';

/**
 * Structured error type for the ThunderID Nuxt SDK.
 *
 * Every error thrown by SDK internals should be an `ThunderIDError` so
 * that callers can branch on `err.code` instead of matching strings.
 *
 * @example
 * ```ts
 * try {
 *   const session = await requireServerSession(event);
 * } catch (err) {
 *   if (err instanceof ThunderIDError && err.code === ErrorCode.SessionMissing) {
 *     throw createError({ statusCode: 401 });
 *   }
 *   throw err;
 * }
 * ```
 */
export class ThunderIDError extends Error {
  readonly code: ErrorCode;

  readonly statusCode?: number;

  override readonly cause?: unknown;

  readonly context?: Record<string, unknown>;

  constructor(
    message: string,
    code: ErrorCode,
    opts?: {
      cause?: unknown;
      context?: Record<string, unknown>;
      statusCode?: number;
    },
  ) {
    super(message);
    this.name = 'ThunderIDError';
    this.code = code;
    this.statusCode = opts?.statusCode;
    this.cause = opts?.cause;
    this.context = opts?.context;

    // Maintain correct prototype chain in transpiled environments
    Object.setPrototypeOf(this, new.target.prototype);
  }
}
