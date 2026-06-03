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
  AllOrganizationsApiResponse,
  ThunderIDRuntimeError,
  generateFlattenedUserProfile,
  OIDCDiscoveryApiResponse,
  Organization,
  SignInOptions,
  User,
  UserProfile,
  getBrandingPreference,
  GetBrandingPreferenceConfig,
  BrandingPreference,
  IdToken,
  getActiveTheme,
  Platform,
  extractUserClaimsFromIdToken,
  EmbeddedSignInFlowResponseV2,
  TokenResponse,
  createPackageComponentLogger,
} from '@thunderid/browser';
import {FC, RefObject, PropsWithChildren, ReactElement, useEffect, useMemo, useRef, useState, useCallback} from 'react';
import ThunderIDContext from './ThunderIDContext';
import useBrowserUrl from '../../hooks/useBrowserUrl';
import {ThunderIDReactConfig} from '../../models/config';
import ThunderIDReactClient from '../../ThunderIDReactClient';
import BrandingProvider from '../Branding/BrandingProvider';
import ComponentRendererProvider from '../ComponentRenderer/ComponentRendererProvider';
import FlowProvider from '../Flow/FlowProvider';
import FlowMetaProvider from '../FlowMeta/FlowMetaProvider';
import I18nProvider from '../I18n/I18nProvider';
import OrganizationProvider from '../Organization/OrganizationProvider';
import ThemeProvider from '../Theme/ThemeProvider';
import UserProvider from '../User/UserProvider';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/react',
  'ThunderIDProvider',
);

/**
 * Props interface of {@link ThunderIDProvider}
 */
export type ThunderIDProviderProps = ThunderIDReactConfig;

