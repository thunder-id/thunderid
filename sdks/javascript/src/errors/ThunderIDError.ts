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

/**
 * Base class for all ThunderID errors. This class extends the native Error class
 * and adds support for error codes and proper stack traces. Each error is prefixed
 * with a lightning emoji and the SDK name for easy identification.
 *
 * @example
 * ```typescript
 * // Create a new error with a message and code
 * throw new ThunderIDError(
 *   "Invalid authentication response",
 *   "AUTH_ERROR"
 * );
 *
 * // Or with a specific SDK name
 * throw new ThunderIDError(
 *   "Invalid authentication response",
 *   "AUTH_ERROR",
 *   "@thunderid/react"
 * );
 *
 * // The error message will be formatted as:
 * // ⚡ ThunderID - @thunderid/react: Invalid authentication response
 * //
 * // (code="AUTH_ERROR")
 */
export default class ThunderIDError extends Error {
  public readonly code: string;

  public readonly origin: string;

  private static resolveOrigin(origin: string): string {
    if (!origin) {
      return '@thunderid/javascript';
    }

    return `@thunderid/${origin}`;
  }

  constructor(message: string, code: string, origin: string) {
    const resolvedOrigin: string = ThunderIDError.resolveOrigin(origin);
    super(message);

    this.name = new.target.name;
    this.code = code;
    this.origin = resolvedOrigin;

    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, new.target);
    }
  }

  public override toString(): string {
    const prefix = `⚡ ThunderID - ${this.origin}:`;
    return `[${this.name}]\n${prefix} ${this.message}\n(code="${this.code}")`;
  }
}
