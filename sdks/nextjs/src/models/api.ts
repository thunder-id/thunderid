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
 * Interface defining the internal API routes for authentication.
 * These routes are used internally by the ThunderID Next.js SDK for handling authentication flows.
 */
export interface InternalAuthAPIRoutes {
  /**
   * Route for handling session management.
   * This route should return the current signed-in status.
   */
  session: string;
  /**
   * Route for handling sign-in requests.
   * This route should handle the sign-in flow and redirect users to the appropriate authentication endpoint.
   */
  signIn: string;

  /**
   * Route for handling sign-out requests.
   * This route should handle the sign-out flow and clean up any authentication state.
   */
  signOut: string;

  /**
   * Route for handling sign-up requests.
   * This route should handle the sign-up flow and redirect users to the appropriate registration endpoint.
   */
  signUp?: string;

  /**
   * Route for handling user information retrieval.
   * This route should return the current user's information, such as username, email, etc.
   */
  user: string;
}
