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
  ThunderIDRuntimeError,
  extractUserClaimsFromIdToken,
  generateFlattenedUserProfile,
  hasAuthParamsInUrl,
  hasCalledForThisInstanceInUrl,
  HttpResponse,
  IdToken,
  Organization,
  Platform,
  User,
  UserProfile,
  SignInOptions,
  TokenResponse,
  EmbeddedSignInFlowResponseV2,
} from '@thunderid/browser';
import {
  type Component,
  defineComponent,
  h,
  onMounted,
  onUnmounted,
  provide,
  type Ref,
  ref,
  type SetupContext,
  type ShallowRef,
  shallowRef,
  type PropType,
  type VNode,
} from 'vue';
import BrandingProvider from './BrandingProvider';
import FlowMetaProvider from './FlowMetaProvider';
import FlowProvider from './FlowProvider';
import I18nProvider from './I18nProvider';
import OrganizationProvider from './OrganizationProvider';
import ThemeProvider from './ThemeProvider';
import UserProvider from './UserProvider';
import {THUNDERID_KEY} from '../keys';
import type {ThunderIDVueConfig} from '../models/config';
import type {ThunderIDContext} from '../models/contexts';
import ThunderIDVueClient from '../ThunderIDVueClient';

interface ThunderIDProviderProps {
  afterSignInUrl: string | undefined;
  afterSignOutUrl: string | undefined;
  applicationId: string | undefined;
  baseUrl: string;
  clientId: string;
  instanceId: number;
  organizationChain: object | undefined;
  organizationHandle: string | undefined;
  platform: string | undefined;
  scopes: string | string[] | undefined;
  signInOptions: SignInOptions | undefined;
  signInUrl: string | undefined;
  signUpUrl: string | undefined;
  storage: string | undefined;
  syncSession: boolean | undefined;
}

/**
 * Checks if the current URL contains authentication parameters.
 */
function hasAuthParams(url: URL, afterSignInUrl: string | undefined): boolean {
  return (
    (hasAuthParamsInUrl() &&
      !!afterSignInUrl &&
      new URL(url.origin + url.pathname).toString() === new URL(afterSignInUrl).toString()) ||
    url.searchParams.get('error') !== null
  );
}

/**
 * Root provider component for the ThunderID Vue SDK.
 *
 * This component initializes the client, manages authentication state,
 * and provides the ThunderID context to child components via Vue's provide/inject.
 *
 * @example
 * ```vue
 * <template>
 *   <ThunderIDProvider v-bind="config">
 *     <router-view />
 *   </ThunderIDProvider>
 * </template>
 * ```
 */
