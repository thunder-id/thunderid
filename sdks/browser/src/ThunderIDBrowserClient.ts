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
  ThunderIDJavaScriptClient,
  ThunderIDAuthException,
  User,
  IsomorphicCrypto,
  IdToken,
  OIDCEndpoints,
  OIDCRequestConstants,
  SessionData,
  Storage,
  TokenResponse,
  extractPkceStorageKeyFromState,
  HttpError,
  HttpRequestConfig,
  HttpResponse,
  createPackageComponentLogger,
} from '@thunderid/javascript';
import {REFRESH_ACCESS_TOKEN_ERR0R, SILENT_SIGN_IN_STATE, TOKEN_REQUEST_CONFIG_KEY} from './constants/SPAConstants';
import FetchHttpClient from './FetchHttpClient';
import {BrowserAuthConfig} from './models/BrowserConfig';
import BrowserStorage from './models/BrowserStorage';
import SignInConfig from './models/SignInConfig';
import SignOutError from './models/SignOutError';
import {SPATokenExchangeConfig} from './models/TokenExchangeConfig';
import LocalStore from './stores/LocalStore';
import MemoryStore from './stores/MemoryStore';
import SessionStore from './stores/SessionStore';
import AuthenticationHelper from './utils/AuthenticationHelper';
import navigate from './utils/navigate';
import createSessionManagementHelper, {SessionManagementHelperInterface} from './utils/SessionManagementHelper';
import SPACryptoUtils from './utils/SPACryptoUtils';
import SPAHelper from './utils/SPAHelper';
import SPAUtils from './utils/SPAUtils';

const logger = createPackageComponentLogger('@thunderid/browser', 'ThunderIDBrowserClient');

const BROWSER_DEFAULT_CONFIG = {
  autoLogoutOnTokenRefreshError: false,
  checkSessionInterval: 3,
  periodicTokenRefresh: false,
  sessionRefreshInterval: 300,
  syncSession: false,
};

const initiateStore = (storage: BrowserStorage | string | undefined): Storage => {
  switch (storage) {
    case BrowserStorage.LocalStorage:
    case 'localStorage':
      return new LocalStore();
    case BrowserStorage.BrowserMemory:
    case 'browserMemory':
      return new MemoryStore();
    default:
      return new SessionStore();
  }
};

class ThunderIDBrowserClient<T = BrowserAuthConfig> extends ThunderIDJavaScriptClient<T> {
  private _browserInstanceId = 0;
  private _httpClient: FetchHttpClient | undefined;
  private _spaHelper: SPAHelper<T> | undefined;
  private _sessionManagementHelper: SessionManagementHelperInterface | undefined;
  private _authHelper: AuthenticationHelper<T> | undefined;
  private _storage: BrowserStorage | string = BrowserStorage.SessionStorage;
  private _getSignOutURLFromSessionStorage = false;
  private _isHttpHandlerEnabled = true;
  private _httpErrorCallback: ((error: HttpError) => void | Promise<void>) | undefined;
  private _httpFinishCallback: (() => void) | undefined;
  private _onSignInCallback: (response: User) => void = () => null;
  private _onSignOutCallback: () => void = () => null;
  private _onSignOutFailedCallback: (error: SignOutError) => void = () => null;
  private _onEndUserSession: (response: any) => void = () => null;
  private _onInitialize: (response: boolean) => void = () => null;
  private _onCustomGrant = new Map<string, (response: any) => void>();
  private _initialized = false;
  private _startedInitialize = false;

  public constructor(instanceId = 0) {
    super(undefined, new SPACryptoUtils());
    this._browserInstanceId = instanceId;
  }

