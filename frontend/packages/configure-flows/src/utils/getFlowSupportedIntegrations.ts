// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AuthenticatorTypes} from '@thunderid/configure-connections';

/**
 * Determines which integrations are supported by a given flow
 *
 * @param flowHandle - The flow handle to analyze
 * @returns Array of integration types supported by the flow
 *
 * @public
 * @example
 * ```ts
 * import getFlowSupportedIntegrations from './getFlowSupportedIntegrations';
 *
 * const integrations = getFlowSupportedIntegrations('basic-google-github-flow');
 * // Returns: ['credentials_auth', 'google', 'github']
 * ```
 */
function getFlowSupportedIntegrations(flowHandle: string): string[] {
  const integrations: string[] = [];

  // Check for credentials auth (flow handles use 'basic' as the handle segment)
  if (flowHandle.includes('basic')) {
    integrations.push(AuthenticatorTypes.CREDENTIALS_AUTH);
  }

  // Check for Google
  if (flowHandle.includes('google')) {
    integrations.push('google');
  }

  // Check for GitHub
  if (flowHandle.includes('github')) {
    integrations.push('github');
  }

  // Check for SMS OTP
  if (flowHandle.includes('sms')) {
    integrations.push('sms-otp');
  }

  // Check for Passkey
  if (flowHandle.includes('passkey')) {
    integrations.push(AuthenticatorTypes.PASSKEY);
  }

  // Check for Magic Link
  if (flowHandle.includes('magiclink')) {
    integrations.push(AuthenticatorTypes.MAGIC_LINK);
  }

  return integrations;
}

export default getFlowSupportedIntegrations;
