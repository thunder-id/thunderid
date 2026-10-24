// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ApiError} from '@thunderid/types';
import type {FlowExecutionError} from '../flow/administrationFlow';

/**
 * A minimal subset of the i18next TFunction interface used by {@link getErrorMessage}.
 */
type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

/**
 * Extracts a localized error message from an API error response.
 *
 * Attempts to resolve a specific i18n message for the error code returned by the API, first against the
 * caller's own namespace (e.g. `errors.APP-1020`), then against the shared `common` namespace (e.g.
 * `common:errors.SSE-4030`) for codes that are shared across services rather than owned by one feature. If
 * neither resolves, falls back to the provided generic key.
 *
 * A refused administration flow step is resolved the same way from the code the thrown error carries,
 * with the step's params interpolated into the message.
 *
 * @param error - The error thrown by the mutation
 * @param t - The i18next translation function scoped to the relevant namespace. Must forward an explicit `ns:`
 *   prefix unchanged (rather than re-prefixing it) so the `common:errors.<CODE>` lookup can resolve
 * @param fallbackKey - i18n key to use when no specific message is found (e.g. `'create.error'`)
 * @param fallbackDefaultValue - Default string for `fallbackKey`, per the i18n Fallback Values convention, so a
 *   missing locale key degrades to readable text instead of the raw key
 * @returns Localized error message string
 *
 * @example
 * ```typescript
 * setError(getErrorMessage(error, t, 'create.error', 'Failed to create application. Please try again.'));
 * ```
 *
 * @public
 */
export default function getErrorMessage(
  error: Error,
  t: TranslateFn,
  fallbackKey: string,
  fallbackDefaultValue?: string,
): string {
  const apiError = (error as {response?: {data?: ApiError}}).response?.data;

  if (apiError?.code) {
    // The server sends its description as an i18n object with optional params; interpolate them so a
    // localized key (e.g. a usage summary) fills in, while unused params are ignored by i18next.
    const options = {...apiErrorParams(apiError), defaultValue: ''};
    // Feature namespace first, then the shared catalog for codes shared across services (e.g. SSE-4030).
    const specific = t(`errors.${apiError.code}`, options) || t(`common:errors.${apiError.code}`, options);

    if (specific) {
      return specific;
    }
  } else {
    const flowError = flowErrorOf(error);

    if (flowError?.code) {
      const options = {...(flowError.message?.params ?? flowError.description?.params), defaultValue: ''};
      const specific = t(`errors.${flowError.code}`, options) || t(`common:errors.${flowError.code}`, options);

      if (specific) {
        return specific;
      }
    }
  }

  if (fallbackDefaultValue !== undefined) {
    return t(fallbackKey, {defaultValue: fallbackDefaultValue});
  }

  return t(fallbackKey);
}

/**
 * Reads the interpolation params the server attaches to an API error's `description`. The wire sends
 * `description` as an i18n object (`{key, defaultValue, params}`) but tolerates a plain string, so
 * return params only when the object shape is present.
 */
function apiErrorParams(apiError: ApiError): Record<string, string> | undefined {
  const {description} = apiError;

  return typeof description === 'object' ? description.params : undefined;
}

/**
 * Reads the step error a failed flow reports on the thrown error, as `FlowExecutionFailure` and the embedded
 * create flows both do. An axios transport error also carries a `code` (e.g. `ERR_NETWORK`), which is not one.
 */
function flowErrorOf(error: Error): FlowExecutionError | undefined {
  const candidate = error as Error & {code?: string; error?: FlowExecutionError; isAxiosError?: boolean};

  if (candidate.isAxiosError) {
    return undefined;
  }

  return {...candidate.error, code: candidate.code ?? candidate.error?.code};
}