  public override async initialize(config: T, storage?: Storage): Promise<boolean> {
    this._startedInitialize = true;
    this._initialized = false;

    const configAny = config as any;
    this._storage = configAny.storage ?? BrowserStorage.SessionStorage;

    const merged = {
      afterSignInUrl: window.location.origin,
      afterSignOutUrl: window.location.origin,
      ...BROWSER_DEFAULT_CONFIG,
      ...configAny,
      instanceId: this._browserInstanceId,
    };
    const store: Storage = storage ?? initiateStore(this._storage as BrowserStorage);

    await super.initialize(merged as unknown as T, store);

    const sm = this.getStorageManager();
    this._spaHelper = new SPAHelper<T>(sm);

    const attachToken = async (request: HttpRequestConfig): Promise<void> => {
      await this._authHelper?.attachTokenToRequestConfig(request);
    };

    this._httpClient = FetchHttpClient.getInstance(this._browserInstanceId, true, attachToken);

    this._sessionManagementHelper = await createSessionManagementHelper(
      async () => this.getSignOutUrl(),
      (sessionState: string) =>
        (sm as any).setSessionDataParameter(
          OIDCRequestConstants.Params.SESSION_STATE as keyof SessionData,
          sessionState,
        ),
    );

    this._authHelper = new AuthenticationHelper<T>(sm, this._spaHelper, this._browserInstanceId, {
      exchangeToken: (cfg: SPATokenExchangeConfig) => super.exchangeToken(cfg as any),
      getAccessToken: (sessionId?: string) => this.getAccessToken(sessionId),
      getCrypto: async () => this.getCryptoHelper() as unknown as IsomorphicCrypto,
      getDecodedIdToken: (sessionId?: string) => this.getDecodedIdToken(sessionId),
      getIDPAccessToken: async () => ((await sm.getSessionData()) as any)?.access_token,
      getIdToken: () => this.getIdToken(),
      getOpenIDProviderEndpoints: async () => this.getOpenIDProviderEndpoints() as Promise<OIDCEndpoints>,
      getUser: () => this.getUser(),
      isSignedIn: () => this.isSignedIn(),
      refreshAccessToken: () => super.refreshAccessToken(),
      setPKCECode: (pkceKey: string, state: string) => this.setPKCECode(pkceKey, state),
    });

    this._initialized = true;

    if (this._onInitialize) {
      this._onInitialize(true);
    }

    if (!merged.autoLogoutOnTokenRefreshError) {
      return true;
    }

    window.addEventListener('message', (event) => {
      if (event?.data?.type === REFRESH_ACCESS_TOKEN_ERR0R) {
        this.signOut();
      }
    });

    return true;
  }

