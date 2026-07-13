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

/**
 * Standard API Error Structure
 *
 * Represents the error response body returned by the API
 * when an operation fails.
 *
 * @public
 * @remarks
 * This structure is used for error handling in API responses.
 * The `code` field contains a machine-readable error identifier,
 * while `message` and `description` provide human-readable details.
 *
 * @example
 * ```typescript
 * const error: ApiError = {
 *   code: 'OU-60001',
 *   message: {key: 'error.ouservice.not_found', defaultValue: 'Organization unit not found'},
 *   description: {key: 'error.ouservice.not_found_description', defaultValue: 'No organization unit exists with the given ID'}
 * };
 * ```
 */
export interface ApiError {
  /**
   * Machine-readable error code
   * @example 'OU-60001'
   */
  code: string;

  /**
   * Short error message. The backend serializes this as a translatable
   * {@link I18nMessage} object, but plain strings are tolerated for safety.
   */
  message: I18nMessage | string;

  /**
   * Detailed error description. The backend serializes this as a translatable
   * {@link I18nMessage} object, but plain strings are tolerated for safety.
   */
  description: I18nMessage | string;
}

/**
 * Translatable message returned by the backend, carrying a translation key
 * and a human-readable default value.
 *
 * @public
 */
export interface I18nMessage {
  /**
   * Translation key
   * @example 'error.ouservice.organization_unit_has_children_description'
   */
  key: string;

  /**
   * Human-readable fallback used when no translation is available
   * @example 'Cannot delete organization unit with children or users/groups'
   */
  defaultValue: string;

  /**
   * Optional parameters substituted into the default value
   */
  params?: Record<string, string>;
}
