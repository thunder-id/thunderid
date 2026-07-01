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

import {AuthenticatorTypes} from '../../connections/models/authenticators';

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

  return integrations;
}

export default getFlowSupportedIntegrations;