  public async isInitialized(): Promise<boolean> {
    if (!this._startedInitialize) {
      return false;
    }

    const sleep = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 1));
    let iterations = 0;

    while (!this._initialized) {
      if (iterations === 1e4) {
        logger.warn('Initialization is taking longer than expected');
      }
      await sleep();
      iterations++;
    }

    return true;
  }

  private async _validateMethod(validateAuthentication = true): Promise<boolean> {
    if (!(await this.isInitialized())) {
      return Promise.reject(
        new ThunderIDAuthException(
          'SPA-AUTH_CLIENT-VM-NF01',
          'The SDK is not initialized.',
          'The SDK must be initialized first.',
        ),
      );
    }

    if (validateAuthentication && !(await this.isSignedIn())) {
      return Promise.reject(
        new ThunderIDAuthException(
          'SPA-AUTH_CLIENT-VM-IV02',
          'The user is not authenticated.',
          'The user must be authenticated first.',
        ),
      );
    }

    return true;
  }

  public override isLoading(): boolean {
    return false;
  }

  public override async signIn(
    config?: SignInConfig,
    authorizationCode?: string,
    sessionState?: string,
    state?: string,
    tokenRequestConfig?: {params: Record<string, unknown>},
  ): Promise<User | undefined> {
    await this.isInitialized();

    if (!SPAUtils.canContinueSignIn(Boolean(config?.callOnlyOnRedirect), authorizationCode)) {
      return undefined;
    }

    delete config?.callOnlyOnRedirect;

    const user = await this._signInInternal(config, authorizationCode, sessionState, state, tokenRequestConfig);

    if (user && this._onSignInCallback) {
      this._onSignInCallback(user);
    }

    return user;
  }

  private async _signInInternal(
    signInConfig?: SignInConfig,
    authorizationCode?: string,
    sessionState?: string,
    state?: string,
    tokenRequestConfig?: {params: Record<string, unknown>},
  ): Promise<User> {
    const sm = this.getStorageManager();
    const config = await (sm as any).getConfigData();

    const basicUserInfo = await this._authHelper!.handleSignIn(
      () => this._shouldStopAuthn(),
      () => this._checkSession(),
    );

    if (basicUserInfo) {
      return basicUserInfo;
    }

    let resolvedAuthorizationCode: string;
    let resolvedSessionState: string;
    let resolvedState: string;
    let resolvedTokenRequestConfig: {params: Record<string, unknown>} = {params: {}};

    if (config?.responseMode === 'form_post' && authorizationCode) {
      resolvedAuthorizationCode = authorizationCode;
      resolvedSessionState = sessionState ?? '';
      resolvedState = state ?? '';
    } else {
      resolvedAuthorizationCode =
        new URL(window.location.href).searchParams.get(OIDCRequestConstants.Params.AUTHORIZATION_CODE) ?? '';
      resolvedSessionState =
        new URL(window.location.href).searchParams.get(OIDCRequestConstants.Params.SESSION_STATE) ?? '';
      resolvedState = new URL(window.location.href).searchParams.get(OIDCRequestConstants.Params.STATE) ?? '';

      SPAUtils.removeAuthorizationCode();
    }

    if (resolvedAuthorizationCode && resolvedState) {
      (sm as any).setSessionStatus('true');
      const storedTokenRequestConfig = await (sm as any).getTemporaryDataParameter(TOKEN_REQUEST_CONFIG_KEY);
      if (storedTokenRequestConfig && typeof storedTokenRequestConfig === 'string') {
        resolvedTokenRequestConfig = JSON.parse(storedTokenRequestConfig);
      }
      return this._exchangeCodeForTokens(
        resolvedAuthorizationCode,
        resolvedSessionState,
        resolvedState,
        resolvedTokenRequestConfig,
      );
    }

    return this.getSignInUrl(signInConfig as any).then(async (url: string) => {
      if (this._storage === BrowserStorage.BrowserMemory && config.enablePKCE) {
        const pkceKey: string = extractPkceStorageKeyFromState(resolvedState);
        SPAUtils.setPKCE(pkceKey, await this.getPKCECode(resolvedState));
      }

      if (tokenRequestConfig) {
        (sm as any).setTemporaryDataParameter(TOKEN_REQUEST_CONFIG_KEY, JSON.stringify(tokenRequestConfig));
      }

      location.href = url;
      await SPAUtils.waitTillPageRedirect();

      return {allowedScopes: '', displayName: '', email: '', sessionState: '', sub: '', tenantDomain: '', username: ''};
    });
  }

  private async _exchangeCodeForTokens(
    authorizationCode: string,
    sessionState: string,
    state: string,
    tokenRequestConfig?: {params: Record<string, unknown>},
  ): Promise<User> {
    const sm = this.getStorageManager();
    const config = await (sm as any).getConfigData();

    if (this._storage === BrowserStorage.BrowserMemory && config.enablePKCE && sessionState) {
      const pkceKey = extractPkceStorageKeyFromState(sessionState);
      const pkce = SPAUtils.getPKCE(pkceKey);
      await this.setPKCECode(pkce, pkceKey);
    }

    await this.requestAccessToken(authorizationCode, sessionState ?? '', state ?? '', undefined, tokenRequestConfig);

    try {
      const signOutUrl = await this.getSignOutUrl();
      SPAUtils.setSignOutURL(signOutUrl, config.clientId, this._browserInstanceId);
    } catch {
      // end_session_endpoint absent — signOut() falls back to signInUrl navigation.
    }

    await this._spaHelper!.clearRefreshTokenTimeout();
    await this._spaHelper!.refreshAccessTokenAutomatically(() => this.refreshAccessToken() as any);

    if (config.syncSession) {
      this._checkSession();
    }

    return this.getUser();
  }

  public override async signInSilently(
    additionalParams?: Record<string, string | boolean>,
    tokenRequestConfig?: {params: Record<string, unknown>},
  ): Promise<User | boolean | undefined> {
    await this.isInitialized();

    if (SPAUtils.wasSignInCalled()) {
      return undefined;
    }

    const response = await this._authHelper!.signInSilently(
      (params) => this._constructSilentSignInUrl(params),
      (code, ss, s, trc) => this._exchangeCodeForTokens(code, ss, s, trc),
      this._sessionManagementHelper!,
      additionalParams,
      tokenRequestConfig,
    );

    if (this._onSignInCallback && response) {
      this._onSignInCallback(response as User);
    }

    return response;
  }

  public override async signOut(
    _options?: any,
    sessionIdOrAfterSignOut?: string | ((url: string) => void),
    afterSignOutParam?: (url: string) => void,
  ): Promise<string> {
    let afterSignOut: ((url: string) => void) | undefined;

    if (typeof sessionIdOrAfterSignOut === 'function') {
      afterSignOut = sessionIdOrAfterSignOut;
    } else if (typeof sessionIdOrAfterSignOut === 'string') {
      afterSignOut = afterSignOutParam;
    }

    const sm = this.getStorageManager();
    const config = await (sm as any).getConfigData();

    // TEMPORARY: Handle sign-out by clearing the session and navigating back to sign-in,
    // until the OIDC end-session flow is fully supported.
    this.clearSession();

    if (config?.signInUrl) {
      navigate(config.signInUrl);
    } else {
      this.signIn(config?.signInOptions);
    }

    afterSignOut?.(config?.afterSignOutUrl || '');

    return config?.afterSignOutUrl || '';
  }

  public async httpRequest(requestConfig: HttpRequestConfig): Promise<HttpResponse | undefined> {
    await this._validateMethod(false);

    return this._authHelper!.httpRequest(
      this._httpClient!,
      requestConfig,
      this._isHttpHandlerEnabled,
      this._httpErrorCallback,
      this._httpFinishCallback,
    );
  }

  public async httpRequestAll(configs: HttpRequestConfig[]): Promise<HttpResponse[] | undefined> {
    await this._validateMethod(false);

    return this._authHelper!.httpRequestAll(
      configs,
      this._httpClient!,
      this._isHttpHandlerEnabled,
      this._httpErrorCallback,
      this._httpFinishCallback,
    );
  }

  public override async getUser(userId?: string): Promise<User> {
    await this._validateMethod();

    return super.getUser(userId);
  }

  public override async getAccessToken(sessionId?: string): Promise<string> {
    return super.getAccessToken(sessionId);
  }

  public override async getDecodedIdToken(userId?: string, idToken?: string): Promise<IdToken> {
    await this._validateMethod();

    return super.getDecodedIdToken(userId, idToken);
  }

  public override async getIdToken(userId?: string): Promise<string> {
    await this._validateMethod();

    return super.getIdToken(userId);
  }

  public override async getOpenIDProviderEndpoints(): Promise<Partial<OIDCEndpoints>> {
    await this.isInitialized();

    return super.getOpenIDProviderEndpoints();
  }

  public async getCrypto(): Promise<IsomorphicCrypto | undefined> {
    await this._validateMethod();

    return this.getCryptoHelper() as any;
  }

  public getHttpClient(): FetchHttpClient {
    if (!this._httpClient) {
      throw new ThunderIDAuthException(
        'SPA-AUTH_CLIENT-GHC-NF02',
        'The SDK is not initialized.',
        'Call initialize() before getHttpClient().',
      );
    }

    return this._httpClient;
  }

  public override async isSignedIn(userId?: string): Promise<boolean> {
    await this.isInitialized();

    return super.isSignedIn(userId);
  }

  protected notifySignIn(user: User): void {
    this._onSignInCallback(user);
  }

  public async isSessionActive(): Promise<boolean | undefined> {
    await this.isInitialized();

    return (await (this.getStorageManager() as any).getSessionStatus()) === 'true';
  }

  public override async refreshAccessToken(userId?: string): Promise<User | TokenResponse> {
    await this._validateMethod(false);

    try {
      return await this._authHelper!.refreshAccessToken((cfg) => this._enableRetrievingSignOutURLFromSession(cfg));
    } catch (error) {
      return Promise.reject(error);
    }
  }

  public override async revokeAccessToken(userId?: string): Promise<boolean | Response> {
    await this._validateMethod();

    const timer: number = await this._spaHelper!.getRefreshTimeoutTimer();
    await super.revokeAccessToken(userId);

    this._sessionManagementHelper?.reset();
    await this._spaHelper!.clearRefreshTokenTimeout(timer);

    this._onEndUserSession && (await this._onEndUserSession(true));

    return true;
  }

  public override async exchangeToken(config: SPATokenExchangeConfig): Promise<Response | User> {
    if (config.signInRequired) {
      await this._validateMethod();
    }

    if (!config.id) {
      return Promise.reject(
        new ThunderIDAuthException(
          'SPA-AUTH_CLIENT-RCG-NF01',
          'The custom grant request id not found.',
          'Set the `id` attribute on the token exchange config.',
        ),
      );
    }

    const response = await this._authHelper!.exchangeToken(config, (cfg) =>
      this._enableRetrievingSignOutURLFromSession(cfg),
    );

    const cb = this._onCustomGrant.get(config.id);
    cb?.(response);

    return response;
  }

  public override async reInitialize(config: Partial<T>): Promise<boolean> {
    await this.isInitialized();

    const sm = this.getStorageManager();
    const existingConfig = await (sm as any).getConfigData();
    const isCheckSessionIframeDifferent = !(
      existingConfig?.endpoints?.checkSessionIframe &&
      (config as any)?.endpoints?.checkSessionIframe &&
      existingConfig.endpoints.checkSessionIframe === (config as any).endpoints.checkSessionIframe
    );
    const result = await super.reInitialize(config);

    const merged = {...existingConfig, ...config};
    if (merged.syncSession && isCheckSessionIframeDifferent) {
      this._sessionManagementHelper?.reset();
      this._checkSession();
    }

    return result;
  }

  public async startAutoRefreshToken(): Promise<void> {
    await this.isInitialized();

    await this._spaHelper!.clearRefreshTokenTimeout();
    await this._spaHelper!.refreshAccessTokenAutomatically(() => this.refreshAccessToken() as any);
  }

  public async enableHttpHandler(): Promise<boolean | undefined> {
    await this.isInitialized();

    this._authHelper?.enableHttpHandler(this._httpClient!);
    this._isHttpHandlerEnabled = true;

    return true;
  }

  public async disableHttpHandler(): Promise<boolean | undefined> {
    await this.isInitialized();

    this._authHelper?.disableHttpHandler(this._httpClient!);
    this._isHttpHandlerEnabled = false;

    return true;
  }

  public override async decodeJwtToken<R = Record<string, unknown>>(token: string): Promise<R> {
    return this.getCryptoHelper().decodeJwtToken<R>(token);
  }

  public async on(hook: string, callback: (response?: any) => void | Promise<void>, id?: string): Promise<void> {
    await this.isInitialized();

    if (!callback || typeof callback !== 'function') {
      throw new ThunderIDAuthException(
        'SPA-AUTH_CLIENT-ON-IV02',
        'Invalid callback function.',
        'The provided callback must be a function.',
      );
    }

    switch (hook) {
      case 'sign-in':
        this._onSignInCallback = callback;
        break;
      case 'sign-out':
        this._onSignOutCallback = callback;
        if (
          await SPAUtils.isSignOutSuccessful(
            ThunderIDBrowserClient.isSignOutSuccessful.bind(ThunderIDBrowserClient),
            () => ThunderIDBrowserClient.clearSession(),
          )
        ) {
          this._onSignOutCallback();
        }
        break;
      case 'revoke-access-token':
        this._onEndUserSession = callback;
        break;
      case 'initialize':
        this._onInitialize = callback;
        break;
      case 'http-request-error':
        this._httpErrorCallback = callback;
        break;
      case 'http-request-finish':
        this._httpFinishCallback = callback;
        break;
      case 'http-request-start':
        this._httpClient?.setHttpRequestStartCallback?.(callback);
        break;
      case 'http-request-success':
        this._httpClient?.setHttpRequestSuccessCallback?.(callback);
        break;
      case 'custom-grant':
        id && this._onCustomGrant.set(id, callback);
        break;
      case 'sign-out-failed': {
        this._onSignOutFailedCallback = callback;
        const signOutFail = SPAUtils.didSignOutFail(ThunderIDBrowserClient.didSignOutFail.bind(ThunderIDBrowserClient));
        if (signOutFail) {
          this._onSignOutFailedCallback(signOutFail as SignOutError);
        }
        break;
      }
      default:
        throw new ThunderIDAuthException('SPA-AUTH_CLIENT-ON-IV01', 'Invalid hook.', `Unknown hook: "${hook}"`);
    }
  }

  private async _checkSession(): Promise<void> {
    const oidcEndpoints = (await this.getOpenIDProviderEndpoints()) as OIDCEndpoints;
    const sm = this.getStorageManager();
    const config = await (sm as any).getConfigData();

    this._authHelper!.initializeSessionManger(
      config,
      oidcEndpoints,
      async () => (await (sm as any).getSessionData())?.session_state ?? '',
      async (params?: any): Promise<string> => this.getSignInUrl(params),
      this._sessionManagementHelper!,
    );
  }

  private async _shouldStopAuthn(): Promise<boolean> {
    const sm = this.getStorageManager();

    return this._sessionManagementHelper!.receivePromptNoneResponse(async (sessionState: string | null) => {
      await (sm as any).setSessionDataParameter(
        OIDCRequestConstants.Params.SESSION_STATE as keyof SessionData,
        sessionState ?? '',
      );
    });
  }

  private _enableRetrievingSignOutURLFromSession(config: SPATokenExchangeConfig): void {
    if (config.preventSignOutURLUpdate) {
      this._getSignOutURLFromSessionStorage = true;
    }
  }

  private async _constructSilentSignInUrl(additionalParams: Record<string, string | boolean> = {}): Promise<string> {
    const sm = this.getStorageManager();
    const config = await (sm as any).getConfigData();

    const urlString: string = await this.getSignInUrl({
      prompt: 'none',
      state: SILENT_SIGN_IN_STATE,
      ...additionalParams,
    } as any);

    const urlObject = new URL(urlString);
    urlObject.searchParams.set('response_mode', 'query');

    if (this._storage === BrowserStorage.BrowserMemory && config.enablePKCE) {
      const state = urlObject.searchParams.get(OIDCRequestConstants.Params.STATE);
      SPAUtils.setPKCE(extractPkceStorageKeyFromState(state ?? ''), await this.getPKCECode(state ?? ''));
    }

    return urlObject.toString();
  }

  public override getInstanceId(): number {
    return this._browserInstanceId;
  }

  public static async clearSession(): Promise<void> {
    // Static clear: no-op at base level; session data is cleared by instance signOut
  }
}

export default ThunderIDBrowserClient;
