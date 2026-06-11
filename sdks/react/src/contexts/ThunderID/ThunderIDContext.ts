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
  FlowMetadataResponse,
  HttpRequestConfig,
  HttpResponse,
  IdToken,
  OIDCDiscoveryApiResponse,
  Organization,
  Platform,
  SignInOptions,
  TokenExchangeRequestConfig,
  TokenResponse,
} from '@thunderid/browser';
import {Context, createContext} from 'react';
import {ThunderIDReactConfig} from '../../models/config';
import ThunderIDReactClient from '../../ThunderIDReactClient';

/**
 * Props interface of {@link ThunderIDContext}
 */
export type ThunderIDContextProps = {
  afterSignInUrl: string | undefined;
  applicationId: string | undefined;
  baseUrl: string | undefined;
  clientId: string | undefined;
  scopes: string | string[] | undefined;
  /**
   * OIDC discovery data.
   */
  discovery: {
    /**
     * The response from the `.well-known/openid-configuration` endpoint.
     * Contains server capabilities, supported endpoints, and metadata.
     * `null` while loading or when discovery has not been fetched.
     */
    wellKnown: OIDCDiscoveryApiResponse | null;
  };
  /**
   * Swaps the current access token with a new one based on the provided configuration (with a grant type).
   * @param config - Configuration for the token exchange request.
   * @returns A promise that resolves to the token response or the raw response.
   */
  exchangeToken: (config: TokenExchangeRequestConfig) => Promise<TokenResponse | Response>;
  /**
   * Retrieves the access token stored in the storage.
   * This function retrieves the access token and returns it.
   * @remarks This does not work in the `webWorker` or any other worker environment.
   * @returns A promise that resolves to the access token.
   */
  getAccessToken: () => Promise<string>;
  /**
   * Function to retrieve the decoded ID token.
   * This function decodes the ID token and returns its payload.
   * It can be used to access user claims and other information contained in the ID token.
   *
   * @returns A promise that resolves to the decoded ID token payload.
   */
  getDecodedIdToken: () => Promise<IdToken>;
  /**
   * Function to retrieve the ID token.
   * This function retrieves the ID token and returns it.
   *
   * @returns A promise that resolves to the ID token.
   */
  getIdToken: () => Promise<string>;
  /**
   * Returns the underlying StorageManager instance for reading and writing SDK-managed storage.
   */
  getStorageManager: () => Promise<any>;
  /**
   * HTTP request function to make API calls.
   * @param requestConfig - Configuration for the HTTP request.
   * @returns A promise that resolves to the HTTP response.
   */
  http: {
    /**
     * Makes an HTTP request using the provided configuration.
     * @param requestConfig - Configuration for the HTTP request.
     * @returns A promise that resolves to the HTTP response.
     */
    request: (requestConfig?: HttpRequestConfig) => Promise<HttpResponse<any>>;
    /**
     * Makes multiple HTTP requests based on the provided configuration.
     * @param requestConfigs - Set of configurations for the HTTP requests.
     * @returns A promise that resolves to an array of HTTP responses.
     */
    requestAll: (requestConfigs?: HttpRequestConfig[]) => Promise<HttpResponse<any>[]>;
  };
  /**
   * Instance ID for multi-instance support.
   */
  instanceId: number;
  isInitialized: boolean;
  /**
   * Flag indicating whether the SDK is working in the background.
   */
  isLoading: boolean;
  /**
   * Flag indicating whether flow metadata is currently being fetched.
   */
  isMetaLoading: boolean;
  /**
   * Flag indicating whether the user is signed in or not.
   */
  isSignedIn: boolean;

  /**
   * Flow metadata returned by `GET /flow/meta` (v2 platform only).
   * `null` while loading or when `resolveFromMeta` is disabled.
   */
  meta: FlowMetadataResponse | null;

  organization: Organization;

  organizationHandle: string | undefined;

  /**
   * Re-initializes the client with a new configuration.
   *
   * @remarks
   * This can be partial configuration to update only specific fields.
   *
   * @param config - New configuration to re-initialize the client with.
   * @returns Promise resolving to boolean indicating success.
   */
  reInitialize: (config: Partial<ThunderIDReactConfig>) => Promise<boolean>;

  /**
   * Recovery function to initiate the account/password recovery flow.
   */
  recover: (...args: any[]) => Promise<any>;

  /**
   * Resolves `{{ t(key) }}` and `{{ meta(path) }}` template expressions in a string,
   * using the current i18n translation function and flow metadata from context.
   *
   * Useful in render-props patterns where consumers need to expand template strings
   * that come from the server (e.g. component labels, placeholders, headings).
   *
   * @example
   * const {resolveFlowTemplateLiterals} = useThunderID();
   * resolveFlowTemplateLiterals('{{ t(signin.heading.label) }}') // → 'Sign In'
   * resolveFlowTemplateLiterals('Login to {{ meta(application.name) }}') // → 'Login to My App'
   */
  resolveFlowTemplateLiterals: (text: string | undefined) => string;

  /**
   * Sign-in function to initiate the authentication process.
   * @remark This is the programmatic version of the `SignInButton` component.
   * TODO: Fix the types.
   */
  signIn: (...args: any) => Promise<any>;

  /**
   * Optional additional parameters to be sent in the sign-in request.
   * This can include custom parameters that your authorization server supports.
   * These parameters will be included in the authorization request sent to the server.
   * If not provided, no additional parameters will be sent.
   *
   * @example
   * signInOptions: { prompt: "login", fidp: "OrganizationSSO" }
   */
  signInOptions?: SignInOptions;

  /**
   * Optional token request configuration. params are appended to the token endpoint POST body.
   *
   * @example
   * tokenRequest: { params: { resource: "https://api.example.com" } }
   */
  tokenRequest?: {
    params?: Record<string, unknown>;
  };

  /**
   * Silent sign-in function to re-authenticate the user without user interaction.
   * @remark This is the programmatic version of the `SilentSignIn` component.
   */
  signInSilently: ThunderIDReactClient['signInSilently'];

  signInUrl: string | undefined;
  /**
   * Sign-out function to terminate the authentication session.
   * @remark This is the programmatic version of the `SignOutButton` component.
   * FIXME: Fix the types.
   */
  signOut: any;

  /**
   * Sign-up function to initiate the registration process.
   * @remark This is the programmatic version of the `SignUpButton` component.
   */
  signUp: (...args: any[]) => Promise<any>;

  signUpUrl: string | undefined;

  user: any;
  platform?: Platform;
} & Pick<ThunderIDReactClient, 'clearSession' | 'switchOrganization'>;

