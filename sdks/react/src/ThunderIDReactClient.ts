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

import {
  ThunderIDBrowserClient,
  UserProfile,
  SignInOptions,
  User,
  EmbeddedFlowExecuteResponse,
  SignUpOptions,
  EmbeddedFlowExecuteRequestPayload,
  ThunderIDRuntimeError,
  EmbeddedSignInFlowHandleRequestPayload,
  executeEmbeddedSignInFlowV2,
  Organization,
  IdToken,
  deriveOrganizationHandleFromBaseUrl,
  AllOrganizationsApiResponse,
  extractUserClaimsFromIdToken,
  TokenResponse,
  HttpRequestConfig,
  HttpResponse,
  TokenExchangeRequestConfig,
  isEmpty,
  EmbeddedSignInFlowResponseV2,
  executeEmbeddedSignUpFlowV2,
  executeEmbeddedRecoveryFlowV2,
  EmbeddedSignInFlowStatusV2,
  EmbeddedSignUpFlowStatusV2,
} from '@thunderid/browser';
import getAllOrganizations from './api/getAllOrganizations';
import getMeOrganizations from './api/getMeOrganizations';
import {ThunderIDReactConfig} from './models/config';

class ThunderIDReactClient<T extends ThunderIDReactConfig = ThunderIDReactConfig> extends ThunderIDBrowserClient<T> {
  private loadingState = false;

  private _initializeConfig: ThunderIDReactConfig | undefined;

  constructor(instanceId = 0) {
    super(instanceId);
  }

  private setLoading(loading: boolean): void {
    this.loadingState = loading;
  }

  private async withLoading<TResult>(operation: () => Promise<TResult>): Promise<TResult> {
    this.setLoading(true);
    try {
      const result: TResult = await operation();
      return result;
    } finally {
      this.setLoading(false);
    }
  }

  override initialize(config: ThunderIDReactConfig): Promise<boolean> {
    let resolvedOrganizationHandle: string | undefined = config?.organizationHandle;

    if (!resolvedOrganizationHandle) {
      resolvedOrganizationHandle = deriveOrganizationHandleFromBaseUrl(config?.baseUrl);
    }

    return this.withLoading(async () => {
      this._initializeConfig = {
        ...config,
        organizationHandle: resolvedOrganizationHandle,
        periodicTokenRefresh:
          config?.tokenLifecycle?.refreshToken?.autoRefresh ?? (config as any)?.periodicTokenRefresh,
      } as any;

      return super.initialize(this._initializeConfig as unknown as T);
    });
  }

  override reInitialize(config: Partial<ThunderIDReactConfig>): Promise<boolean> {
    return this.withLoading(async () => {
      let isInitialized: boolean;

      try {
        await super.reInitialize(config as Partial<T>);
        isInitialized = true;
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Failed to check if the client is initialized: ${error instanceof Error ? error.message : String(error)}`,
          'ThunderIDReactClient-reInitialize-RuntimeError-001',
          'react',
          'An error occurred while checking the initialization status of the client.',
        );
      }

      return isInitialized;
    });
  }

  override async updateUserProfile(): Promise<User> {
    throw new Error('Not implemented');
  }

  override async getUser(): Promise<User> {
    return extractUserClaimsFromIdToken(await this.getDecodedIdToken());
  }

  override async getDecodedIdToken(sessionId?: string): Promise<IdToken> {
    return super.getDecodedIdToken(sessionId);
  }

  override async getIdToken(): Promise<string> {
    return this.withLoading(async () => super.getIdToken());
  }

  override async getUserProfile(): Promise<UserProfile> {
    return this.withLoading(async () => {
      const claims: User = extractUserClaimsFromIdToken(await this.getDecodedIdToken());
      return {
        flattenedProfile: claims,
        profile: claims,
        schemas: [],
      };
    });
  }

  override async getMyOrganizations(options?: any): Promise<Organization[]> {
    try {
      let baseUrl: string = options?.baseUrl;

      if (!baseUrl) {
        const configData: any = await this.getStorageManager().getConfigData();
        baseUrl = configData?.baseUrl;
      }

      return await getMeOrganizations({baseUrl, instanceId: this.getInstanceId()});
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to fetch the user's associated organizations: ${
          error instanceof Error ? error.message : String(error)
        }`,
        'ThunderIDReactClient-getMyOrganizations-RuntimeError-001',
        'react',
        'An error occurred while fetching associated organizations of the signed-in user.',
      );
    }
  }

  override async getAllOrganizations(options?: any): Promise<AllOrganizationsApiResponse> {
    try {
      let baseUrl: string = options?.baseUrl;

      if (!baseUrl) {
        const configData: any = await this.getStorageManager().getConfigData();
        baseUrl = configData?.baseUrl;
      }

      return await getAllOrganizations({baseUrl, instanceId: this.getInstanceId()});
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to fetch all organizations: ${error instanceof Error ? error.message : String(error)}`,
        'ThunderIDReactClient-getAllOrganizations-RuntimeError-001',
        'react',
        'An error occurred while fetching all the organizations associated with the user.',
      );
    }
  }

  override async getCurrentOrganization(): Promise<Organization | null> {
    try {
      return await this.withLoading(async () => {
        const idToken: IdToken = await this.getDecodedIdToken();
        return {
          id: idToken?.org_id ?? '',
          name: idToken?.org_name ?? '',
          orgHandle: idToken?.org_handle ?? '',
        };
      });
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to fetch the current organization: ${error instanceof Error ? error.message : String(error)}`,
        'ThunderIDReactClient-getCurrentOrganization-RuntimeError-001',
        'react',
        'An error occurred while fetching the current organization of the signed-in user.',
      );
    }
  }

