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

/**
 * Re-export of the navigation helper from the Browser SDK.
 *
 * Navigates to a new URL within the browser:
 * - For same-origin URLs: Uses the History API and dispatches a `popstate` event (SPA navigation).
 * - For cross-origin URLs: Performs a full page load using `window.location.assign`.
 *
 * @see {@link @thunderid/browser#navigate}
 */
export {navigate, navigate as default} from '@thunderid/browser';
