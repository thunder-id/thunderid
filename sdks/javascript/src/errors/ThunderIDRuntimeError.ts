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

import ThunderIDError from './ThunderIDError';

/**
 * Base class for all runtime errors in ThunderID. This class extends ThunderIDError
 * and adds support for additional error details. Use this class for errors that occur
 * during runtime execution that are not related to API calls.
 *
 * @example
 * ```typescript
 * throw new ThunderIDRuntimeError(
 *   "Failed to parse configuration",
 *   "CONFIG_PARSE_ERROR",
 *   { invalidField: "redirectUri" }
 * );
 * ```
 */
export default class ThunderIDRuntimeError extends ThunderIDError {
  /**
   * Creates an instance of ThunderIDRuntimeError.
   *
   * @param message - Human-readable description of the error
   * @param code - A unique error code that identifies the error type
   * @param details - Additional details about the error that might be helpful for debugging
   * @param origin - Optional. The SDK origin (e.g. 'react', 'vue'). Defaults to generic 'ThunderID'
   * @constructor
   */
  constructor(
    message: string,
    code: string,
    origin: string,
    public readonly details?: unknown,
  ) {
    super(message, code, origin);

    Object.defineProperty(this, 'name', {
      configurable: true,
      value: 'ThunderIDRuntimeError',
      writable: true,
    });
  }

  /**
   * Returns a string representation of the runtime error
   * @returns Formatted error string with name, code, details, and message
   */
  public override toString(): string {
    const details: string = this.details ? `\nDetails: ${JSON.stringify(this.details, null, 2)}` : '';
    return `[${this.name}] (code="${this.code}")${details}\nMessage: ${this.message}`;
  }
}