const ThunderIDProvider: Component = defineComponent({
  name: 'ThunderIDProvider',
  props: {
    /** The URL to redirect to after sign in. */
    afterSignInUrl: {
      default: undefined,
      type: String,
    },
    /** The URL to redirect to after sign out. */
    afterSignOutUrl: {
      default: undefined,
      type: String,
    },
    /** The ThunderID application ID. */
    applicationId: {
      default: undefined,
      type: String,
    },
    /** The base URL of the ThunderID tenant. */
    baseUrl: {
      required: true,
      type: String,
    },
    /** The OAuth2 client ID. */
    clientId: {
      required: true,
      type: String,
    },
    /** Instance ID for multi-instance support. */
    instanceId: {
      default: 0,
      type: Number,
    },
    /** Organization chain config. */
    organizationChain: {
      default: undefined,
      type: Object,
    },
    /** The organization handle. */
    organizationHandle: {
      default: undefined,
      type: String,
    },
    /** Platform type. */
    platform: {
      default: undefined,
      type: String,
    },
    /** The scopes to request. */
    scopes: {
      default: undefined,
      type: [Array, String] as PropType<string | string[]>,
    },
    /** Additional sign-in options. */
    signInOptions: {
      default: undefined,
      type: Object as PropType<SignInOptions>,
    },
    /** The sign-in URL. */
    signInUrl: {
      default: undefined,
      type: String,
    },
    /** The sign-up URL. */
    signUpUrl: {
      default: undefined,
      type: String,
    },
    /** Storage type. */
    storage: {
      default: undefined,
      type: String,
    },
    /** Whether to sync sessions across tabs. */
    syncSession: {
      default: undefined,
      type: Boolean,
    },
  },
  setup(props: ThunderIDProviderProps, {slots}: SetupContext): () => VNode {
    // ── Client ──
    const client: ThunderIDVueClient = new ThunderIDVueClient(props.instanceId);

    // ── Reactive State ──
    const isSignedIn: Ref<boolean> = ref<boolean>(false);
    const isInitialized: Ref<boolean> = ref<boolean>(false);
    const isLoading: Ref<boolean> = ref<boolean>(true);
    const user: ShallowRef<any | null> = shallowRef<any | null>(null);
    const currentOrganization: ShallowRef<Organization | null> = shallowRef<Organization | null>(null);
    const myOrganizations: ShallowRef<Organization[]> = shallowRef<Organization[]>([]);
    const userProfile: ShallowRef<UserProfile | null> = shallowRef<UserProfile | null>(null);
    const resolvedBaseUrl: Ref<string> = ref<string>(props.baseUrl);

    let isUpdatingSession = false;
    let signInCheckInterval: ReturnType<typeof setInterval> | undefined;
    let loadingCheckInterval: ReturnType<typeof setInterval> | undefined;

    // ── Build config from props ──
    function buildConfig(): ThunderIDVueConfig {
      return {
        afterSignInUrl: props.afterSignInUrl ?? window.location.origin,
        afterSignOutUrl: props.afterSignOutUrl ?? window.location.origin,
        applicationId: props.applicationId,
        baseUrl: props.baseUrl,
        clientId: props.clientId,
        organizationChain: props.organizationChain,
        organizationHandle: props.organizationHandle,
        scopes: props.scopes,
        signInOptions: props.signInOptions,
        signInUrl: props.signInUrl,
        signUpUrl: props.signUpUrl,
        storage: props.storage,
        syncSession: props.syncSession,
      } as ThunderIDVueConfig;
    }

    // ── Session Update ──
    async function updateSession(): Promise<void> {
      try {
        isUpdatingSession = true;
        isLoading.value = true;
        let baseUrl: string = resolvedBaseUrl.value;

        const decodedToken: IdToken = await client.getDecodedIdToken();

        if (decodedToken?.['user_org']) {
          baseUrl = `${(await client.getConfiguration()).baseUrl}/o`;
          resolvedBaseUrl.value = baseUrl;
        }

        const claims: User = extractUserClaimsFromIdToken(decodedToken);
        user.value = claims;
        const profileData: UserProfile = {
          flattenedProfile: claims,
          profile: claims,
          schemas: [],
        };
        userProfile.value = profileData;

        const currentSignInStatus: boolean = await client.isSignedIn();
        isSignedIn.value = currentSignInStatus;
      } catch {
        // silent
      } finally {
        isUpdatingSession = false;
        isLoading.value = client.isLoading();
      }
    }

    // ── Sign In (wrapper) ──
    async function signIn(...args: any[]): Promise<User | EmbeddedSignInFlowResponseV2> {
      const arg1: any = args[0];
      const isV2FlowRequest: boolean =
        typeof arg1 === 'object' && arg1 !== null && ('executionId' in arg1 || 'applicationId' in arg1);

      try {
        if (!isV2FlowRequest) {
          isUpdatingSession = true;
          isLoading.value = true;
        }

        return await client.signIn(...args);
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Sign in failed: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
          'thunderid-signIn-Error',
          'vue',
          'An error occurred while trying to sign in.',
        );
      } finally {
        if (!isV2FlowRequest) {
          isUpdatingSession = false;
          isLoading.value = client.isLoading();
        }
      }
    }

    // ── Sign Out ──
    async function signOut(...args: any[]): Promise<any> {
      return client.signOut(...args);
    }

    // ── Sign Up ──
    async function signUp(...args: any[]): Promise<any> {
      return client.signUp(...args);
    }

    // ── Sign In Silently ──
    async function signInSilently(options?: SignInOptions): Promise<User | boolean> {
      try {
        isUpdatingSession = true;
        isLoading.value = true;
        return await client.signInSilently(options);
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Error while signing in silently: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
          'thunderid-signInSilently-Error',
          'vue',
          'An error occurred while trying to sign in silently.',
        );
      } finally {
        isUpdatingSession = false;
        isLoading.value = client.isLoading();
      }
    }

    // ── Switch Organization ──
    async function switchOrganization(organization: Organization): Promise<TokenResponse | Response> {
      try {
        isUpdatingSession = true;
        isLoading.value = true;
        const response: TokenResponse | Response = await client.switchOrganization(organization);

        if (await client.isSignedIn()) {
          await updateSession();
        }

        return response;
      } catch (error) {
        throw new ThunderIDRuntimeError(
          `Failed to switch organization: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
          'thunderid-switchOrganization-Error',
          'vue',
          'An error occurred while switching to the specified organization.',
        );
      } finally {
        isUpdatingSession = false;
        isLoading.value = client.isLoading();
      }
    }

    // ── Provide Context ──
    const context: ThunderIDContext = {
      afterSignInUrl: props.afterSignInUrl,
      applicationId: props.applicationId,
      baseUrl: props.baseUrl,
      clearSession: async (...args: any[]): Promise<void> => {
        await client.clearSession(...args);
      },
      clientId: props.clientId,
      scopes: props.scopes,
      exchangeToken: (config: any): Promise<TokenResponse | Response> => client.exchangeToken(config),
      getAccessToken: (): Promise<string> => client.getAccessToken(),
      getDecodedIdToken: (): Promise<IdToken> => client.getDecodedIdToken(),
      getIdToken: (): Promise<string> => client.getIdToken(),
      getStorageManager: () => client.getStorageManager(),
      http: {
        request: (requestConfig?: any): Promise<HttpResponse<any>> => client.request(requestConfig),
        requestAll: (requestConfigs?: any[]): Promise<HttpResponse<any>[]> => client.requestAll(requestConfigs),
      },
      instanceId: props.instanceId,
      isInitialized,
      isLoading,
      isSignedIn,
      organization: currentOrganization,
      organizationHandle: props.organizationHandle,
      platform: Platform.ThunderID,
      reInitialize: async (config: any): Promise<boolean> => {
        const result: boolean = await client.reInitialize(config);
        return typeof result === 'boolean' ? result : true;
      },
      signIn,
      signInOptions: props.signInOptions,
      signInSilently,
      signInUrl: props.signInUrl,
      signOut,
      signUp,
      signUpUrl: props.signUpUrl,
      storage: props.storage as ThunderIDVueConfig['storage'],
      switchOrganization,
      user,
    };

    provide(THUNDERID_KEY, context);

    // ── Lifecycle ──
    onMounted(async (): Promise<void> => {
      // 1. Initialize the client
      const config: ThunderIDVueConfig = buildConfig();
      await client.initialize(config);

      // 2. Load the OpenID provider configuration
      // We manually initialize this here because `initialize` doesn't load it for us.
      // This is needed for endpoints like /.well-known/openid-configuration.
      await client.getDiscoveryResponse();

      const initializedConfig: any = await (client.getConfiguration() as any);

      if (initializedConfig?.baseUrl) {
        sessionStorage.setItem('thunderid_base_url', initializedConfig.baseUrl);
      }

      // 2. Check initialization status
      try {
        const status: boolean = await client.isInitialized();
        isInitialized.value = status;
      } catch {
        isInitialized.value = false;
      }

      // Sync session state whenever sign-in completes (both redirect and embedded V2 flows).
      await client.on('sign-in', async () => {
        await updateSession();
      });

      // 3. Try to sign in if already authenticated or if URL has auth params
      const alreadySignedIn: boolean = await client.isSignedIn();

      if (alreadySignedIn) {
        await updateSession();
      } else {
        const currentUrl: URL = new URL(window.location.href);
        const hasParams: boolean =
          hasAuthParams(currentUrl, initializedConfig?.afterSignInUrl) &&
          hasCalledForThisInstanceInUrl(props.instanceId ?? 0, currentUrl.search);

        if (hasParams) {
          try {
            const urlParams: URLSearchParams = currentUrl.searchParams;
            const code: string | null = urlParams.get('code');
            const executionIdFromUrl: string | null = urlParams.get('executionId');
            const storedExecutionId: string | null = sessionStorage.getItem('thunderid_execution_id');

            if (code && !executionIdFromUrl && !storedExecutionId) {
              await signIn();
            }
          } catch (error) {
            throw new ThunderIDRuntimeError(
              `Sign in failed: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
              'thunderid-signIn-Error',
              'vue',
              'An error occurred while trying to sign in.',
            );
          }
        }
      }

      // 4. Set up polling for sign-in status
      try {
        const status: boolean = await client.isSignedIn();
        isSignedIn.value = status;

        if (!status) {
          signInCheckInterval = setInterval(async (): Promise<void> => {
            const newStatus: boolean = await client.isSignedIn();
            if (newStatus) {
              isSignedIn.value = true;
              if (signInCheckInterval) {
                clearInterval(signInCheckInterval);
                signInCheckInterval = undefined;
              }
            }
          }, 1000);
        }
      } catch {
        isSignedIn.value = false;
      }

      // 5. Set up polling for loading state
      loadingCheckInterval = setInterval((): void => {
        if (isUpdatingSession) return;

        const currentUrl: URL = new URL(window.location.href);
        if (!isSignedIn.value && hasAuthParams(currentUrl, initializedConfig?.afterSignInUrl)) return;

        isLoading.value = client.isLoading();
      }, 100);
    });

    onUnmounted((): void => {
      if (signInCheckInterval) {
        clearInterval(signInCheckInterval);
      }
      if (loadingCheckInterval) {
        clearInterval(loadingCheckInterval);
      }
    });

    // ── Render ──
    return (): any =>
      h(I18nProvider, null, {
        default: (): any =>
          h(
            FlowMetaProvider,
            {enabled: true},
            {
              default: (): any =>
                h(BrandingProvider, null, {
                  default: (): any =>
                    h(ThemeProvider, null, {
                      default: (): any =>
                        h(FlowProvider, null, {
                          default: (): any =>
                            h(
                              UserProvider,
                              {
                                onUpdateProfile: (updatedUser: User): void => {
                                  user.value = updatedUser;
                                  userProfile.value = {
                                    flattenedProfile: generateFlattenedUserProfile(
                                      updatedUser,
                                      userProfile.value?.schemas ?? [],
                                    ),
                                    profile: updatedUser,
                                    schemas: userProfile.value?.schemas ?? [],
                                  };
                                },
                                profile: userProfile.value,
                                revalidateProfile: async (): Promise<void> => {
                                  try {
                                    const decodedToken: IdToken = await client.getDecodedIdToken();
                                    const claims: User = extractUserClaimsFromIdToken(decodedToken);
                                    user.value = claims;
                                    userProfile.value = {
                                      flattenedProfile: claims,
                                      profile: claims,
                                      schemas: [],
                                    };
                                  } catch {
                                    // silent
                                  }
                                },
                              },
                              {
                                default: (): any =>
                                  h(
                                    OrganizationProvider,
                                    {
                                      currentOrganization: currentOrganization.value,
                                      getAllOrganizations: async (): Promise<AllOrganizationsApiResponse> =>
                                        client.getAllOrganizations({baseUrl: resolvedBaseUrl.value}),
                                      myOrganizations: myOrganizations.value,
                                      onOrganizationSwitch: switchOrganization,
                                      revalidateMyOrganizations: async (): Promise<Organization[]> => {
                                        const baseUrl: string = resolvedBaseUrl.value;
                                        try {
                                          const orgs: Organization[] = await client.getMyOrganizations({baseUrl});
                                          myOrganizations.value = orgs || [];
                                          return orgs || [];
                                        } catch {
                                          return [];
                                        }
                                      },
                                    },
                                    {
                                      default: (): any => slots['default']?.(),
                                    },
                                  ),
                              },
                            ),
                        }),
                    }),
                }),
            },
          ),
      });
  },
});

export default ThunderIDProvider;
