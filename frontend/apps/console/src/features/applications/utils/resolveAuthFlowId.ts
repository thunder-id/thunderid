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

import {
  IdentityProviderTypes,
  type IdentityProvider,
  type IdentityProviderType,
} from '@thunderid/configure-connections';
import {AUTH_FLOW_GRAPHS} from '../models/auth-flow-graphs';

/**
 * Options for resolving authentication flow configuration.
 */
interface ResolveAuthFlowOptions {
  /**
   * Whether username/password authentication is enabled
   */
  hasUsernamePassword: boolean;

  /**
   * Array of selected identity providers (e.g., Google, GitHub)
   */
  identityProviders: IdentityProvider[];
}

/**
 * Resolves the appropriate authentication flow ID based on selected sign-in options.
 *
 * The resolver follows these rules:
 * 1. If only username/password is selected -> auth_flow_config_basic
 * 2. If only Google is selected -> auth_flow_config_google
 * 3. If only GitHub is selected -> auth_flow_config_github
 * 4. If username/password + Google -> auth_flow_config_basic_google
 * 5. If username/password + GitHub -> auth_flow_config_basic (GitHub not supported alone with basic)
 * 6. If username/password + Google + GitHub -> auth_flow_config_basic_google_github
 * 7. If only social logins (Google and/or GitHub) -> Uses the appropriate social flow
 *
 * @param options - Configuration object with sign-in options
 * @param options.hasUsernamePassword - Whether username/password authentication is enabled
 * @param options.identityProviders - Array of selected identity providers
 * @returns The authentication flow ID that matches the selected options
 *
 * @example
 * ```tsx
 * // Username & Password only
 * const flowId = resolveAuthFlowId({
 *   hasUsernamePassword: true,
 *   identityProviders: []
 * });
 * // Returns: 'auth_flow_config_basic'
 *
 * // Username & Password + Google
 * const flowId = resolveAuthFlowId({
 *   hasUsernamePassword: true,
 *   identityProviders: [{ id: '123', name: 'Google', type: 'GOOGLE' }]
 * });
 * // Returns: 'auth_flow_config_basic_google'
 *
 * // Username & Password + Google + GitHub
 * const flowId = resolveAuthFlowId({
 *   hasUsernamePassword: true,
 *   identityProviders: [
 *     { id: '123', name: 'Google', type: 'GOOGLE' },
 *     { id: '456', name: 'GitHub', type: 'GITHUB' }
 *   ]
 * });
 * // Returns: 'auth_flow_config_basic_google_github'
 * ```
 */
export default function resolveAuthFlowId({hasUsernamePassword, identityProviders}: ResolveAuthFlowOptions): string {
  const providerTypes: IdentityProviderType[] = identityProviders.map((idp: IdentityProvider) => idp.type);
  const hasGoogle: boolean = providerTypes.includes(IdentityProviderTypes.GOOGLE);
  const hasGitHub: boolean = providerTypes.includes(IdentityProviderTypes.GITHUB);

  // Only username/password
  if (hasUsernamePassword && !hasGoogle && !hasGitHub) {
    return AUTH_FLOW_GRAPHS.BASIC;
  }

  // Only Google
  if (!hasUsernamePassword && hasGoogle && !hasGitHub) {
    return AUTH_FLOW_GRAPHS.GOOGLE;
  }

  // Only GitHub
  if (!hasUsernamePassword && !hasGoogle && hasGitHub) {
    return AUTH_FLOW_GRAPHS.GITHUB;
  }

  // Username/Password + Google
  if (hasUsernamePassword && hasGoogle && !hasGitHub) {
    return AUTH_FLOW_GRAPHS.BASIC_GOOGLE;
  }

  // Username/Password + Google + GitHub
  if (hasUsernamePassword && hasGoogle && hasGitHub) {
    return AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB;
  }

  // Username/Password + GitHub (fallback to basic since there's no basic_github flow)
  if (hasUsernamePassword && !hasGoogle && hasGitHub) {
    return AUTH_FLOW_GRAPHS.BASIC;
  }

  // Only Google + GitHub (no username/password)
  if (!hasUsernamePassword && hasGoogle && hasGitHub) {
    // Fallback to basic_google_github since there's no social-only multi-provider flow
    return AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB;
  }

  // Default fallback to basic
  return AUTH_FLOW_GRAPHS.BASIC;
}
