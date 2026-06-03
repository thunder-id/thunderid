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

import {
  ThunderIDAuthException,
  User,
  IsomorphicCrypto,
  StorageManager,
  IdToken,
  ExtendedAuthorizeRequestUrlParams,
  OIDCEndpoints,
  TokenResponse,
  Config,
  HttpClient,
  HttpError,
  HttpRequestConfig,
  HttpResponse,
} from '@thunderid/javascript';
import SPAHelper from './SPAHelper';
import SPAUtils from './SPAUtils';
import {
  ACCESS_TOKEN_INVALID,
  CHECK_SESSION_SIGNED_IN,
  CHECK_SESSION_SIGNED_OUT,
  CUSTOM_GRANT_CONFIG,
  ERROR,
  ERROR_DESCRIPTION,
  PROMPT_NONE_IFRAME,
  REFRESH_ACCESS_TOKEN_ERR0R,
  RP_IFRAME,
} from '../constants/SPAConstants';
import {SPATokenExchangeConfig} from '../models/TokenExchangeConfig';

interface AuthorizationInfo {
  code: string;
  sessionState: string;
  pkce?: string;
  state: string;
  tokenRequestConfig?: {params: Record<string, unknown>};
}

interface Message<T> {
  type: string;
  data?: T;
}

interface HttpRequestInterface {
  httpClient: HttpClient;
  requestConfig: HttpRequestConfig;
  isHttpHandlerEnabled?: boolean;
  httpErrorCallback?: (error: HttpError) => void | Promise<void>;
  httpFinishCallback?: () => void;
  enableRetrievingSignOutURLFromSession?: (config: SPATokenExchangeConfig) => void;
}

interface SessionManagementHelperInterface {
  initialize(
    clientId: string,
    checkSessionEndpoint: string,
    getSessionState: () => Promise<string>,
    interval: number,
    sessionRefreshInterval: number,
    redirectURL: string,
    getSignInUrl: (params?: ExtendedAuthorizeRequestUrlParams) => Promise<string>,
  ): void;
  receivePromptNoneResponse(setSessionState?: (sessionState: string | null) => Promise<void>): Promise<boolean>;
  reset(): void;
}

/**
 * Browser-level authentication helper that orchestrates HTTP requests with token attachment,
 * automatic token refresh, session management, and the silent sign-in flow.
 *
 * @typeParam T - The browser client config extension type.
 */
class AuthenticationHelper<T> {
  private _storageManager: StorageManager<T>;
  private _spaHelper: SPAHelper<T>;
  private _instanceId: number;
  private _isTokenRefreshing: boolean;
  private _getUser: () => Promise<User>;
  private _refreshAccessToken: () => Promise<TokenResponse | User>;
  private _getAccessToken: (sessionId?: string) => Promise<string>;
  private _getIDPAccessToken: () => Promise<string>;
  private _isSignedIn: () => Promise<boolean>;
  private _getDecodedIdToken: (sessionId?: string) => Promise<IdToken>;
  private _getCrypto: () => Promise<IsomorphicCrypto>;
  private _getIdToken: () => Promise<string>;
  private _getOpenIDProviderEndpoints: () => Promise<OIDCEndpoints>;
  private _exchangeToken: (config: SPATokenExchangeConfig) => Promise<Response | TokenResponse | User>;
  private _setPKCECode: (pkceKey: string, state: string) => Promise<void>;

  /**
   * @param storageManager - Storage manager for reading config and session data.
   * @param spaHelper - Helper for managing token refresh timers.
   * @param instanceId - The instance ID used for signing out URL storage.
   * @param operations - Client operation callbacks to avoid circular dependency.
   */
  public constructor(
    storageManager: StorageManager<T>,
    spaHelper: SPAHelper<T>,
    instanceId: number,
    operations: {
      getUser: () => Promise<User>;
      refreshAccessToken: () => Promise<TokenResponse | User>;
      getAccessToken: (sessionId?: string) => Promise<string>;
      getIDPAccessToken: () => Promise<string>;
      isSignedIn: () => Promise<boolean>;
      getDecodedIdToken: (sessionId?: string) => Promise<IdToken>;
      getCrypto: () => Promise<IsomorphicCrypto>;
      getIdToken: () => Promise<string>;
      getOpenIDProviderEndpoints: () => Promise<OIDCEndpoints>;
      exchangeToken: (config: SPATokenExchangeConfig) => Promise<Response | TokenResponse | User>;
      setPKCECode: (pkceKey: string, state: string) => Promise<void>;
    },
  ) {
    this._storageManager = storageManager;
    this._spaHelper = spaHelper;
    this._instanceId = instanceId;
    this._isTokenRefreshing = false;
    this._getUser = operations.getUser;
    this._refreshAccessToken = operations.refreshAccessToken;
    this._getAccessToken = operations.getAccessToken;
    this._getIDPAccessToken = operations.getIDPAccessToken;
    this._isSignedIn = operations.isSignedIn;
    this._getDecodedIdToken = operations.getDecodedIdToken;
    this._getCrypto = operations.getCrypto;
    this._getIdToken = operations.getIdToken;
    this._getOpenIDProviderEndpoints = operations.getOpenIDProviderEndpoints;
    this._exchangeToken = operations.exchangeToken;
    this._setPKCECode = operations.setPKCECode;
  }