  override async switchOrganization(organization: Organization): Promise<TokenResponse | Response> {
    return this.withLoading(async () => {
      try {
        const configData: any = await this.getStorageManager().getConfigData();
        const sourceInstanceId: number | undefined = configData?.organizationChain?.sourceInstanceId;

        if (!organization.id) {
          throw new ThunderIDRuntimeError(
            'Organization ID is required for switching organizations',
            'react-ThunderIDReactClient-SwitchOrganizationError-001',
            'react',
            'The organization object must contain a valid ID to perform the organization switch.',
          );
        }

        const exchangeConfig: TokenExchangeRequestConfig = {
          attachToken: false,
          data: {
            client_id: '{{clientId}}',
            grant_type: 'organization_switch',
            scope: '{{scopes}}',
            switching_organization: organization.id,
            token: '{{accessToken}}',
          },
          id: 'organization-switch',
          returnsSession: true,
          signInRequired: sourceInstanceId === undefined,
        };

        return (await super.exchangeToken(exchangeConfig)) as TokenResponse | Response;
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Failed to switch organization: ${error.message || error}`,
          'react-ThunderIDReactClient-SwitchOrganizationError-003',
          'react',
          'An error occurred while switching to the specified organization. Please try again.',
        );
      }
    });
  }

  override isLoading(): boolean {
    return this.loadingState;
  }

  override async isSignedIn(): Promise<boolean> {
    return super.isSignedIn();
  }

  override async exchangeToken(config: TokenExchangeRequestConfig): Promise<TokenResponse | Response> {
    return this.withLoading(async () => super.exchangeToken(config) as unknown as TokenResponse | Response);
  }

  override async signIn(...args: any[]): Promise<User | EmbeddedSignInFlowResponseV2> {
    return this.withLoading(async () => {
      const arg1: any = args[0];
      const arg2: any = args[1];

      const config: ThunderIDReactConfig | undefined = (await this.getStorageManager().getConfigData()) as
        | ThunderIDReactConfig
        | undefined;

      // NOTE: With React 19 strict mode, the initialization logic runs twice, and there's an intermittent
      // issue where the config object is not getting stored in the storage layer with Vite scaffolding.
      // Hence, we need to check if the client is initialized but the config object is empty, and reinitialize.
      if (!config || Object.keys(config).length === 0) {
        await this.initialize(this._initializeConfig!);
      }

      if (typeof arg1 === 'object' && arg1 !== null && arg1.callOnlyOnRedirect === true) {
        return undefined as any;
      }

      if (
        typeof arg1 === 'object' &&
        arg1 !== null &&
        !isEmpty(arg1) &&
        ('executionId' in arg1 || 'applicationId' in arg1)
      ) {
        const authIdFromUrl: string = new URL(window.location.href).searchParams.get('authId') ?? '';
        const authIdFromStorage: string =
          ((await this.getStorageManager().getHybridDataParameter('authId')) as string) ?? '';
        const authId: string = authIdFromUrl || authIdFromStorage;
        const baseUrl: string = config?.baseUrl ?? '';

        const response: EmbeddedSignInFlowResponseV2 = await executeEmbeddedSignInFlowV2({
          authId,
          baseUrl,
          payload: arg1 as EmbeddedSignInFlowHandleRequestPayload,
          url: arg2?.url,
        });

        if (
          response &&
          typeof response === 'object' &&
          response.flowStatus === EmbeddedSignInFlowStatusV2.Complete &&
          response.assertion
        ) {
          const decodedAssertion: {
            [key: string]: unknown;
            exp?: number;
            iat?: number;
            scope?: string;
          } = await this.decodeJwtToken<{
            [key: string]: unknown;
            exp?: number;
            iat?: number;
            scope?: string;
          }>(response.assertion);

          const createdAt: number = decodedAssertion.iat ? decodedAssertion.iat * 1000 : Date.now();
          const expiresIn: number =
            decodedAssertion.exp && decodedAssertion.iat ? decodedAssertion.exp - decodedAssertion.iat : 3600;

          await this.setSession({
            access_token: response.assertion,
            created_at: createdAt,
            expires_in: expiresIn,
            id_token: response.assertion,
            scope: decodedAssertion.scope,
            token_type: 'Bearer',
          });

          this.notifySignIn(extractUserClaimsFromIdToken(decodedAssertion as IdToken) as User);
        }

        return response;
      }

      return (await super.signIn(...args)) as unknown as User;
    });
  }

  override async signInSilently(options?: SignInOptions): Promise<User | boolean> {
    return super.signInSilently(options as Record<string, string | boolean>) as Promise<User | boolean>;
  }

  override async signUp(options?: SignUpOptions): Promise<void>;
  override async signUp(payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse>;
  override async signUp(...args: any[]): Promise<void | EmbeddedFlowExecuteResponse> {
    const config: ThunderIDReactConfig = (await this.getStorageManager().getConfigData()) as ThunderIDReactConfig;
    const firstArg: any = args[0];
    const baseUrl: string = config?.baseUrl ?? '';

    const authIdFromUrl: string = new URL(window.location.href).searchParams.get('authId') ?? '';
    const authIdFromStorage: string =
      ((await this.getStorageManager().getHybridDataParameter('authId')) as string) ?? '';
    const authId: string = authIdFromUrl || authIdFromStorage;

    if (authIdFromUrl && !authIdFromStorage) {
      await this.getStorageManager().setHybridDataParameter('authId', authIdFromUrl);
    }

    const response: any = await executeEmbeddedSignUpFlowV2({
      authId,
      baseUrl,
      payload:
        typeof firstArg === 'object' && 'flowType' in firstArg
          ? {...(firstArg as EmbeddedFlowExecuteRequestPayload), verbose: true}
          : (firstArg as EmbeddedFlowExecuteRequestPayload),
    });

    if (
      response &&
      typeof response === 'object' &&
      response.flowStatus === EmbeddedSignUpFlowStatusV2.Complete &&
      response.assertion
    ) {
      const decodedAssertion: {
        [key: string]: unknown;
        exp?: number;
        iat?: number;
        scope?: string;
      } = await this.decodeJwtToken<{
        [key: string]: unknown;
        exp?: number;
        iat?: number;
        scope?: string;
      }>(response.assertion);

      const createdAt: number = decodedAssertion.iat ? decodedAssertion.iat * 1000 : Date.now();
      const expiresIn: number =
        decodedAssertion.exp && decodedAssertion.iat ? decodedAssertion.exp - decodedAssertion.iat : 3600;

      await this.setSession({
        access_token: response.assertion,
        created_at: createdAt,
        expires_in: expiresIn,
        id_token: response.assertion,
        scope: decodedAssertion.scope,
        token_type: 'Bearer',
      });

      this.notifySignIn(extractUserClaimsFromIdToken(decodedAssertion as IdToken) as User);
    }

    return response;
  }

  override async recover(payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> {
    const config: ThunderIDReactConfig = (await this.getStorageManager().getConfigData()) as ThunderIDReactConfig;

    return executeEmbeddedRecoveryFlowV2({
      baseUrl: config?.baseUrl,
      payload: {...payload, verbose: true},
    }) as any;
  }

  public override getStorageManager(): any {
    return super.getStorageManager();
  }

  async request(requestConfig?: HttpRequestConfig): Promise<HttpResponse<any>> {
    return super.httpRequest(requestConfig!) as Promise<HttpResponse<any>>;
  }

  async requestAll(requestConfigs?: HttpRequestConfig[]): Promise<HttpResponse<any>[]> {
    return super.httpRequestAll(requestConfigs!) as Promise<HttpResponse<any>[]>;
  }
}

export default ThunderIDReactClient;
