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
  AllOrganizationsApiResponse,
  ThunderIDNodeClient,
  ThunderIDRuntimeError,
  AuthClientConfig,
  CreateOrganizationPayload,
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedFlowExecuteResponse,
  ExtendedAuthorizeRequestUrlParams,
  FlattenedSchema,
  IdToken,
  Organization,
  OrganizationDetails,
  Schema,
  SignInOptions,
  SignUpOptions,
  Storage,
  TokenExchangeRequestConfig,
  TokenResponse,
  User,
  UserProfile,
  createOrganization,
  deriveOrganizationHandleFromBaseUrl,
  executeEmbeddedSignInFlow,
  executeEmbeddedSignUpFlow,
  extractUserClaimsFromIdToken,
  flattenUserSchema,
  generateFlattenedUserProfile,
  generateUserProfile,
  getAllOrganizations,
  getMeOrganizations,
  getOrganization,
  getScim2Me,
  getSchemas,
  initializeEmbeddedSignInFlow,
  updateMeProfile,
} from '@thunderid/node';
import {ThunderIDNextConfig} from './models/config';
import getClientOrigin from './server/actions/getClientOrigin';
import getSessionId from './server/actions/getSessionId';
import decorateConfigWithNextEnv from './utils/decorateConfigWithNextEnv';

class ThunderIDNextClient<T extends ThunderIDNextConfig = ThunderIDNextConfig> extends ThunderIDNodeClient<T> {
  public isInitialized = false;

  public constructor() {
    super();
  }

  private async ensureInitialized(): Promise<void> {
    if (!this.isInitialized) {
      throw new Error(
        '[ThunderIDNextClient] Client is not initialized. Make sure you have wrapped your app with ThunderIDProvider and provided the required configuration (baseUrl, clientId, etc.).',
      );
    }
  }

  override async initialize(config: T, storage?: Storage): Promise<boolean> {
    if (this.isInitialized) {
      return Promise.resolve(true);
    }

    const {
      baseUrl,
      organizationHandle,
      clientId,
      clientSecret,
      signInUrl,
      afterSignInUrl,
      afterSignOutUrl,
      signUpUrl,
      ...rest
    } = decorateConfigWithNextEnv(config);

    this.isInitialized = true;

    let resolvedOrganizationHandle: string | undefined = organizationHandle;

    if (!resolvedOrganizationHandle) {
      resolvedOrganizationHandle = deriveOrganizationHandleFromBaseUrl(baseUrl);
    }

    const origin: string = await getClientOrigin();

    return super.initialize(
      {
        afterSignInUrl: afterSignInUrl ?? origin,
        afterSignOutUrl: afterSignOutUrl ?? origin,
        baseUrl,
        clientId,
        clientSecret,
        enablePKCE: false,
        organizationHandle: resolvedOrganizationHandle,
        signInUrl,
        signUpUrl,
        ...rest,
      } as any,
      storage,
    );
  }

