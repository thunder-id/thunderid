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

import ScopeConstants from './ScopeConstants';

/**
 * Constants representing standard OpenID Connect (OIDC) request and response parameters.
 * These parameters are commonly used during authorization, token exchange, and logout flows.
 */
const OIDCRequestConstants: {
  readonly Params: {
    readonly AUTHORIZATION_CODE: string;
    readonly SESSION_STATE: string;
    readonly SIGN_OUT_SUCCESS: string;
    readonly STATE: string;
  };
  readonly SignIn: {
    readonly Payload: {
      readonly DEFAULT_SCOPES: readonly string[];
    };
  };
  readonly SignOut: {
    readonly Storage: {
      readonly StorageKeys: {
        readonly SIGN_OUT_URL: string;
      };
    };
  };
} = {
  Params: {
    /**
     * The authorization code returned from the authorization endpoint.
     * Used in the authorization code flow.
     */
    AUTHORIZATION_CODE: 'code',

    /**
     * Session state parameter used for session management between the client and the OP.
     */
    SESSION_STATE: 'session_state',

    /**
     * Indicates whether sign-out was successful during the end-session flow.
     * May be returned by the OP after a logout request.
     */
    SIGN_OUT_SUCCESS: 'sign_out_success',

    /**
     * State parameter used to maintain state between the request and the callback.
     * Helps in preventing CSRF attacks.
     */
    STATE: 'state',
  },

  /**
   * Constants related to the OpenID Connect (OIDC) sign-in flow.
   */
  SignIn: {
    /**
     * Constants related to the payload of the OIDC sign-in request.
     */
    Payload: {
      /**
       * The default scopes used in OIDC sign-in requests.
       */
      DEFAULT_SCOPES: [ScopeConstants.OPENID, ScopeConstants.PROFILE, ScopeConstants.INTERNAL_LOGIN],
    },
  },

  /**
   * Sign-out related constants for managing the end-session flow in OIDC.
   */
  SignOut: {
    /**
     * Storage-related constants for managing sign-out state.
     */
    Storage: {
      /**
       * Collection of storage keys used in sign-out implementation
       */
      StorageKeys: {
        /**
         * Storage key for the sign-out URL.
         * Used to store the complete URL where the user should be redirected after
         * completing the OIDC logout process.
         */
        SIGN_OUT_URL: 'sign_out_url',
      },
    },
  },
} as const;

export default OIDCRequestConstants;
