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
  ThunderIDBrowserClient,
  flattenUserSchema,
  generateFlattenedUserProfile,
  UserProfile,
  User,
  generateUserProfile,
  EmbeddedFlowExecuteResponse,
  SignUpOptions,
  EmbeddedFlowExecuteRequestPayload,
  ThunderIDRuntimeError,
  executeEmbeddedSignUpFlowV2,
  executeEmbeddedSignInFlowV2,
  Organization,
  IdToken,
  AllOrganizationsApiResponse,
  extractUserClaimsFromIdToken,
  TokenResponse,
  HttpRequestConfig,
  HttpResponse,
  TokenExchangeRequestConfig,
  isEmpty,
  EmbeddedSignInFlowResponseV2,
  EmbeddedSignInFlowStatusV2,
  EmbeddedSignUpFlowStatusV2,
  deriveOrganizationHandleFromBaseUrl,
  StorageManager,
} from '@thunderid/browser';
import getAllOrganizations from './api/getAllOrganizations';
import getMeOrganizations from './api/getMeOrganizations';
import getSchemas from './api/getSchemas';
import getScim2Me from './api/getScim2Me';
import {ThunderIDVueConfig} from './models/config';

/**
 * Client for implementing ThunderID in Vue applications.
 * This class provides the core functionality for managing user authentication and sessions.
 *
 * @typeParam T - Configuration type that extends ThunderIDVueConfig.
 */
class ThunderIDVueClient<T extends ThunderIDVueConfig = ThunderIDVueConfig> extends ThunderIDBrowserClient<T> {
  private loadingState = false;

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

  override initialize(config: ThunderIDVueConfig): Promise<boolean> {
    let resolvedOrganizationHandle: string | undefined = config?.organizationHandle;

    if (!resolvedOrganizationHandle) {
      resolvedOrganizationHandle = deriveOrganizationHandleFromBaseUrl(config?.baseUrl);
    }

    return this.withLoading(async () =>
      super.initialize({...config, organizationHandle: resolvedOrganizationHandle} as unknown as T),
    );
  }