  override async reInitialize(config: Partial<T>): Promise<boolean> {
    try {
      await super.reInitialize(config);
      return true;
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to re-initialize the client: ${error instanceof Error ? error.message : String(error)}`,
        'ThunderIDNextClient-reInitialize-RuntimeError-001',
        'nextjs',
        'An error occurred while re-initializing the client. Please check your configuration and network connection.',
      );
    }
  }

  override async getUser(userId?: string): Promise<User> {
    await this.ensureInitialized();
    const resolvedSessionId: string = userId || (await getSessionId())!;

    try {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string | undefined = configData?.baseUrl;

      const profile: User = await getScim2Me({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
      });

      const schemas: Schema[] = await getSchemas({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
      });

      return generateUserProfile(profile, flattenUserSchema(schemas));
    } catch (error) {
      return (await super.getUser(resolvedSessionId)) as User;
    }
  }

  override async getUserProfile(userId?: string): Promise<UserProfile> {
    await this.ensureInitialized();

    try {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string | undefined = configData?.baseUrl;

      const profile: User = await getScim2Me({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
      });

      const schemas: Schema[] = await getSchemas({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
      });

      const processedSchemas: FlattenedSchema[] = flattenUserSchema(schemas);

      return {
        flattenedProfile: generateFlattenedUserProfile(profile, processedSchemas),
        profile,
        schemas: processedSchemas,
      };
    } catch (error) {
      return {
        flattenedProfile: extractUserClaimsFromIdToken((await super.getDecodedIdToken(userId))!),
        profile: extractUserClaimsFromIdToken((await super.getDecodedIdToken(userId))!),
        schemas: [],
      };
    }
  }

  override async updateUserProfile(payload: any, userId?: string): Promise<User> {
    await this.ensureInitialized();

    try {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string | undefined = configData?.baseUrl;

      return updateMeProfile({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
        payload,
      });
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to update user profile: ${error instanceof Error ? error.message : String(error)}`,
        'ThunderIDNextClient-UpdateProfileError-001',
        'react',
        'An error occurred while updating the user profile. Please check your configuration and network connection.',
      );
    }
  }

  async createOrganization(payload: CreateOrganizationPayload, userId?: string): Promise<Organization> {
    try {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string = configData?.baseUrl!;

      return createOrganization({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
        payload,
      });
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to create organization: ${error instanceof Error ? error.message : String(error)}`,
        'ThunderIDReactClient-createOrganization-RuntimeError-001',
        'nextjs',
        'An error occurred while creating the organization. Please check your configuration and network connection.',
      );
    }
  }

  async getOrganization(organizationId: string, userId?: string): Promise<OrganizationDetails> {
    try {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string = configData?.baseUrl!;

      return getOrganization({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
        organizationId,
      });
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to fetch the organization details of ${organizationId}: ${String(error)}`,
        'ThunderIDReactClient-getOrganization-RuntimeError-001',
        'nextjs',
        `An error occurred while fetching the organization with the id: ${organizationId}.`,
      );
    }
  }

  override async getMyOrganizations(options?: any, userId?: string): Promise<Organization[]> {
    try {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string = configData?.baseUrl!;

      return getMeOrganizations({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
      });
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to fetch the user's associated organizations: ${
          error instanceof Error ? error.message : String(error)
        }`,
        'ThunderIDNextClient-getMyOrganizations-RuntimeError-001',
        'nextjs',
        'An error occurred while fetching associated organizations of the signed-in user.',
      );
    }
  }

  override async getAllOrganizations(options?: any, userId?: string): Promise<AllOrganizationsApiResponse> {
    try {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string = configData?.baseUrl!;

      return getAllOrganizations({
        baseUrl,
        headers: {
          Authorization: `Bearer ${await this.getAccessToken(userId)}`,
        },
      });
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to fetch all organizations: ${error instanceof Error ? error.message : String(error)}`,
        'ThunderIDNextClient-getAllOrganizations-RuntimeError-001',
        'nextjs',
        'An error occurred while fetching all the organizations associated with the user.',
      );
    }
  }

  override async getCurrentOrganization(userId?: string): Promise<Organization | null> {
    const idToken: IdToken = (await super.getDecodedIdToken(userId))!;

    return {
      id: idToken?.org_id!,
      name: idToken?.org_name!,
      orgHandle: idToken?.org_handle!,
    };
  }

  override async switchOrganization(organization: Organization, userId?: string): Promise<TokenResponse | Response> {
    try {
      if (!organization.id) {
        throw new ThunderIDRuntimeError(
          'Organization ID is required for switching organizations',
          'ThunderIDNextClient-switchOrganization-ValidationError-001',
          'nextjs',
          'The organization object must contain a valid ID to perform the organization switch.',
        );
      }

      const exchangeConfig: TokenExchangeRequestConfig = {
        attachToken: false,
        data: {
          client_id: '{{clientId}}',
          client_secret: '{{clientSecret}}',
          grant_type: 'organization_switch',
          scope: '{{scopes}}',
          switching_organization: organization.id,
          token: '{{accessToken}}',
        },
        id: 'organization-switch',
        returnsSession: true,
        signInRequired: true,
      };

      return super.exchangeToken(exchangeConfig, userId) as unknown as Promise<TokenResponse | Response>;
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to switch organization: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
        'ThunderIDReactClient-RuntimeError-003',
        'nextjs',
        'An error occurred while switching to the specified organization. Please try again.',
      );
    }
  }

  override isLoading(): boolean {
    return false;
  }

  override isSignedIn(sessionId?: string): Promise<boolean> {
    return super.isSignedIn(sessionId!) as Promise<boolean>;
  }

  override exchangeToken(config: TokenExchangeRequestConfig, sessionId?: string): Promise<TokenResponse | Response> {
    return super.exchangeToken(config, sessionId) as unknown as Promise<TokenResponse | Response>;
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  override async getAccessToken(_sessionId?: string): Promise<string> {
    const {default: getAccessToken} = await import('./server/actions/getAccessToken');
    const token: string | undefined = await getAccessToken();

    if (typeof token !== 'string' || !token) {
      throw new ThunderIDRuntimeError(
        'Failed to get access token.',
        'ThunderIDNextClient-getAccessToken-RuntimeError-003',
        'nextjs',
        'An error occurred while obtaining the access token. Please check your configuration and network connection.',
      );
    }

    return token;
  }

  override async getDecodedIdToken(sessionId?: string, idToken?: string): Promise<IdToken> {
    await this.ensureInitialized();
    return (await super.getDecodedIdToken(sessionId, idToken)) as IdToken;
  }

  override async signIn(...args: any[]): Promise<any> {
    const arg1: any = args[0];
    const arg2: any = args[1];
    const arg3: any = args[2];
    const arg4: any = args[3];

    if (typeof arg1 === 'object' && 'flowId' in arg1) {
      if (arg1.flowId === '') {
        const defaultSignInUrl: URL = new URL(
          await this.getAuthorizeRequestUrl({
            client_secret: '{{clientSecret}}',
            response_mode: 'direct',
          }),
        );

        return initializeEmbeddedSignInFlow({
          payload: Object.fromEntries(defaultSignInUrl.searchParams.entries()),
          url: `${defaultSignInUrl.origin}${defaultSignInUrl.pathname}`,
        });
      }

      return executeEmbeddedSignInFlow({
        payload: arg1,
        url: arg2.url,
      });
    }

    return super.signIn(
      arg4,
      arg3,
      arg1?.code,
      arg1?.session_state,
      arg1?.state,
      arg1,
    ) as unknown as Promise<User>;
  }

  override async signOut(...args: any[]): Promise<string> {
    if (args[1] && typeof args[1] !== 'string') {
      throw new Error('The second argument must be a string.');
    }

    const resolvedSessionId: string = args[1] || (await getSessionId())!;

    return super.signOut(resolvedSessionId);
  }

  override async signUp(options?: SignUpOptions): Promise<void>;
  override async signUp(payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse>;
  override async signUp(firstArg?: any): Promise<void | EmbeddedFlowExecuteResponse> {
    if (firstArg === undefined || firstArg === null) {
      throw new ThunderIDRuntimeError(
        'No arguments provided for signUp method.',
        'ThunderIDNextClient-ValidationError-001',
        'nextjs',
        'The signUp method requires at least one argument, either a SignUpOptions object or an EmbeddedFlowExecuteRequestPayload.',
      );
    }

    if (typeof firstArg === 'object' && 'flowType' in firstArg) {
      const configData: AuthClientConfig<T> = await this.getStorageManager().getConfigData();
      const baseUrl: string | undefined = configData?.baseUrl;

      return executeEmbeddedSignUpFlow({
        baseUrl,
        payload: firstArg as EmbeddedFlowExecuteRequestPayload,
      });
    }
    throw new ThunderIDRuntimeError(
      'Not implemented',
      'ThunderIDNextClient-ValidationError-002',
      'nextjs',
      'The signUp method with SignUpOptions is not implemented in the Next.js client.',
    );
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  override signInSilently(_options?: SignInOptions): Promise<User | boolean> {
    throw new ThunderIDRuntimeError(
      'Not implemented',
      'ThunderIDNextClient-signInSilently-NotImplementedError-001',
      'nextjs',
      'The signInSilently method is not implemented in the Next.js client.',
    );
  }

  public async getAuthorizeRequestUrl(
    customParams: ExtendedAuthorizeRequestUrlParams,
    userId?: string,
  ): Promise<string> {
    await this.ensureInitialized();
    return this.getSignInUrl(customParams, userId);
  }

  public override getStorageManager(): any {
    return super.getStorageManager();
  }

  public override async clearSession(): Promise<void> {
    throw new ThunderIDRuntimeError(
      'Not implemented',
      'ThunderIDNextClient-clearSession-NotImplementedError-001',
      'nextjs',
      'The clearSession method is not implemented in the Next.js client.',
    );
  }

  override async setSession(sessionData: Record<string, unknown>, sessionId?: string): Promise<void> {
    return this.getStorageManager().setSessionData(sessionData, sessionId);
  }

  override decodeJwtToken<R = Record<string, unknown>>(token: string): Promise<R> {
    return super.decodeJwtToken<R>(token);
  }
}

export default ThunderIDNextClient;
