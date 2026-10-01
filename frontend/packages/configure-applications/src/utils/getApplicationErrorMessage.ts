// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';

// Applications now show every mutation failure inline (see frontend/AGENTS.md's Error Display
// section) instead of duplicating it as a toast. The rest of the console still has this
// duplicate-or-toast-only inconsistency across features; unifying it is tracked in
// https://github.com/thunder-id/thunderid/issues/4555.

/**
 * The subset of the APP-1039 (flow mismatch) API error response this util reads.
 */
interface FlowMismatchApiError {
  code: string;
  description?: {
    params?: {
      sourceFlowType?: string;
      flowType?: string;
    };
  };
}

const FLOW_MISMATCH_ERROR_CODE = 'APP-1039';

const FLOW_MISMATCH_DEFAULT_VALUE =
  'The {{sourceFlowType}} references a different {{flowType}} than the one configured for this application. Update ' +
  "the {{sourceFlowType}} so it calls the same {{flowType}}, or change the application's {{flowType}} configuration.";

/**
 * Maps the raw flow-type values the backend sends in APP-1039's params to the i18n key and
 * fallback default already used for the same flows on the application's Flows tab.
 */
const FLOW_TYPE_LABELS: Record<string, {key: string; defaultValue: string}> = {
  authentication: {key: 'edit.flows.labels.authFlow', defaultValue: 'Sign-in Flow'},
  registration: {key: 'edit.flows.labels.registrationFlow', defaultValue: 'Sign-up Flow'},
  recovery: {key: 'edit.flows.labels.recoveryFlow', defaultValue: 'Recovery Flow'},
  signout: {key: 'edit.flows.labels.signOutFlow', defaultValue: 'Sign Out Flow'},
};

/**
 * Reads the error code an administration flow reports on a refused step.
 *
 * A refusal arrives as a failed step inside a 200 response, so its code rides on the thrown error
 * rather than on `response.data`, which is what {@link getErrorMessage} reads.
 */
function extractFlowErrorCode(error: Error): string | undefined {
  const candidate = error as Error & {code?: string; error?: {code?: string}};

  return candidate.code ?? candidate.error?.code;
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
 * Extracts a localized error message from an application API error response, with a dedicated,
 * actionable message for APP-1039 (conflicting flow references), and code resolution for the
 * administration flows the delete and secret-regeneration actions run through.
 *
 * For APP-1039, builds the message from the error's `sourceFlowType`/`flowType` params,
 * translated into the same flow labels shown on the application's Flows tab (e.g. "Sign-up Flow",
 * "Recovery Flow"), instead of using the backend's generic description text. Falls back to
 * {@link getErrorMessage} for every other error, and for APP-1039 responses without usable params.
 *
 * @param error - The error thrown by the mutation
 * @param t - The i18next translation function scoped to the relevant namespace
 * @param fallbackKey - i18n key to use when no specific message is found (e.g. `'create.error'`)
 * @param fallbackDefaultValue - Default string for `fallbackKey`, per the i18n Fallback Values convention
 * @returns Localized error message string
 *
 * @public
 */
export default function getApplicationErrorMessage(
  error: Error,
  t: (key: string, options?: Record<string, unknown>) => string,
  fallbackKey: string,
  fallbackDefaultValue?: string,
): string {
  const apiError = (error as {response?: {data?: FlowMismatchApiError}}).response?.data;

  if (apiError?.code === FLOW_MISMATCH_ERROR_CODE) {
    const sourceFlowType = apiError.description?.params?.sourceFlowType?.toLowerCase();
    const flowType = apiError.description?.params?.flowType?.toLowerCase();
    const sourceLabel = sourceFlowType ? FLOW_TYPE_LABELS[sourceFlowType] : undefined;
    const flowLabel = flowType ? FLOW_TYPE_LABELS[flowType] : undefined;

    if (sourceLabel && flowLabel) {
      return t(`errors.${FLOW_MISMATCH_ERROR_CODE}`, {
        sourceFlowType: t(sourceLabel.key, {defaultValue: sourceLabel.defaultValue}),
        flowType: t(flowLabel.key, {defaultValue: flowLabel.defaultValue}),
        defaultValue: FLOW_MISMATCH_DEFAULT_VALUE,
      });
    }

    // Skip the generic code-based lookup: it would resolve the same errors.APP-1039 key and
    // return its {{sourceFlowType}}/{{flowType}} placeholders unresolved.
    if (fallbackDefaultValue !== undefined) {
      return t(fallbackKey, {defaultValue: fallbackDefaultValue});
    }
    return t(fallbackKey);
  }

  const flowCode = extractFlowErrorCode(error);

  if (flowCode && !apiError?.code) {
    // The code came from a failed flow step getErrorMessage cannot see (response.data.code is
    // absent), so resolve it here instead of delegating. Mirrors getErrorMessage's two-tier lookup:
    // the feature namespace first, then the shared catalog for cross-service codes (e.g. FET-1086).
    const options = {...extractFlowErrorParams(error), defaultValue: ''};
    const specific = t(`errors.${flowCode}`, options) || t(`common:errors.${flowCode}`, options);

    if (specific) {
      return specific;
    }
  }

  return getErrorMessage(error, t, fallbackKey, fallbackDefaultValue);
}
