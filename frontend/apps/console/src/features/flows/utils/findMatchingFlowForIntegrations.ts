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

import getFlowSupportedIntegrations from './getFlowSupportedIntegrations';
import type {BasicFlowDefinition} from '../models/responses';

/**
 * Find the best matching flow for given enabled integrations
 *
 * @param enabledIntegrations - Array of currently enabled integration IDs
 * @param availableFlows - List of available authentication flows
 * @returns Matching flow or null if no suitable match found
 *
 * @public
 * @example
 * ```ts
 * import findMatchingFlowForIntegrations from './findMatchingFlowForIntegrations';
 *
 * const flows = [
 *   { id: '1', handle: 'basic-google-flow', name: 'Basic + Google' },
 *   { id: '2', handle: 'default-flow', name: 'Credentials Auth' }
 * ];
 * const match = findMatchingFlowForIntegrations(['credentials_auth', 'google'], flows);
 * // Returns the basic-google-flow
 * ```
 */
function findMatchingFlowForIntegrations(
  enabledIntegrations: string[],
  availableFlows: BasicFlowDefinition[],
): BasicFlowDefinition | null {
  if (enabledIntegrations.length === 0) {
    return null;
  }

  // Normalize integration IDs for comparison
  const normalizedIntegrations = enabledIntegrations.map((id) => {
    if (id.includes('google')) return 'google';
    if (id.includes('github')) return 'github';
    if (id.includes('sms') || id === 'sms-otp') return 'sms-otp';
    return id;
  });

  // Find flow that best matches the enabled integrations
  const matchingFlow = availableFlows.find((flow) => {
    if (!flow.handle) return false;

    const normalizedFlowIntegrations = getFlowSupportedIntegrations(flow.handle);

    // Check if all enabled integrations are supported by this flow
    const allIntegrationsSupported = normalizedIntegrations.every((integration) =>
      normalizedFlowIntegrations.includes(integration),
    );

    // Prefer flows that match exactly (same number of integrations)
    const exactMatch = normalizedIntegrations.length === normalizedFlowIntegrations.length;

    return allIntegrationsSupported && exactMatch;
  });

  if (!matchingFlow) {
    return null;
  }

  return matchingFlow;
}

export default findMatchingFlowForIntegrations;