/**
 * Context object for managing the Authentication flow builder core context.
 */
const ThunderIDContext: Context<ThunderIDContextProps | null> = createContext<null | ThunderIDContextProps>({
  afterSignInUrl: undefined,
  applicationId: undefined,
  baseUrl: undefined,
  clearSession: () => {},
  clientId: undefined,
  scopes: undefined,
  discovery: {
    wellKnown: null,
  },
  exchangeToken: null as unknown as ThunderIDContextProps['exchangeToken'],
  getAccessToken: null as unknown as ThunderIDContextProps['getAccessToken'],
  getDecodedIdToken: null as unknown as ThunderIDContextProps['getDecodedIdToken'],
  getIdToken: null as unknown as ThunderIDContextProps['getIdToken'],
  getStorageManager: () => Promise.resolve(null),
  http: {
    request: () => null as unknown as Promise<HttpResponse<any>>,
    requestAll: () => null as unknown as Promise<HttpResponse<any>[]>,
  },
  instanceId: 0,
  isInitialized: false,
  isLoading: true,
  isMetaLoading: false,
  isSignedIn: false,
  meta: null,
  organization: null as unknown as Organization,
  organizationHandle: undefined,
  platform: undefined,
  reInitialize: null as unknown as ThunderIDContextProps['reInitialize'],
  recover: () => Promise.resolve({} as any),
  resolveFlowTemplateLiterals: (text: string | undefined) => text ?? '',
  signIn: () => Promise.resolve({} as any),
  signInSilently: () => Promise.resolve({} as any),
  signInUrl: undefined,
  signOut: () => Promise.resolve({} as any),
  signUp: () => Promise.resolve({} as any),
  signUpUrl: undefined,
  switchOrganization: null as unknown as ThunderIDContextProps['switchOrganization'],
  user: null,
});

ThunderIDContext.displayName = 'ThunderIDContext';

export default ThunderIDContext;