  /**
   * Enables request interception on the HTTP client.
   *
   * @param httpClient - The HTTP client to enable.
   */
  public enableHttpHandler(httpClient: HttpClient): void {
    httpClient?.enableHandler && httpClient.enableHandler();
  }

  /**
   * Disables request interception on the HTTP client.
   *
   * @param httpClient - The HTTP client to disable.
   */
  public disableHttpHandler(httpClient: HttpClient): void {
    httpClient?.disableHandler && httpClient.disableHandler();
  }

  /**
   * Initializes OIDC Session Management via an RP iframe.
   *
   * @param config - The current auth config.
   * @param oidcEndpoints - Resolved OIDC provider endpoints.
   * @param getSessionState - Returns the current session state from storage.
   * @param getAuthzURL - Builds an authorization URL with optional params.
   * @param sessionManagementHelper - The session management helper instance.
   */
  public initializeSessionManger(
    config: any,
    oidcEndpoints: OIDCEndpoints,
    getSessionState: () => Promise<string>,
    getAuthzURL: (params?: ExtendedAuthorizeRequestUrlParams) => Promise<string>,
    sessionManagementHelper: SessionManagementHelperInterface,
  ): void {
    sessionManagementHelper.initialize(
      config.clientId,
      oidcEndpoints.checkSessionIframe ?? '',
      getSessionState,
      config.checkSessionInterval ?? 3,
      config.sessionRefreshInterval ?? 300,
      config.afterSignInUrl,
      getAuthzURL,
    );
  }

  /**
   * Executes a custom token exchange grant, enforcing `allowedExternalUrls` rules when applicable.
   *
   * @param config - The token exchange configuration.
   * @param enableRetrievingSignOutURLFromSession - Callback invoked when `preventSignOutURLUpdate` is set.
   * @returns The user session or raw response.
   */
  public async exchangeToken(
    config: SPATokenExchangeConfig,
    enableRetrievingSignOutURLFromSession?: (config: SPATokenExchangeConfig) => void,
  ): Promise<User | Response> {
    const _config: Config = (await this._storageManager.getConfigData()) as Config;
    let useDefaultEndpoint = true;
    let matches = false;

    if (config?.tokenEndpoint) {
      useDefaultEndpoint = false;
      matches = true;
    }

    if (config.shouldReplayAfterRefresh) {
      this._storageManager.setTemporaryDataParameter(CUSTOM_GRANT_CONFIG, JSON.stringify(config));
    }

    if (useDefaultEndpoint || matches) {
      return this._exchangeToken(config)
        .then(async (response: Response | TokenResponse) => {
          if (enableRetrievingSignOutURLFromSession && typeof enableRetrievingSignOutURLFromSession === 'function') {
            enableRetrievingSignOutURLFromSession(config);
          }

          if (config.returnsSession) {
            await this._spaHelper.refreshAccessTokenAutomatically(() => this.refreshAccessToken());

            return this._getUser();
          } else {
            return response as Response;
          }
        })
        .catch((error) => {
          return Promise.reject(error);
        });
    } else {
      return Promise.reject(
        new ThunderIDAuthException(
          'SPA-MAIN_THREAD_CLIENT-RCG-IV01',
          'Request to the provided endpoint is prohibited.',
          'Requests can only be sent to resource servers specified by the `allowedExternalUrls`' +
            ' attribute while initializing the SDK.',
        ),
      );
    }
  }

  /**
   * Returns the stored custom grant config if a replay-after-refresh was scheduled, or `null`.
   */
  public async getCustomGrantConfigData(): Promise<SPATokenExchangeConfig | null> {
    const configString = await this._storageManager.getTemporaryDataParameter(CUSTOM_GRANT_CONFIG);

    if (configString) {
      return JSON.parse(configString as string);
    } else {
      return null;
    }
  }

