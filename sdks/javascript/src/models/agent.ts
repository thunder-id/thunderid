/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

/**
 * Interface representing the configuration for an agent.
 */
export interface AgentConfig {
  /**
   * The unique identifier for the agent
   */
  agentID: string;
  /**
   * The secret credential for the agent
   */
  agentSecret: string;
  /**
   * The authenticator name to match during the embedded sign-in flow.
   * Defaults to {@link AgentConfig.DEFAULT_AUTHENTICATOR_NAME} if not provided.
   */
  authenticatorName?: string;
}

/**
 * Namespace that holds constants related to {@link AgentConfig}.
 */
export namespace AgentConfig {
  /**
   * Default authenticator name used when none is specified.
   */
  export const DEFAULT_AUTHENTICATOR_NAME = 'Username & Password';
}
