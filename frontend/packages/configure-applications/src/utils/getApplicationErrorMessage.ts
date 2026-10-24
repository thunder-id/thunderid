// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import getBackchannelLogoutUriServerError from './getBackchannelLogoutUriServerError';

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
  authentication: {key: 'edit.flows.labels.authFlow', defaultValue: 'Sign-in flow'},
  registration: {key: 'edit.flows.labels.registrationFlow', defaultValue: 'Sign-up flow'},
  recovery: {key: 'edit.flows.labels.recoveryFlow', defaultValue: 'Recovery flow'},
  signout: {key: 'edit.flows.labels.signOutFlow', defaultValue: 'Sign out flow'},
};

/**
 * Extracts a localized error message from an application API error response, with a dedicated,
 * actionable message for APP-1039 (conflicting flow references).
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
  const backchannelLogoutUriError = getBackchannelLogoutUriServerError(error, t);
  if (backchannelLogoutUriError) {
    return backchannelLogoutUriError;
  }

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

  return getErrorMessage(error, t, fallbackKey, fallbackDefaultValue);
}
