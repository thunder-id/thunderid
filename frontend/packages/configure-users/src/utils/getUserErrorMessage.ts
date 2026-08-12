// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';

/**
 * The subset of the USR-1027 (blocking dependencies) API error response this util reads.
 */
interface BlockingDependenciesApiError {
  code: string;
  description?: {
    params?: {
      dependencies?: string;
    };
  };
}

const BLOCKING_DEPENDENCIES_ERROR_CODE = 'USR-1027';

const BLOCKING_DEPENDENCIES_DEFAULT_VALUE =
  'This user cannot be deleted because {{dependencies}} depend on it. Remove or reassign them first.';

/**
 * Reads the error code from either the REST envelope (`response.data.code`) or the embedded
 * flow's variants (`code`, `error.code`), since the user create flow reports failures through a
 * different shape than a plain API mutation. See `UserAddPage`'s `isMissingOnboardingFlow`.
 */
function extractErrorCode(error: Error): string | undefined {
  const candidate = error as Error & {
    code?: string;
    error?: {code?: string};
    response?: {data?: {code?: string}};
  };

  return candidate.response?.data?.code ?? candidate.code ?? candidate.error?.code;
}

/**
 * Reads the interpolation params the flow engine attaches to a parameterized error, so a mapped
 * message resolves its placeholders instead of rendering them literally. Params ride on the
 * message, with the description as a fallback.
 */
function extractFlowErrorParams(error: Error): Record<string, string> | undefined {
  const candidate = error as Error & {
    error?: {
      description?: {params?: Record<string, string>};
      message?: {params?: Record<string, string>};
    };
  };

  return candidate.error?.message?.params ?? candidate.error?.description?.params;
}

/**
 * Extracts a localized error message from a user API error response, with a dedicated,
 * actionable message for USR-1027 (blocking dependencies) and normalization for the embedded
 * flow's error envelope.
 *
 * For USR-1027, builds the message from the error's `dependencies` param, a human-readable
 * summary (e.g. "2 agent(s)") supplied by the backend. Falls back to {@link getErrorMessage} for
 * every other error, and for USR-1027 responses without usable params.
 *
 * @param error - The error thrown by the mutation, or reported by the embedded create flow
 * @param t - The i18next translation function scoped to the relevant namespace
 * @param fallbackKey - i18n key to use when no specific message is found (e.g. `'delete.error'`)
 * @param fallbackDefaultValue - Default string for `fallbackKey`, per the i18n Fallback Values convention
 * @returns Localized error message string
 *
 * @public
 */
export default function getUserErrorMessage(
  error: Error,
  t: (key: string, options?: Record<string, unknown>) => string,
  fallbackKey: string,
  fallbackDefaultValue?: string,
): string {
  const code = extractErrorCode(error);

  if (code === BLOCKING_DEPENDENCIES_ERROR_CODE) {
    const apiError = (error as {response?: {data?: BlockingDependenciesApiError}}).response?.data;
    const dependencies = apiError?.description?.params?.dependencies;

    if (dependencies) {
      return t(`errors.${BLOCKING_DEPENDENCIES_ERROR_CODE}`, {
        dependencies,
        defaultValue: BLOCKING_DEPENDENCIES_DEFAULT_VALUE,
      });
    }

    // Skip the generic code lookup: it would resolve the same errors.USR-1027 key and return its
    // {{dependencies}} placeholder unresolved.
    return fallbackDefaultValue !== undefined ? t(fallbackKey, {defaultValue: fallbackDefaultValue}) : t(fallbackKey);
  }

  if (code && !(error as {response?: {data?: {code?: string}}}).response?.data?.code) {
    // The code came from a flow-shaped envelope getErrorMessage cannot see (response.data.code is
    // absent), so resolve it here instead of delegating. Mirrors getErrorMessage's two-tier lookup:
    // the feature namespace first, then the shared catalog for cross-service codes (e.g. SSE-4030).
    const params = extractFlowErrorParams(error);
    const options = {...params, defaultValue: ''};
    const specific = t(`errors.${code}`, options) || t(`common:errors.${code}`, options);

    if (specific) {
      return specific;
    }

    return fallbackDefaultValue !== undefined ? t(fallbackKey, {defaultValue: fallbackDefaultValue}) : t(fallbackKey);
  }

  return getErrorMessage(error, t, fallbackKey, fallbackDefaultValue);
}
