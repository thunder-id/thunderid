/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import executeEmbeddedSignInFlow from './api/executeEmbeddedSignInFlow';
import initializeEmbeddedSignInFlow from './api/initializeEmbeddedSignInFlow';
import OIDCDiscoveryConstants from './constants/OIDCDiscoveryConstants';
import OIDCRequestConstants from './constants/OIDCRequestConstants';
import PKCEConstants from './constants/PKCEConstants';
import {DefaultCacheStore} from './DefaultCacheStore';
import {DefaultCrypto} from './DefaultCrypto';
import {ThunderIDAuthException} from './errors/exception';
import {IsomorphicCrypto} from './IsomorphicCrypto';
import {AgentConfig} from './models/agent';
import {AuthCodeResponse} from './models/auth-code-response';
import {ThunderIDClient} from './models/client';
import {AuthClientConfig, Config, SignInOptions, SignOutOptions, SignUpOptions} from './models/config';
import {Crypto} from './models/crypto';
import {
  EmbeddedFlowExecuteRequestConfig,
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
} from './models/embedded-flow';
import {
  EmbeddedSignInFlowAuthenticator,
  EmbeddedSignInFlowHandleResponse,
  EmbeddedSignInFlowInitiateResponse,
  EmbeddedSignInFlowStatus,
} from './models/embedded-signin-flow';
import {ExtendedAuthorizeRequestUrlParams} from './models/oauth-request';
import {OIDCDiscoveryApiResponse} from './models/oidc-discovery';
import {OIDCEndpoints} from './models/oidc-endpoints';
import {AllOrganizationsApiResponse, Organization} from './models/organization';
import {SessionData, UserSession} from './models/session';
import {Storage, TemporaryStore} from './models/store';
import {IdToken, TokenExchangeRequestConfig, TokenResponse} from './models/token';
import {User, UserProfile} from './models/user';
import StorageManager from './StorageManager';
import AuthenticationHelper from './utils/AuthenticationHelper';
import base64Encode from './utils/base64Encode';
import deepMerge from './utils/deepMerge';
import extractPkceStorageKeyFromState from './utils/extractPkceStorageKeyFromState';
import generatePkceStorageKey from './utils/generatePkceStorageKey';
import getAuthorizeRequestUrlParams from './utils/getAuthorizeRequestUrlParams';
import processOpenIDScopes from './utils/processOpenIDScopes';

const WELL_KNOWN_PATH = '/.well-known/openid-configuration';

const DEFAULT_CONFIG: Partial<AuthClientConfig<unknown>> = {
  enablePKCE: true,
  responseMode: 'query',
  sendCookiesInRequests: true,
  tokenValidation: {
    idToken: {
      clockTolerance: 300,
      validate: true,
      validateIssuer: true,
    },
  },
};

class ThunderIDJavaScriptClient<T = Config> implements ThunderIDClient<T> {
  protected storageManager!: StorageManager<T>;

  protected cryptoUtils: Crypto;

  private configProvider!: () => Promise<AuthClientConfig<T>>;

  private oidcProviderMetaDataProvider!: () => Promise<OIDCDiscoveryApiResponse>;

  private authHelper!: AuthenticationHelper<T>;

  private cryptoHelper!: IsomorphicCrypto;

  private instanceIdValue = 0;

  private cacheStore: Storage;

  private baseURL = '';

  constructor(storage?: Storage, cryptoUtils?: Crypto) {
    this.cacheStore = storage ?? new DefaultCacheStore();
    this.cryptoUtils = cryptoUtils ?? new DefaultCrypto();
  }

  // ─── ThunderIDClient interface ───────────────────────────────────────────────

