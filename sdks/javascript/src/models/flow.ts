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

export enum FlowMode {
  /**
   * This mode is suitable for embedded sign-in, sign-up, etc. flows where the authentication
   * UIs are rendered within the application.
   * @see {@link https://is.docs.wso2.com/en/7.1.0/references/app-native-authentication/}
   */
  Embedded = 'DIRECT',
  /**
   * Traditional redirect based sign-in, sign-up, etc. flows where the authentication
   * UIs are from a external Identity Provider (ex: WSO2 Identity Server or ThunderID).
   */
  Redirect = 'REDIRECTION',
}