  /**
   * Refreshes the access token, replays any scheduled custom grant, and reschedules auto-refresh.
   *
   * @param enableRetrievingSignOutURLFromSession - Callback for custom grant sign-out URL handling.
   * @returns The updated user session.
   */
  public async refreshAccessToken(
    enableRetrievingSignOutURLFromSession?: (config: SPATokenExchangeConfig) => void,
  ): Promise<User> {
    try {
      await this._refreshAccessToken();
      const customGrantConfig = await this.getCustomGrantConfigData();
      if (customGrantConfig) {
        await this.exchangeToken(customGrantConfig, enableRetrievingSignOutURLFromSession);
      }
      await this._spaHelper.refreshAccessTokenAutomatically(() => this.refreshAccessToken());

      return this._getUser();
    } catch (error) {
      const refreshTokenError: Message<string> = {
        type: REFRESH_ACCESS_TOKEN_ERR0R,
      };

      window.postMessage(refreshTokenError);
      return Promise.reject(error);
    }
  }

  private async retryFailedRequests(failedRequest: HttpRequestInterface): Promise<HttpResponse> {
    const {httpClient, requestConfig, isHttpHandlerEnabled, httpErrorCallback, httpFinishCallback} = failedRequest;

    await SPAUtils.until(() => !this._isTokenRefreshing);

    try {
      return await httpClient.request(requestConfig);
    } catch (error: any) {
      if (isHttpHandlerEnabled) {
        if (typeof httpErrorCallback === 'function') {
          await httpErrorCallback(error);
        }
        if (typeof httpFinishCallback === 'function') {
          httpFinishCallback();
        }
      }

      return Promise.reject(error);
    }
  }

  /**
   * Sends an HTTP request via the provided client, automatically attaching the token,
   * and retries once after a token refresh on a 401 response.
   *
   * @param httpClient - The HTTP client to use.
   * @param requestConfig - The request configuration.
   * @param isHttpHandlerEnabled - Whether request callbacks are active.
   * @param httpErrorCallback - Called when a request fails.
   * @param httpFinishCallback - Called when a request finishes.
   * @param enableRetrievingSignOutURLFromSession - Callback for custom grant sign-out handling.
   * @returns The HTTP response.
   */
  public async httpRequest(
    httpClient: HttpClient,
    requestConfig: HttpRequestConfig,
    isHttpHandlerEnabled?: boolean,
    httpErrorCallback?: (error: HttpError) => void | Promise<void>,
    httpFinishCallback?: () => void,
    enableRetrievingSignOutURLFromSession?: (config: SPATokenExchangeConfig) => void,
  ): Promise<HttpResponse> {
    return httpClient
      .request(requestConfig)
      .then((response: HttpResponse) => {
        return Promise.resolve(response);
      })
      .catch(async (error: HttpError) => {
        if (error?.response?.status === 401 || !error?.response) {
          if (this._isTokenRefreshing) {
            return this.retryFailedRequests({
              enableRetrievingSignOutURLFromSession,
              httpClient,
              httpErrorCallback,
              httpFinishCallback,
              isHttpHandlerEnabled,
              requestConfig,
            });
          }

          this._isTokenRefreshing = true;
          let refreshAccessTokenResponse: User;
          try {
            refreshAccessTokenResponse = await this.refreshAccessToken(enableRetrievingSignOutURLFromSession);
            this._isTokenRefreshing = false;
          } catch (refreshError: any) {
            this._isTokenRefreshing = false;

            if (isHttpHandlerEnabled) {
              if (typeof httpErrorCallback === 'function') {
                await httpErrorCallback({...error, code: ACCESS_TOKEN_INVALID});
              }
              if (typeof httpFinishCallback === 'function') {
                httpFinishCallback();
              }
            }

            throw new ThunderIDAuthException(
              'SPA-AUTH_HELPER-HR-SE01',
              refreshError?.name ?? 'Refresh token request failed.',
              refreshError?.message ?? 'An error occurred while trying to refresh the access token.',
            );
          }

          if (refreshAccessTokenResponse) {
            try {
              return await httpClient.request(requestConfig);
            } catch (error: any) {
              if (isHttpHandlerEnabled) {
                if (typeof httpErrorCallback === 'function') {
                  await httpErrorCallback(error);
                }
                if (typeof httpFinishCallback === 'function') {
                  httpFinishCallback();
                }
              }
              return Promise.reject(error);
            }
          }
        }

        if (isHttpHandlerEnabled) {
          if (typeof httpErrorCallback === 'function') {
            await httpErrorCallback(error);
          }
          if (typeof httpFinishCallback === 'function') {
            httpFinishCallback();
          }
        }

        return Promise.reject(error);
      });
  }

