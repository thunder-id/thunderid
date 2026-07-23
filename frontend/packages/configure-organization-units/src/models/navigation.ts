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
 * Organization Unit Navigation State
 *
 * State passed via React Router when navigating between organization units.
 * Used to track the source OU for proper back navigation in the edit page.
 *
 * @public
 * @remarks
 * This state is passed through `useNavigate` and consumed via `useLocation().state`.
 * It enables the "Back to [OU Name]" navigation link in the edit page header.
 *
 * @example
 * ```typescript
 * // Navigating to a child OU from a parent
 * navigate(routes.detail(childId), {
 *   state: {
 *     fromOU: { id: parentId, name: parentName }
 *   } satisfies OUNavigationState
 * });
 * ```
 */
export interface OUNavigationState {
  /** The source organization unit that was navigated away from */
  fromOU: {
    /** ID of the source organization unit */
    id: string;
    /** Display name of the source organization unit */
    name: string;
  };
}