  override reInitialize(config: Partial<ThunderIDVueConfig>): Promise<boolean> {
    return this.withLoading(async () => {
      let isInitialized: boolean;

      try {
        await super.reInitialize(config as Partial<T>);
        isInitialized = true;
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Failed to check if the client is initialized: ${error instanceof Error ? error.message : String(error)}`,
          'ThunderIDVueClient-reInitialize-RuntimeError-001',
          'vue',
          'An error occurred while checking the initialization status of the client.',
        );
      }

      return isInitialized;
    });
  }

  override async updateUserProfile(): Promise<User> {
    throw new Error('Not implemented');
  }

  override async getUser(options?: any): Promise<User> {
    try {
      let baseUrl: string = options?.baseUrl;

      if (!baseUrl) {
        const configData: any = await this.getStorageManager().getConfigData();
        baseUrl = configData?.baseUrl;
      }

      const profile: User = await getScim2Me({baseUrl});
      const schemas: any = await getSchemas({baseUrl});

      return generateUserProfile(profile, flattenUserSchema(schemas));
    } catch (error) {
      return extractUserClaimsFromIdToken(await this.getDecodedIdToken());
    }
  }

  override async getDecodedIdToken(sessionId?: string): Promise<IdToken> {
    return await super.getDecodedIdToken(sessionId);
  }

  override async getIdToken(): Promise<string> {
    return this.withLoading(async () => super.getIdToken());
  }

  override async getUserProfile(options?: any): Promise<UserProfile> {
    return this.withLoading(async () => {
      try {
        let baseUrl: string = options?.baseUrl;

        if (!baseUrl) {
          const configData: any = await this.getStorageManager().getConfigData();
          baseUrl = configData?.baseUrl;
        }

        const profile: User = await getScim2Me({baseUrl, instanceId: this.getInstanceId()});
        const schemas: any = await getSchemas({baseUrl, instanceId: this.getInstanceId()});

        const processedSchemas: any = flattenUserSchema(schemas);

        const output: UserProfile = {
          flattenedProfile: generateFlattenedUserProfile(profile, processedSchemas),
          profile,
          schemas: processedSchemas,
        };

        return output;
      } catch (error) {
        return {
          flattenedProfile: extractUserClaimsFromIdToken(await this.getDecodedIdToken()),
          profile: extractUserClaimsFromIdToken(await this.getDecodedIdToken()),
          schemas: [],
        };
      }
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
        'ThunderIDVueClient-getMyOrganizations-RuntimeError-001',
        'vue',
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
        'ThunderIDVueClient-getAllOrganizations-RuntimeError-001',
        'vue',
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
        'ThunderIDVueClient-getCurrentOrganization-RuntimeError-001',
        'vue',
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
            'vue-ThunderIDVueClient-SwitchOrganizationError-001',
            'vue',
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

        return (await super.exchangeToken(exchangeConfig as any)) as unknown as TokenResponse | Response;
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Failed to switch organization: ${error.message || error}`,
          'vue-ThunderIDVueClient-SwitchOrganizationError-003',
          'vue',
          'An error occurred while switching to the specified organization. Please try again.',
        );
      }
    });
  }

  override isLoading(): boolean {
    return this.loadingState || super.isLoading();
  }

  override async isInitialized(): Promise<boolean> {
    return super.isInitialized();
  }

  override async isSignedIn(): Promise<boolean> {
    return await super.isSignedIn();
  }

  override async exchangeToken(config: TokenExchangeRequestConfig): Promise<TokenResponse | Response> {
    return this.withLoading(
      async () => (await super.exchangeToken(config as any)) as unknown as TokenResponse | Response,
    );
  }

  override async signIn(...args: any[]): Promise<any> {
    return this.withLoading(async () => {
      const arg1: any = args[0];
      const arg2: any = args[1];

      if (typeof arg1 === 'object' && arg1 !== null && arg1.callOnlyOnRedirect === true) {
        return undefined;
      }

      if (
        typeof arg1 === 'object' &&
        arg1 !== null &&
        !isEmpty(arg1) &&
        ('executionId' in arg1 || 'applicationId' in arg1)
      ) {
        const configData: any = await this.getStorageManager().getConfigData();
        const authIdFromUrl: string | null = new URL(window.location.href).searchParams.get('authId');
        const authIdFromStorage: string | null = (await this.getStorageManager().getHybridDataParameter('authId')) as
          | string
          | null;
        const authId: string = authIdFromUrl || authIdFromStorage || '';
        const baseUrl: string = configData?.baseUrl || '';

        const response: EmbeddedSignInFlowResponseV2 = await executeEmbeddedSignInFlowV2({
          authId,
          baseUrl,
          payload: arg1,
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

      return (await super.signIn(arg1))!;
    });
  }

  override async signInSilently(options?: any): Promise<User | boolean> {
    return (await super.signInSilently(options as Record<string, string | boolean>))!;
  }

  override async signUp(options?: SignUpOptions): Promise<void>;
  override async signUp(payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse>;
  override async signUp(...args: any[]): Promise<void | EmbeddedFlowExecuteResponse> {
    const configData: any = await this.getStorageManager().getConfigData();
    const firstArg: any = args[0];
    const baseUrl: string = configData?.baseUrl || '';

    const authIdFromUrl: string | null = new URL(window.location.href).searchParams.get('authId');
    const authIdFromStorage: string | null = (await this.getStorageManager().getHybridDataParameter('authId')) as
      | string
      | null;
    const authId: string = authIdFromUrl || authIdFromStorage || '';

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

  async request(requestConfig?: HttpRequestConfig): Promise<HttpResponse<any>> {
    return (await this.httpRequest(requestConfig!))!;
  }

  async requestAll(requestConfigs?: HttpRequestConfig[]): Promise<HttpResponse<any>[]> {
    return (await this.httpRequestAll(requestConfigs!))!;
  }

  override async getAccessToken(sessionId?: string): Promise<string> {
    return super.getAccessToken(sessionId);
  }

  override clearSession(sessionId?: string): void {
    super.clearSession(sessionId);
  }

  override async setSession(sessionData: Record<string, unknown>, sessionId?: string): Promise<void> {
    return this.getStorageManager().setSessionData(sessionData, sessionId);
  }

  override decodeJwtToken<TResult = Record<string, unknown>>(token: string): Promise<TResult> {
    return super.decodeJwtToken<TResult>(token);
  }

  public override getStorageManager(): StorageManager<T> {
    return super.getStorageManager();
  }
}

export default ThunderIDVueClient;