  /**
   * Sends multiple HTTP requests in parallel via the provided client,
   * retrying all on a 401 after a token refresh.
   *
   * @param requestConfigs - Array of request configurations.
   * @param httpClient - The HTTP client to use.
   * @param isHttpHandlerEnabled - Whether request callbacks are active.
   * @param httpErrorCallback - Called when a batch fails.
   * @param httpFinishCallback - Called when the batch finishes.
   * @returns Array of responses.
   */
  public async httpRequestAll(
    requestConfigs: HttpRequestConfig[],
    httpClient: HttpClient,
    isHttpHandlerEnabled?: boolean,
    httpErrorCallback?: (error: HttpError) => void | Promise<void>,
    httpFinishCallback?: () => void,
  ): Promise<HttpResponse[] | undefined> {
    const requests: Promise<HttpResponse<any>>[] = requestConfigs.map((req) => httpClient.request(req));

    return (
      httpClient?.all &&
      httpClient
        .all(requests)
        .then((responses: HttpResponse[]) => {
          return Promise.resolve(responses);
        })
        .catch(async (error: HttpError) => {
          if (error?.response?.status === 401 || !error?.response) {
            try {
              await this._refreshAccessToken();
            } catch (refreshError: any) {
              if (isHttpHandlerEnabled) {
                if (typeof httpErrorCallback === 'function') {
                  await httpErrorCallback({...error, code: ACCESS_TOKEN_INVALID});
                }
                if (typeof httpFinishCallback === 'function') {
                  httpFinishCallback();
                }
              }

              throw new ThunderIDAuthException(
                'SPA-AUTH_HELPER-HRA-SE01',
                refreshError?.name ?? 'Refresh token request failed.',
                refreshError?.message ?? 'An error occurred while trying to refresh the access token.',
              );
            }

            return (
              httpClient.all &&
              httpClient
                .all(requests)
                .then((response) => Promise.resolve(response))
                .catch(async (error) => {
                  if (isHttpHandlerEnabled) {
                    if (typeof httpErrorCallback === 'function') {
                      await httpErrorCallback(error);
                    }
                    if (typeof httpFinishCallback === 'function') {
                      httpFinishCallback();
                    }
                  }
                  return Promise.reject(error);
                })
            );
          }

          if (isHttpHandlerEnabled) {
            if (typeof httpErrorCallback === 'function') {
              await httpErrorCallback(error);
            }
            if (typeof httpFinishCallback === 'function') {
              httpFinishCallback();
            }
          }

          return Promise.reject(error);
        })
    );
  }

