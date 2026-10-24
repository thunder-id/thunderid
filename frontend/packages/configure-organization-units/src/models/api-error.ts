// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
   * @example 'Cannot delete an organization unit that has child organization units, users, groups, or sharing policies it issued'
   */
  defaultValue: string;

  /**
   * Optional parameters substituted into the default value
   */
  params?: Record<string, string>;
}