  public async initialize(config: T, storage?: Storage): Promise<boolean> {
    const store = storage ?? this.cacheStore;
    const fullConfig = config as unknown as AuthClientConfig<T>;
    const {clientId, instanceId} = fullConfig as any;

    if (instanceId !== undefined) {
      this.instanceIdValue = instanceId;
    }

    const storageKey = clientId ? `instance_${this.instanceIdValue}-${clientId}` : `instance_${this.instanceIdValue}`;

    this.storageManager = new StorageManager<T>(storageKey, store);
    this.cryptoHelper = new IsomorphicCrypto(this.cryptoUtils);
    this.authHelper = new AuthenticationHelper(this.storageManager, this.cryptoHelper);
    this.configProvider = async (): Promise<AuthClientConfig<T>> => this.storageManager.getConfigData();
    this.oidcProviderMetaDataProvider = async (): Promise<OIDCDiscoveryApiResponse> =>
      this.storageManager.loadOpenIDProviderConfiguration();

    const {applicationId, endpoints} = fullConfig as any;
    let resolvedApplicationId: string | undefined = applicationId;

    if (applicationId) {
      await this.storageManager.setPersistedData({applicationId});
    } else {
      const persistedData: TemporaryStore = await this.storageManager.getPersistedData();
      if (persistedData?.['applicationId']) {
        resolvedApplicationId = persistedData['applicationId'] as string;
      }
    }

    const resolvedEndpoints = endpoints ? {...endpoints} : {};

    await this.storageManager.setConfigData(
      deepMerge({} as Record<string, any>, DEFAULT_CONFIG, fullConfig as Record<string, any>, {
        applicationId: resolvedApplicationId,
        endpoints: resolvedEndpoints,
        scope: processOpenIDScopes((fullConfig as any).scopes),
      }) as any,
    );

    this.baseURL = (fullConfig as any).baseUrl ?? '';

    return true;
  }

  public async reInitialize(config: Partial<T>): Promise<boolean> {
    const currentConfig = (await this.storageManager.getConfigData()) as unknown as AuthClientConfig<T>;
    const newConfig = deepMerge(currentConfig, config);

    await this.storageManager.setConfigData(newConfig);
    await this.loadOpenIDProviderConfiguration(true);

    return true;
  }

  public getConfiguration(): T {
    return this.storageManager.getConfigData() as unknown as T;
  }

  public async getUser(userId?: string): Promise<User> {
    const sessionData: SessionData = await this.storageManager.getSessionData(userId);
    const authenticatedUser: User = this.authHelper.getAuthenticatedUserInfo(sessionData?.id_token);

    Object.keys(authenticatedUser).forEach((key: string) => {
      if (authenticatedUser[key] === undefined || authenticatedUser[key] === '' || authenticatedUser[key] === null) {
        delete authenticatedUser[key];
      }
    });

    return authenticatedUser;
  }

  public async isSignedIn(userId?: string): Promise<boolean> {
    const hasToken = Boolean(await this.getAccessToken(userId));

    if (!hasToken) {
      return false;
    }

    const sessionData = await this.storageManager.getSessionData(userId);
    const createdAt: number = sessionData?.created_at;
    const expiresInString: string = sessionData?.expires_in;

    if (!expiresInString) {
      return false;
    }

    const expiresIn: number = parseInt(expiresInString, 10) * 1000;

    return createdAt + expiresIn > new Date().getTime();
  }

  public async getAccessToken(sessionId?: string): Promise<string> {
    return (await this.storageManager.getSessionData(sessionId))?.access_token;
  }

  public clearSession(sessionId?: string): void {
    this.authHelper.clearSession(sessionId);
  }

  public async setSession(sessionData: Record<string, unknown>, sessionId?: string): Promise<void> {
    await this.storageManager.setSessionData(sessionData, sessionId);
  }

  public async decodeJwtToken<R = Record<string, unknown>>(token: string): Promise<R> {
    return this.cryptoHelper.decodeJwtToken<R>(token);
  }