const ThunderIDProvider: FC<PropsWithChildren<ThunderIDProviderProps>> = ({
  afterSignInUrl,
  afterSignOutUrl,
  baseUrl: initialBaseUrl,
  clientId,
  children,
  extensions,
  scopes,
  preferences,
  signInUrl,
  signUpUrl,
  organizationHandle,
  applicationId,
  signInOptions,
  tokenRequest,
  syncSession,
  instanceId = 0,
  organizationChain,
  ...rest
}: PropsWithChildren<ThunderIDProviderProps>): ReactElement => {
  const reRenderCheckRef: RefObject<boolean> = useRef(false);
  const client: ThunderIDReactClient = useMemo(() => new ThunderIDReactClient(instanceId), [instanceId]);
  const storageManagerRef: any = useRef<any>(null);
  const {hasAuthParams, hasCalledForThisInstance} = useBrowserUrl();
  const [user, setUser] = useState<any | null>(null);
  const [currentOrganization, setCurrentOrganization] = useState<Organization | null>(null);

  const [isSignedInSync, setIsSignedInSync] = useState<boolean>(false);
  const [isInitializedSync, setIsInitializedSync] = useState<boolean>(false);
  const [isLoadingSync, setIsLoadingSync] = useState<boolean>(true);

  const [myOrganizations, setMyOrganizations] = useState<Organization[]>([]);
  const [userProfile, setUserProfile] = useState<UserProfile | null>(null);
  const [baseUrl, setBaseUrl] = useState<string>(initialBaseUrl ?? '');
  const [config, setConfig] = useState<ThunderIDReactConfig>({
    afterSignInUrl: afterSignInUrl ?? window.location.origin,
    afterSignOutUrl: afterSignOutUrl ?? window.location.origin,
    applicationId,
    baseUrl,
    clientId,
    organizationChain,
    organizationHandle,
    scopes,
    signInOptions,
    tokenRequest,
    signInUrl,
    signUpUrl,
    syncSession,
    ...rest,
  });

  const [isUpdatingSession, setIsUpdatingSession] = useState<boolean>(false);
  const [wellKnown, setWellKnown] = useState<OIDCDiscoveryApiResponse | null>(null);

  // Branding state
  const [brandingPreference, setBrandingPreference] = useState<BrandingPreference | null>(null);
  const [isBrandingLoading, setIsBrandingLoading] = useState<boolean>(false);
  const [brandingError, setBrandingError] = useState<Error | null>(null);
  const [hasFetchedBranding, setHasFetchedBranding] = useState<boolean>(false);

  useEffect(() => {
    setBaseUrl(initialBaseUrl ?? '');
    // Reset branding state when baseUrl changes
    if (initialBaseUrl !== baseUrl) {
      setHasFetchedBranding(false);
      setBrandingPreference(null);
      setBrandingError(null);
    }
  }, [initialBaseUrl, baseUrl]);

  useEffect(() => {
    (async (): Promise<void> => {
      await client.initialize(config);
      const initializedConfig: ThunderIDReactConfig = await client.getConfiguration();
      setConfig(initializedConfig);
      setWellKnown(await client.getDiscoveryResponse());
    })();
  }, []);

  async function updateSession(): Promise<void> {
    try {
      // Set flag to prevent loading state tracking from interfering
      setIsUpdatingSession(true);
      setIsLoadingSync(true);
      let resolvedBaseUrl: string = baseUrl;

      const decodedToken: IdToken = await client.getDecodedIdToken();

      // If there's a `user_org` claim in the ID token,
      // Treat this login as a organization login.
      if (decodedToken?.['user_org']) {
        resolvedBaseUrl = `${(await client.getConfiguration()).baseUrl}/o`;
        setBaseUrl(resolvedBaseUrl);
      }

      // TEMPORARY: SCIM2 and Organizations endpoints are not yet supported.
      const claims: User = extractUserClaimsFromIdToken(decodedToken) as User;
      setUser(claims);
      setUserProfile({
        flattenedProfile: claims,
        profile: claims,
        schemas: [],
      });

      // CRITICAL: Update sign-in status BEFORE setting loading to false
      // This prevents the race condition where ProtectedRoute sees isLoading=false but isSignedIn=false
      const currentSignInStatus: boolean = await client.isSignedIn();
      setIsSignedInSync(currentSignInStatus);
    } catch (error) {
      // TODO: Add an error log.
    } finally {
      // Clear the flag and set final loading state
      setIsUpdatingSession(false);
      setIsLoadingSync(client.isLoading());
    }
  }

  async function signIn(...args: any): Promise<User | EmbeddedSignInFlowResponseV2> {
    // Check if this is a V2 embedded flow request BEFORE calling signIn
    // This allows us to skip session checks entirely for V2 flows
    const arg1: any = args[0];
    const isV2FlowRequest: boolean =
      typeof arg1 === 'object' && arg1 !== null && ('executionId' in arg1 || 'applicationId' in arg1);

    try {
      if (!isV2FlowRequest) {
        setIsUpdatingSession(true);
        setIsLoadingSync(true);
      }

      return await client.signIn(...args);
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Sign in failed: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
        'thunderid-signIn-Error',
        'react',
        'An error occurred while trying to sign in.',
      );
    } finally {
      if (!isV2FlowRequest) {
        setIsUpdatingSession(false);
        setIsLoadingSync(client.isLoading());
      }
    }
  }

  /**
   * Try signing in when the component is mounted.
   */
  useEffect(() => {
    // React 18.x Strict.Mode has a new check for `Ensuring reusable state` to facilitate an upcoming react feature.
    // https://reactjs.org/docs/strict-mode.html#ensuring-reusable-state
    // This will remount all the useEffects to ensure that there are no unexpected side effects.
    // When react remounts the signIn hook of the AuthProvider, it will cause a race condition. Hence, we have to
    // prevent the re-render of this hook as suggested in the following discussion.
    // https://github.com/reactwg/react-18/discussions/18#discussioncomment-795623
    if (reRenderCheckRef.current) {
      return;
    }

    reRenderCheckRef.current = true;

    (async (): Promise<void> => {
      // Sync session state whenever sign-in completes (both redirect and embedded V2 flows).
      // Pass the user returned by the SDK's sign-in flow so SCIM2/Me result is not discarded.
      await client.on('sign-in', async () => {
        await updateSession();
      });

      // User is already authenticated. Skip...
      const isAlreadySignedIn: boolean = await client.isSignedIn();

      // Start auto-refresh with a soft failure.
      const scheduleAutoRefresh = async (): Promise<void> => {
        try {
          await client.startAutoRefreshToken();
        } catch (error) {
          logger.warn('Failed to schedule automatic token refresh.', error);
        }
      };

      // Restore session state and kick off the refresh timer.
      const resumeSession = async (): Promise<void> => {
        await updateSession();
        await scheduleAutoRefresh();
      };

      if (isAlreadySignedIn) {
        await resumeSession();
      }

      // The access token may have expired while the refresh token is still valid.
      // Attempt a silent refresh — startAutoRefreshToken() calls refreshAccessToken()
      // immediately when timeUntilRefresh <= 0, then re-check sign-in status.
      await scheduleAutoRefresh();

      if (await client.isSignedIn()) {
        await resumeSession();
        return;
      }

      const currentUrl: URL = new URL(window.location.href);
      const hasAuthParamsResult: boolean =
        hasAuthParams(currentUrl, config.afterSignInUrl!) && hasCalledForThisInstance(currentUrl, instanceId ?? 0);

      if (hasAuthParamsResult) {
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
            'react',
            'An error occurred while trying to sign in.',
          );
        }
      }
    })();
  }, []);

  /**
   * Check if the user is signed in and update the state accordingly.
   * This will also set an interval to check for the sign-in status every second
   * until the user is signed in.
   */
  useEffect(() => {
    let interval: NodeJS.Timeout;

    (async (): Promise<void> => {
      try {
        const status: boolean = await client.isSignedIn();

        setIsSignedInSync(status);

        if (!status) {
          interval = setInterval(async () => {
            const newStatus: boolean = await client.isSignedIn();

            if (newStatus) {
              setIsSignedInSync(true);
              clearInterval(interval);
            }
          }, 1000);
        } else {
          // TODO: Add a debug log to indicate that the user is already signed in.
        }
      } catch (error) {
        setIsSignedInSync(false);
      }
    })();

    return (): void => {
      if (interval) {
        clearInterval(interval);
      }
    };
  }, [client]);

  useEffect(() => {
    (async (): Promise<void> => {
      try {
        const status: boolean = await client.isInitialized();

        setIsInitializedSync(status);
      } catch (error) {
        setIsInitializedSync(false);
      }
    })();
  }, [client]);

  /**
   * Track loading state changes from the ThunderID client
   */
  useEffect(() => {
    const checkLoadingState = (): void => {
      // Don't override loading state during critical session updates
      if (isUpdatingSession) {
        return;
      }

      // Don't set loading=false while auth params are in the URL and user isn't signed in yet.
      // This prevents ProtectedRoute from redirecting before the sign-in effect processes the auth code.
      const currentUrl: URL = new URL(window.location.href);
      if (!isSignedInSync && hasAuthParams(currentUrl, config.afterSignInUrl!)) {
        return;
      }

      setIsLoadingSync(client.isLoading());
    };

    // Initial check
    checkLoadingState();

    // Set up an interval to check for loading state changes
    const interval: NodeJS.Timeout = setInterval(checkLoadingState, 100);

    return (): void => {
      clearInterval(interval);
    };
  }, [client, isLoadingSync, isSignedInSync, isUpdatingSession]);

  // Branding fetch function
  const fetchBranding: () => Promise<void> = useCallback(async (): Promise<void> => {
    if (!baseUrl) {
      return;
    }

    // Prevent multiple calls if already fetching
    if (isBrandingLoading) {
      return;
    }

    setIsBrandingLoading(true);
    setBrandingError(null);

    try {
      const getBrandingConfig: GetBrandingPreferenceConfig = {
        baseUrl,
        locale: preferences?.i18n?.language,
        // Add other branding config options as needed
      };

      const brandingData: BrandingPreference = await getBrandingPreference(getBrandingConfig);
      setBrandingPreference(brandingData);
      setHasFetchedBranding(true);
    } catch (err) {
      const errorMessage: Error = err instanceof Error ? err : new Error('Failed to fetch branding preference');
      setBrandingError(errorMessage);
      setBrandingPreference(null);
      setHasFetchedBranding(true); // Mark as fetched even on error to prevent retries
    } finally {
      setIsBrandingLoading(false);
    }
  }, [baseUrl, preferences?.i18n?.language]);

  // Refetch branding function
  const refetchBranding: () => Promise<void> = useCallback(async (): Promise<void> => {
    setHasFetchedBranding(false); // Reset the flag to allow refetching
    await fetchBranding();
  }, [fetchBranding]);

  // Auto-fetch branding when initialized and configured
  useEffect(() => {
    // TEMPORARY: Branding preference is not yet supported.
    return;

    // Only fetch branding when explicitly enabled via preferences.theme.inheritFromBranding
    const shouldFetchBranding: boolean = preferences?.theme?.inheritFromBranding === true;

    if (shouldFetchBranding && isInitializedSync && baseUrl && !hasFetchedBranding && !isBrandingLoading) {
      fetchBranding();
    }
  }, [
    preferences?.theme?.inheritFromBranding,
    isInitializedSync,
    baseUrl,
    hasFetchedBranding,
    isBrandingLoading,
    fetchBranding,
  ]);

  const signInSilently = async (options?: SignInOptions): Promise<User | boolean> => {
    try {
      setIsUpdatingSession(true);
      setIsLoadingSync(true);
      return await client.signInSilently(options);
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Error while signing in silently: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
        'thunderid-signInSilently-Error',
        'react',
        'An error occurred while trying to sign in silently.',
      );
    } finally {
      setIsUpdatingSession(false);
      setIsLoadingSync(client.isLoading());
    }
  };

  const switchOrganization = async (organization: Organization): Promise<TokenResponse | Response> => {
    try {
      setIsUpdatingSession(true);
      setIsLoadingSync(true);
      const response: TokenResponse | Response = await client.switchOrganization(organization);

      if (await client.isSignedIn()) {
        await updateSession();
      }

      return response;
    } catch (error) {
      throw new ThunderIDRuntimeError(
        `Failed to switch organization: ${error instanceof Error ? error.message : String(JSON.stringify(error))}`,
        'thunderid-switchOrganization-Error',
        'react',
        'An error occurred while switching to the specified organization.',
      );
    } finally {
      setIsUpdatingSession(false);
      setIsLoadingSync(client.isLoading());
    }
  };

  const handleProfileUpdate = (payload: User): void => {
    setUser(payload);
    setUserProfile((prev: UserProfile | null) => ({
      schemas: prev?.schemas ?? [],
      flattenedProfile: generateFlattenedUserProfile(payload, prev?.schemas ?? []),
      profile: payload,
    }));
  };

  const getDecodedIdToken: () => Promise<IdToken> = useCallback(
    async (): Promise<IdToken> => client.getDecodedIdToken(),
    [client],
  );

  const getIdToken: () => Promise<string> = useCallback(async (): Promise<string> => client.getIdToken(), [client]);

  const getAccessToken: () => Promise<string> = useCallback(
    async (): Promise<string> => client.getAccessToken(),
    [client],
  );

  const getStorageManager: () => Promise<any> = useCallback(async (): Promise<any> => {
    const storageManager: any = storageManagerRef.current ?? (await client.getStorageManager());
    if (storageManager) {
      storageManagerRef.current = storageManager;
    }
    return storageManager;
  }, [client]);

  const request: (...args: any[]) => Promise<any> = useCallback(
    async (...args: any[]): Promise<any> => client.request(...args),
    [client],
  );

  const requestAll: (...args: any[]) => Promise<any> = useCallback(
    async (...args: any[]): Promise<any> => client.requestAll(...args),
    [client],
  );

  const exchangeToken: (exchangeConfig: any) => Promise<any> = useCallback(
    async (exchangeConfig: any): Promise<any> => client.exchangeToken(exchangeConfig),
    [client],
  );

  const signOut: (...args: any[]) => Promise<any> = useCallback(
    async (...args: any[]): Promise<any> => client.signOut(...args),
    [client],
  );

  const recover: (payload: any) => Promise<any> = useCallback(
    async (payload: any): Promise<any> => client.recover(payload),
    [client],
  );

  const signUp: (...args: any[]) => Promise<any> = useCallback(
    async (...args: any[]): Promise<any> => client.signUp(...args),
    [client],
  );

  const clearSession: (...args: any[]) => Promise<any> = useCallback(
    async (...args: any[]): Promise<any> => client.clearSession(...args),
    [client],
  );

  const reInitialize: (reInitConfig: any) => Promise<any> = useCallback(
    async (reInitConfig: any): Promise<any> => client.reInitialize(reInitConfig),
    [client],
  );

  const value: any = useMemo(
    () => ({
      afterSignInUrl: config.afterSignInUrl,
      applicationId: config.applicationId,
      baseUrl,
      scopes: config.scopes,
      clearSession,
      clientId,
      discovery: {
        wellKnown,
      },
      exchangeToken,
      getAccessToken,
      getDecodedIdToken,
      getIdToken,
      getStorageManager,
      http: {
        request,
        requestAll,
      },
      instanceId,
      isInitialized: isInitializedSync,
      isLoading: isLoadingSync,
      isSignedIn: isSignedInSync,
      organization: currentOrganization,
      organizationChain,
      organizationHandle: config?.organizationHandle,
      platform: Platform.ThunderID,
      reInitialize,
      recover,
      signIn,
      signInOptions,
      tokenRequest,
      signInSilently,
      signInUrl,
      signOut,
      signUp,
      signUpUrl,
      switchOrganization,
      syncSession,
      user,
    }),
    [
      applicationId,
      config?.organizationHandle,
      config.afterSignInUrl,
      config.scopes,
      signInUrl,
      signUpUrl,
      baseUrl,
      clientId,
      wellKnown,
      isInitializedSync,
      isLoadingSync,
      isSignedInSync,
      currentOrganization,
      signIn,
      signInSilently,
      user,
      client,
      signInOptions,
      tokenRequest,
      syncSession,
      switchOrganization,
      getDecodedIdToken,
      clearSession,
      exchangeToken,
      getAccessToken,
      getStorageManager,
      instanceId,
      organizationChain,
      recover,
      reInitialize,
      request,
      requestAll,
      signOut,
      signUp,
    ],
  );

  return (
    <ThunderIDContext.Provider value={value}>
      <I18nProvider preferences={preferences?.i18n}>
        <FlowMetaProvider enabled={preferences?.resolveFromMeta !== false}>
          <BrandingProvider
            brandingPreference={brandingPreference}
            isLoading={isBrandingLoading}
            error={brandingError}
            enabled={preferences?.theme?.inheritFromBranding === true}
            refetch={refetchBranding}
          >
            <ThemeProvider
              inheritFromBranding={preferences?.theme?.inheritFromBranding}
              theme={{
                ...preferences?.theme?.overrides,
                direction: preferences?.theme?.direction,
              }}
              mode={getActiveTheme(preferences?.theme?.mode ?? 'light')}
            >
              <FlowProvider>
                <UserProvider profile={userProfile!} onUpdateProfile={handleProfileUpdate}>
                  <OrganizationProvider
                    getAllOrganizations={async (): Promise<AllOrganizationsApiResponse> => client.getAllOrganizations()}
                    myOrganizations={myOrganizations}
                    currentOrganization={currentOrganization}
                    onOrganizationSwitch={switchOrganization}
                    revalidateMyOrganizations={async (): Promise<Organization[]> => client.getMyOrganizations()}
                  >
                    <ComponentRendererProvider renderers={(extensions?.components?.renderers ?? {}) as any}>
                      {children}
                    </ComponentRendererProvider>
                  </OrganizationProvider>
                </UserProvider>
              </FlowProvider>
            </ThemeProvider>
          </BrandingProvider>
        </FlowMetaProvider>
      </I18nProvider>
    </ThunderIDContext.Provider>
  );
};

export default ThunderIDProvider;