  /**
   * Executes the silent sign-in flow using a prompt-none request via an iFrame.
   *
   * @param constructSilentSignInUrl - Builds the prompt-none authorize URL.
   * @param requestAccessToken - Exchanges the returned code for tokens.
   * @param sessionManagementHelper - Handles the iFrame prompt-none response.
   * @param additionalParams - Extra authorize request params.
   * @param tokenRequestConfig - Additional params for the token request.
   * @returns The user session, or `false` if the user is not signed in.
   */
  public async signInSilently(
    constructSilentSignInUrl: (additionalParams?: Record<string, string | boolean>) => Promise<string>,
    requestAccessToken: (
      authzCode: string,
      sessionState: string,
      state: string,
      tokenRequestConfig?: {params: Record<string, unknown>},
    ) => Promise<User>,
    sessionManagementHelper: SessionManagementHelperInterface,
    additionalParams?: Record<string, string | boolean>,
    tokenRequestConfig?: {params: Record<string, unknown>},
  ): Promise<User | boolean> {
    if (SPAUtils.isInitializedSilentSignIn()) {
      await sessionManagementHelper.receivePromptNoneResponse();

      return Promise.resolve({
        allowedScopes: '',
        displayName: '',
        email: '',
        sessionState: '',
        sub: '',
        tenantDomain: '',
        username: '',
      });
    }

    const rpIFrame = document.getElementById(RP_IFRAME) as HTMLIFrameElement;
    const promptNoneIFrame: HTMLIFrameElement = rpIFrame?.contentDocument?.getElementById(
      PROMPT_NONE_IFRAME,
    ) as HTMLIFrameElement;

    try {
      const url = await constructSilentSignInUrl(additionalParams);
      promptNoneIFrame.src = url;
    } catch (error) {
      return Promise.reject(error);
    }

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        resolve(false);
      }, 10000);

      const listenToPromptNoneIFrame = async (e: MessageEvent) => {
        const data: Message<AuthorizationInfo | null> = e.data;

        if (data?.type == CHECK_SESSION_SIGNED_OUT) {
          window.removeEventListener('message', listenToPromptNoneIFrame);
          clearTimeout(timer);
          resolve(false);
        }

        if (data?.type == CHECK_SESSION_SIGNED_IN && (data?.data)?.code) {
          const authInfo = data.data;
          requestAccessToken(authInfo.code, authInfo.sessionState, authInfo.state, tokenRequestConfig)
            .then((response: User) => {
              window.removeEventListener('message', listenToPromptNoneIFrame);
              resolve(response);
            })
            .catch((error) => {
              window.removeEventListener('message', listenToPromptNoneIFrame);
              reject(error);
            })
            .finally(() => {
              clearTimeout(timer);
            });
        }
      };

      window.addEventListener('message', listenToPromptNoneIFrame);
    });
  }

  /**
   * Handles the early-return path of `signIn()` when a session already exists
   * or the page is handling a prompt-none response.
   *
   * @param shouldStopAuthn - Returns `true` if we should short-circuit and return early.
   * @param checkSession - Callback to initialize OIDC session management.
   * @returns The current user if already signed in, or `undefined` to continue normal sign-in.
   */
  public async handleSignIn(
    shouldStopAuthn: () => Promise<boolean>,
    checkSession: () => Promise<void>,
  ): Promise<User | undefined> {
    const config = await this._storageManager.getConfigData();
    const configAny = config as any;

    if (await shouldStopAuthn()) {
      return Promise.resolve({
        allowedScopes: '',
        displayName: '',
        email: '',
        sessionState: '',
        sub: '',
        tenantDomain: '',
        username: '',
      });
    }

    if (await this._isSignedIn()) {
      await this._spaHelper.clearRefreshTokenTimeout();
      await this._spaHelper.refreshAccessTokenAutomatically(() => this.refreshAccessToken());

      if (configAny.syncSession) {
        checkSession();
      }

      return Promise.resolve(await this._getUser());
    }

    const error = new URL(window.location.href).searchParams.get(ERROR);
    const errorDescription = new URL(window.location.href).searchParams.get(ERROR_DESCRIPTION);

    if (error) {
      const url = new URL(window.location.href);
      url.searchParams.delete(ERROR);
      url.searchParams.delete(ERROR_DESCRIPTION);

      history.pushState(null, document.title, url.toString());

      throw new ThunderIDAuthException('SPA-AUTH_HELPER-SI-SE01', error, errorDescription ?? '');
    }

    return Promise.resolve(undefined);
  }

  /**
   * Attaches the access token (or IDP token) to an HTTP request config's `Authorization` header.
   *
   * @param request - The request config to mutate.
   */
  public async attachTokenToRequestConfig(request: HttpRequestConfig): Promise<void> {
    const requestConfig = {attachToken: true, ...request};
    if (requestConfig.attachToken) {
      if (requestConfig.shouldAttachIDPAccessToken) {
        request.headers = {
          ...request.headers,
          Authorization: `Bearer ${await this._getIDPAccessToken()}`,
        };
      } else {
        request.headers = {
          ...request.headers,
          Authorization: `Bearer ${await this._getAccessToken()}`,
        };
      }
    }
  }

  /** Returns the current authenticated user from the ID token. */
  public async getUser(): Promise<User> {
    return this._getUser();
  }

  /**
   * Returns the decoded ID token payload.
   *
   * @param sessionId - Optional session ID.
   */
  public async getDecodedIdToken(sessionId?: string): Promise<IdToken> {
    return this._getDecodedIdToken(sessionId);
  }

  /** Returns the IsomorphicCrypto instance used by the client. */
  public async getCrypto(): Promise<IsomorphicCrypto> {
    return this._getCrypto();
  }

  /** Returns the raw ID token string. */
  public async getIdToken(): Promise<string> {
    return this._getIdToken();
  }

  /** Returns the resolved OIDC provider endpoints. */
  public async getOpenIDProviderEndpoints(): Promise<OIDCEndpoints> {
    return this._getOpenIDProviderEndpoints();
  }

  /**
   * Returns the current access token.
   *
   * @param sessionId - Optional session ID.
   */
  public async getAccessToken(sessionId?: string): Promise<string> {
    return this._getAccessToken(sessionId);
  }

  /** Returns the IDP access token from the session. */
  public async getIDPAccessToken(): Promise<string> {
    return (await this._storageManager.getSessionData())?.access_token;
  }

  /** Returns the storage manager. */
  public getStorageManager(): StorageManager<T> {
    return this._storageManager;
  }

  /** Returns whether the user is currently signed in. */
  public async isSignedIn(): Promise<boolean> {
    return this._isSignedIn();
  }
}

export default AuthenticationHelper;
