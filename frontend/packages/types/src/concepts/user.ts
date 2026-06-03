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
 * User representation.
 *
 * Describes the core user shape shared across frontend packages
 * for identity, organizational context, and profile data.
 *
 * @public
 */
export interface User {
  /**
   * Unique identifier of the user.
   */
  id: string;

  /**
   * Identifier of the organizational unit the user belongs to.
   */
  ouId: string;

  /**
   * Handle of the organizational unit, when available.
   */
  ouHandle?: string;

  /**
   * User category or classification.
   */
  type: string;

  /**
   * Additional user attributes returned by the API.
   */
  attributes?: Record<string, unknown>;

  /**
   * Human-readable display name for the user.
   */
  display?: string;

  /**
   * Whether the user is read-only and cannot be modified or deleted.
   */
  isReadOnly?: boolean;
}