  public async exchangeToken(
    config: TokenExchangeRequestConfig,
    sessionId?: string,
  ): Promise<TokenResponse | Response | User> {
    if (
      !(await this.storageManager.getTemporaryDataParameter(
        OIDCDiscoveryConstants.Storage.StorageKeys.OPENID_PROVIDER_CONFIG_INITIATED,
      ))
    ) {
      await this.loadOpenIDProviderConfiguration(false);
    }

    const oidcProviderMetadata: OIDCDiscoveryApiResponse = await this.oidcProviderMetaDataProvider();
    const configData = await this.configProvider();

    let tokenEndpoint: string | undefined;

    if (config.tokenEndpoint && config.tokenEndpoint.trim().length !== 0) {
      tokenEndpoint = config.tokenEndpoint;
    } else {
      tokenEndpoint = oidcProviderMetadata.token_endpoint;
    }

    if (!tokenEndpoint || tokenEndpoint.trim().length === 0) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RCG-NF01',
        'Token endpoint not found.',
        'No token endpoint was found in the OIDC provider meta data returned by the well-known endpoint ' +
          'or the token endpoint passed to the SDK is empty.',
      );
    }

    const data: string[] = await Promise.all(
      Object.entries(config.data).map(async ([key, value]: [key: string, value: any]) => {
        const newValue: string = await this.authHelper.replaceCustomGrantTemplateTags(value as string, sessionId);

        return `${key}=${newValue}`;
      }),
    );

    let requestHeaders: Record<string, any> = {
      Accept: 'application/json',
      'Content-Type': 'application/x-www-form-urlencoded',
    };

    if (config.attachToken) {
      requestHeaders = {
        ...requestHeaders,
        Authorization: `Bearer ${(await this.storageManager.getSessionData(sessionId)).access_token}`,
      };
    }

    const requestConfig: RequestInit = {
      body: data.join('&'),
      credentials: configData.sendCookiesInRequests ? 'include' : 'same-origin',
      headers: new Headers(requestHeaders),
      method: 'POST',
    };

    let response: Response;

    try {
      response = await fetch(tokenEndpoint, requestConfig);
    } catch (error: any) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RCG-NE02',
        'The custom grant request failed.',
        error ?? 'The request sent to get the custom grant failed.',
      );
    }

    if (response.status !== 200 || !response.ok) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RCG-HE03',
        `Invalid response status received for the custom grant request. (${response.statusText})`,
        (await response.json()) as string,
      );
    }

    if (config.returnsSession) {
      return this.authHelper.handleTokenResponse(response, sessionId);
    }

    return (await response.json()) as TokenResponse | Response;
  }

  // ─── Platform-specific methods (must be overridden by subclasses) ────────────

  public isLoading(): boolean {
    throw new Error('Method not implemented.');
  }

  public signIn(_options?: SignInOptions): Promise<User | TokenResponse | undefined> {
    throw new Error('Method not implemented.');
  }

  public signOut(
    _options?: SignOutOptions,
    _sessionIdOrAfterSignOut?: string | ((afterSignOutUrl: string) => void),
    _afterSignOut?: (afterSignOutUrl: string) => void,
  ): Promise<string | boolean> {
    throw new Error('Method not implemented.');
  }

  public signInSilently(_options?: SignInOptions): Promise<User | boolean | undefined> {
    throw new Error('Method not implemented.');
  }

  public signUp(options?: SignUpOptions): Promise<void>;
  public signUp(payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse>;
  public signUp(
    _optionsOrPayload?: SignUpOptions | EmbeddedFlowExecuteRequestPayload,
  ): Promise<void | EmbeddedFlowExecuteResponse> {
    throw new Error('Method not implemented.');
  }

  public recover(_payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> {
    throw new Error('Method not implemented.');
  }

  public switchOrganization(_organization: Organization, _sessionId?: string): Promise<TokenResponse | Response> {
    throw new Error('Method not implemented.');
  }

  public getCurrentOrganization(_sessionId?: string): Promise<Organization | null> {
    throw new Error('Method not implemented.');
  }

  public getAllOrganizations(_options?: any, _sessionId?: string): Promise<AllOrganizationsApiResponse> {
    throw new Error('Method not implemented.');
  }

  public getMyOrganizations(_options?: any, _sessionId?: string): Promise<Organization[]> {
    throw new Error('Method not implemented.');
  }

  public getUserProfile(_options?: any): Promise<UserProfile> {
    throw new Error('Method not implemented.');
  }

  public updateUserProfile(_payload: any, _userId?: string): Promise<User> {
    throw new Error('Method not implemented.');
  }

  // ─── OIDC core (protected, used by subclasses) ──────────────────────────────

  protected async loadOpenIDProviderConfiguration(forceInit = false): Promise<void> {
    const configData = await this.configProvider();

    if (
      !forceInit &&
      (await this.storageManager.getTemporaryDataParameter(
        OIDCDiscoveryConstants.Storage.StorageKeys.OPENID_PROVIDER_CONFIG_INITIATED,
      ))
    ) {
      return;
    }

    const {discovery, baseUrl, endpoints} = configData as any;
    const wellKnownOverride: string | undefined = endpoints?.wellKnown;

    // Discovery is enabled by default; only skip if explicitly disabled.
    const resolvedWellKnownEndpoint: string | undefined =
      wellKnownOverride ||
      (discovery?.wellKnown?.enabled !== false && baseUrl ? `${baseUrl}${WELL_KNOWN_PATH}` : undefined);

    if (resolvedWellKnownEndpoint) {
      let response: Response;

      try {
        response = await fetch(resolvedWellKnownEndpoint);
        if (response.status !== 200 || !response.ok) {
          throw new Error();
        }
      } catch {
        throw new ThunderIDAuthException(
          'JS-AUTH_CORE-GOPMD-HE01',
          'Invalid well-known response',
          'The well known endpoint response has been failed with an error.',
        );
      }

      await this.storageManager.setOIDCProviderMetaData(await this.authHelper.resolveEndpoints(await response.json()));
    } else if (baseUrl) {
      try {
        await this.storageManager.setOIDCProviderMetaData(await this.authHelper.resolveEndpointsByBaseURL());
      } catch (error: any) {
        throw new ThunderIDAuthException(
          'JS-AUTH_CORE-GOPMD-IV02',
          'Resolving endpoints failed.',
          error ?? 'Resolving endpoints by base url failed.',
        );
      }
    } else {
      await this.storageManager.setOIDCProviderMetaData(await this.authHelper.resolveEndpointsExplicitly());
    }

    await this.storageManager.setTemporaryDataParameter(
      OIDCDiscoveryConstants.Storage.StorageKeys.OPENID_PROVIDER_CONFIG_INITIATED,
      true,
    );
  }

  protected async getSignInUrl(requestConfig?: ExtendedAuthorizeRequestUrlParams, userId?: string): Promise<string> {
    const authRequestConfig: ExtendedAuthorizeRequestUrlParams = {...requestConfig};

    delete authRequestConfig?.forceInit;

    const buildSignInUrl = async (): Promise<string> => {
      const authorizeEndpoint: string = (await this.storageManager.getOIDCProviderMetaDataParameter(
        OIDCDiscoveryConstants.Storage.StorageKeys.Endpoints.AUTHORIZATION as keyof OIDCDiscoveryApiResponse,
      )) as string;

      if (!authorizeEndpoint || authorizeEndpoint.trim().length === 0) {
        throw new ThunderIDAuthException(
          'JS-AUTH_CORE-GAU-NF01',
          'No authorization endpoint found.',
          'No authorization endpoint was found in the OIDC provider meta data from the well-known endpoint ' +
            'or the authorization endpoint passed to the SDK is empty.',
        );
      }

      const authorizeRequest: URL = new URL(authorizeEndpoint);
      const configData = await this.configProvider();
      const tempStore: TemporaryStore = await this.storageManager.getTemporaryData(userId);
      const pkceKey: string = await generatePkceStorageKey(tempStore);

      let codeVerifier: string | undefined;
      let codeChallenge: string | undefined;

      if (configData.enablePKCE) {
        codeVerifier = this.cryptoHelper?.getCodeVerifier();
        codeChallenge = await this.cryptoHelper?.getCodeChallenge(codeVerifier);
        await this.storageManager.setHybridDataParameter(pkceKey, codeVerifier, userId);
      }

      if (authRequestConfig['client_secret']) {
        authRequestConfig['client_secret'] = configData.clientSecret ?? '';
      }

      const authorizeRequestParams: Map<string, string> = getAuthorizeRequestUrlParams(
        Object.fromEntries(
          Object.entries({
            clientId: configData.clientId ?? '',
            codeChallenge,
            codeChallengeMethod: PKCEConstants.DEFAULT_CODE_CHALLENGE_METHOD,
            instanceId: this.getInstanceId().toString(),
            prompt: configData.prompt,
            redirectUri: configData.afterSignInUrl ?? '',
            responseMode: configData.responseMode,
            scopes: processOpenIDScopes(configData.scopes),
          }).filter(([, v]) => v !== undefined),
        ) as Parameters<typeof getAuthorizeRequestUrlParams>[0],
        {key: pkceKey},
        authRequestConfig,
      );

      Array.from(authorizeRequestParams.entries()).forEach(([paramKey, paramValue]: [string, string]) => {
        authorizeRequest.searchParams.append(paramKey, paramValue);
      });

      return authorizeRequest.toString();
    };

    if (
      await this.storageManager.getTemporaryDataParameter(
        OIDCDiscoveryConstants.Storage.StorageKeys.OPENID_PROVIDER_CONFIG_INITIATED,
      )
    ) {
      return buildSignInUrl();
    }

    return this.loadOpenIDProviderConfiguration(requestConfig?.forceInit).then(() => buildSignInUrl());
  }

  protected async requestAccessToken(
    authorizationCode: string,
    sessionState: string,
    state: string,
    userId?: string,
    tokenRequestConfig?: {params: Record<string, unknown>},
  ): Promise<TokenResponse> {
    if (
      !(await this.storageManager.getTemporaryDataParameter(
        OIDCDiscoveryConstants.Storage.StorageKeys.OPENID_PROVIDER_CONFIG_INITIATED,
      ))
    ) {
      await this.loadOpenIDProviderConfiguration(false);
    }

    const tokenEndpoint: string | undefined = (await this.oidcProviderMetaDataProvider()).token_endpoint;
    const configData = await this.configProvider();

    if (!tokenEndpoint || tokenEndpoint.trim().length === 0) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT1-NF01',
        'Token endpoint not found.',
        'No token endpoint was found in the OIDC provider meta data returned by the well-known endpoint ' +
          'or the token endpoint passed to the SDK is empty.',
      );
    }

    if (sessionState) {
      await this.storageManager.setSessionDataParameter(
        OIDCRequestConstants.Params.SESSION_STATE as keyof SessionData,
        sessionState,
        userId,
      );
    }

    const body: URLSearchParams = new URLSearchParams();

    body.set('client_id', configData.clientId ?? '');

    const hasSecret = Boolean(configData.clientSecret && configData.clientSecret.trim().length > 0);
    const tokenEndpointAuthMethod = configData.tokenRequest?.authMethod ?? 'client_secret_basic';

    if (hasSecret && tokenEndpointAuthMethod === 'client_secret_post') {
      body.set('client_secret', configData.clientSecret!);
    }

    body.set('code', authorizationCode);
    body.set('grant_type', 'authorization_code');
    body.set('redirect_uri', configData.afterSignInUrl ?? '');

    if (tokenRequestConfig?.params) {
      Object.entries(tokenRequestConfig.params).forEach(([key, value]: [string, unknown]) => {
        body.append(key, value as string);
      });
    }

    if (configData.enablePKCE) {
      body.set(
        'code_verifier',
        `${await this.storageManager.getHybridDataParameter(extractPkceStorageKeyFromState(state), userId)}`,
      );
      await this.storageManager.removeHybridDataParameter(extractPkceStorageKeyFromState(state), userId);
    }

    const tokenRequestHeaders: Record<string, string> = {
      Accept: 'application/json',
      'Content-Type': 'application/x-www-form-urlencoded',
    };

    if (hasSecret && tokenEndpointAuthMethod === 'client_secret_basic') {
      const credential = `${encodeURIComponent(configData.clientId!)}:${encodeURIComponent(configData.clientSecret!)}`;
      tokenRequestHeaders['Authorization'] = `Basic ${base64Encode(credential)}`;
    }

    let tokenResponse: Response;

    try {
      tokenResponse = await fetch(tokenEndpoint, {
        body,
        credentials: configData.sendCookiesInRequests ? 'include' : 'same-origin',
        headers: tokenRequestHeaders,
        method: 'POST',
      });
    } catch (error: any) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT1-NE02',
        'Requesting access token failed',
        error ?? 'The request to get the access token from the server failed.',
      );
    }

    if (!tokenResponse.ok) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT1-HE03',
        `Requesting access token failed with ${tokenResponse.statusText}`,
        (await tokenResponse.json()) as string,
      );
    }

    return this.authHelper.handleTokenResponse(tokenResponse, userId);
  }

  protected async getSignOutUrl(userId?: string): Promise<string> {
    const logoutEndpoint: string | undefined = (await this.oidcProviderMetaDataProvider())?.end_session_endpoint;
    const configData = await this.configProvider();

    if (!logoutEndpoint || logoutEndpoint.trim().length === 0) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-GSOU-NF01',
        'Sign-out endpoint not found.',
        'No sign-out endpoint was found in the OIDC provider meta data returned by the well-known endpoint ' +
          'or the sign-out endpoint passed to the SDK is empty.',
      );
    }

    const callbackURL: string | undefined = configData?.afterSignOutUrl ?? configData?.afterSignInUrl;

    if (!callbackURL || callbackURL.trim().length === 0) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-GSOU-NF03',
        'No sign-out redirect URL found.',
        'The sign-out redirect URL cannot be found or the URL passed to the SDK is empty.',
      );
    }

    const queryParams: URLSearchParams = new URLSearchParams();

    queryParams.set('post_logout_redirect_uri', callbackURL);

    if (configData.sendIdTokenInLogoutRequest) {
      const idToken: string = (await this.storageManager.getSessionData(userId))?.id_token;

      if (!idToken || idToken.trim().length === 0) {
        throw new ThunderIDAuthException(
          'JS-AUTH_CORE-GSOU-NF02',
          'ID token not found.',
          'No ID token could be found. Either the session information is lost or you have not signed in.',
        );
      }
      queryParams.set('id_token_hint', idToken);
    } else {
      queryParams.set('client_id', configData.clientId ?? '');
    }

    queryParams.set('state', OIDCRequestConstants.Params.SIGN_OUT_SUCCESS);

    return `${logoutEndpoint}?${queryParams.toString()}`;
  }

  protected async getOpenIDProviderEndpoints(): Promise<Partial<OIDCEndpoints>> {
    const meta: OIDCDiscoveryApiResponse = await this.oidcProviderMetaDataProvider();

    return {
      authorizationEndpoint: meta.authorization_endpoint ?? '',
      checkSessionIframe: meta.check_session_iframe ?? '',
      endSessionEndpoint: meta.end_session_endpoint ?? '',
      introspectionEndpoint: meta.introspection_endpoint ?? '',
      issuer: meta.issuer ?? '',
      jwksUri: meta.jwks_uri ?? '',
      registrationEndpoint: meta.registration_endpoint ?? '',
      revocationEndpoint: meta.revocation_endpoint ?? '',
      tokenEndpoint: meta.token_endpoint ?? '',
      userinfoEndpoint: meta.userinfo_endpoint ?? '',
    };
  }

  public async getDiscoveryResponse(): Promise<OIDCDiscoveryApiResponse | null> {
    if (!this.storageManager) {
      return null;
    }

    return this.storageManager.loadOpenIDProviderConfiguration();
  }

  protected async getDecodedIdToken(userId?: string, idToken?: string): Promise<IdToken> {
    const storedIdToken: string = (await this.storageManager.getSessionData(userId)).id_token;

    return this.cryptoHelper.decodeJwtToken<IdToken>(storedIdToken ?? idToken);
  }

  protected async getIdToken(userId?: string): Promise<string> {
    return (await this.storageManager.getSessionData(userId)).id_token;
  }

  protected async getUserSession(userId?: string): Promise<UserSession> {
    const sessionData: SessionData = await this.storageManager.getSessionData(userId);

    return {
      scopes: sessionData?.scope?.split(' '),
      sessionState: sessionData?.session_state ?? '',
    };
  }

  protected async refreshAccessToken(userId?: string): Promise<TokenResponse | User> {
    const tokenEndpoint: string | undefined = (await this.oidcProviderMetaDataProvider()).token_endpoint;
    const configData = await this.configProvider();
    const sessionData: SessionData = await this.storageManager.getSessionData(userId);

    if (!sessionData.refresh_token) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT2-NF01',
        'No refresh token found.',
        "There was no refresh token found. The server doesn't return a refresh token if the refresh token grant is not enabled.",
      );
    }

    if (!tokenEndpoint || tokenEndpoint.trim().length === 0) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT2-NF02',
        'No refresh token endpoint found.',
        'No refresh token endpoint was in the OIDC provider meta data returned by the well-known endpoint.',
      );
    }

    const body: string[] = [
      `client_id=${configData.clientId}`,
      `refresh_token=${sessionData.refresh_token}`,
      'grant_type=refresh_token',
    ];

    if (configData.clientSecret && configData.clientSecret.trim().length > 0) {
      body.push(`client_secret=${configData.clientSecret}`);
    }

    let tokenResponse: Response;

    try {
      tokenResponse = await fetch(tokenEndpoint, {
        body: body.join('&'),
        credentials: configData.sendCookiesInRequests ? 'include' : 'same-origin',
        headers: {Accept: 'application/json', 'Content-Type': 'application/x-www-form-urlencoded'},
        method: 'POST',
      });
    } catch (error: any) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT2-NR03',
        'Refresh access token request failed.',
        error ?? 'The request to refresh the access token failed.',
      );
    }

    if (!tokenResponse.ok) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT2-HE04',
        `Refreshing access token failed with ${tokenResponse.statusText}`,
        (await tokenResponse.json()) as string,
      );
    }

    return this.authHelper.handleTokenResponse(tokenResponse, userId);
  }

  protected async revokeAccessToken(userId?: string): Promise<Response | boolean> {
    const revokeTokenEndpoint: string | undefined = (await this.oidcProviderMetaDataProvider()).revocation_endpoint;
    const configData = await this.configProvider();

    if (!revokeTokenEndpoint || revokeTokenEndpoint.trim().length === 0) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT3-NF01',
        'No revoke access token endpoint found.',
        'No revoke access token endpoint was found in the OIDC provider meta data.',
      );
    }

    const body: string[] = [
      `client_id=${configData.clientId}`,
      `token=${(await this.storageManager.getSessionData(userId)).access_token}`,
      'token_type_hint=access_token',
    ];

    if (configData.clientSecret && configData.clientSecret.trim().length > 0) {
      body.push(`client_secret=${configData.clientSecret}`);
    }

    let response: Response;

    try {
      response = await fetch(revokeTokenEndpoint, {
        body: body.join('&'),
        credentials: configData.sendCookiesInRequests ? 'include' : 'same-origin',
        headers: {Accept: 'application/json', 'Content-Type': 'application/x-www-form-urlencoded'},
        method: 'POST',
      });
    } catch (error: any) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT3-NE02',
        'The request to revoke access token failed.',
        error ?? 'The request sent to revoke the access token failed.',
      );
    }

    if (response.status !== 200 || !response.ok) {
      throw new ThunderIDAuthException(
        'JS-AUTH_CORE-RAT3-HE03',
        `Invalid response status received for revoke access token request (${response.statusText}).`,
        (await response.json()) as string,
      );
    }

    this.authHelper.clearSession(userId);

    return response;
  }

  protected async getPKCECode(state: string, userId?: string): Promise<string> {
    return (await this.storageManager.getHybridDataParameter(extractPkceStorageKeyFromState(state), userId)) as string;
  }

  protected async setPKCECode(pkce: string, state: string, userId?: string): Promise<void> {
    return this.storageManager.setHybridDataParameter(extractPkceStorageKeyFromState(state), pkce, userId);
  }

  public getInstanceId(): number {
    return this.instanceIdValue;
  }

  protected getStorageManager(): StorageManager<T> {
    return this.storageManager;
  }

  protected getCryptoHelper(): IsomorphicCrypto {
    return this.cryptoHelper;
  }

  // ─── Static helpers ──────────────────────────────────────────────────────────

  public static isSignOutSuccessful(afterSignOutUrl: string): boolean {
    const url: URL = new URL(afterSignOutUrl);
    const stateParam: string | null = url.searchParams.get(OIDCRequestConstants.Params.STATE);
    const error = Boolean(url.searchParams.get('error'));

    return stateParam ? stateParam === OIDCRequestConstants.Params.SIGN_OUT_SUCCESS && !error : false;
  }

  public static didSignOutFail(afterSignOutUrl: string): boolean {
    const url: URL = new URL(afterSignOutUrl);
    const stateParam: string | null = url.searchParams.get(OIDCRequestConstants.Params.STATE);
    const error = Boolean(url.searchParams.get('error'));

    return stateParam ? stateParam === OIDCRequestConstants.Params.SIGN_OUT_SUCCESS && error : false;
  }

  // ─── Agent / OBO helpers ─────────────────────────────────────────────────────

  public async getAgentToken(agentConfig: AgentConfig): Promise<TokenResponse> {
    const customParam: Record<string, string> = {response_mode: 'direct'};
    const authorizeURL: URL = new URL(await this.getSignInUrl(customParam));

    const authorizeResponse: EmbeddedSignInFlowInitiateResponse = await initializeEmbeddedSignInFlow({
      payload: Object.fromEntries(authorizeURL.searchParams.entries()),
      url: `${authorizeURL.origin}${authorizeURL.pathname}`,
    });

    const authenticatorName: string = agentConfig.authenticatorName ?? AgentConfig.DEFAULT_AUTHENTICATOR_NAME;
    const targetAuthenticator: EmbeddedSignInFlowAuthenticator | undefined =
      authorizeResponse.nextStep.authenticators.find(
        (auth: EmbeddedSignInFlowAuthenticator) => auth.authenticator === authenticatorName,
      );

    if (!targetAuthenticator) {
      throw new Error(`Authenticator '${authenticatorName}' not found among authentication steps.`);
    }

    const authnRequest: EmbeddedFlowExecuteRequestConfig = {
      baseUrl: this.baseURL,
      payload: {
        flowId: authorizeResponse.flowId,
        selectedAuthenticator: {
          authenticatorId: targetAuthenticator.authenticatorId,
          params: {
            password: agentConfig.agentSecret,
            username: agentConfig.agentID,
          },
        },
      },
    };

    const authnResponse: EmbeddedSignInFlowHandleResponse = await executeEmbeddedSignInFlow(authnRequest);

    if (authnResponse.flowStatus !== EmbeddedSignInFlowStatus.SuccessCompleted) {
      throw new Error('Agent authentication failed.');
    }

    return this.requestAccessToken(
      authnResponse.authData['code'],
      authnResponse.authData['session_state'],
      authnResponse.authData['state'],
    );
  }

  public async getOBOSignInURL(agentConfig: AgentConfig): Promise<string> {
    const authURL: string = await this.getSignInUrl({requested_actor: agentConfig.agentID});

    if (authURL) {
      return authURL.toString();
    }

    throw new Error('Could not build Authorize URL');
  }

  public async getOBOToken(agentConfig: AgentConfig, authCodeResponse: AuthCodeResponse): Promise<TokenResponse> {
    const agentToken: TokenResponse = await this.getAgentToken(agentConfig);

    return this.requestAccessToken(
      authCodeResponse.code,
      authCodeResponse.session_state,
      authCodeResponse.state,
      undefined,
      {params: {actor_token: agentToken.accessToken}},
    );
  }
}

export default ThunderIDJavaScriptClient;
