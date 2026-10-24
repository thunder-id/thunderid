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
 *   code: 'APP-1020',
 *   message: 'Application already exists',
 *   description: 'An application with the same name already exists'
 * };
 * ```
 */
export interface ApiError {
  /**
   * Machine-readable error code
   * @example 'APP-1020'
   */
  code: string;

  /**
   * Short error message. The backend serializes this as a translatable {@link I18nMessage} object,
   * but plain strings are tolerated for safety.
   */
  message: I18nMessage | string;

  /**
   * Detailed error description. The backend serializes this as a translatable {@link I18nMessage}
   * object, but plain strings are tolerated for safety.
   */
  description: I18nMessage | string;
}

/**
 * Translatable message the backend returns, carrying a translation key, a human-readable default
 * value, and optional substitution params.
 *
 * @public
 */
export interface I18nMessage {
  /** Translation key. */
  key: string;

  /** Human-readable fallback used when no translation is available. */
  defaultValue: string;

  /** Optional parameters substituted into the default value. */
  params?: Record<string, string>;
}
