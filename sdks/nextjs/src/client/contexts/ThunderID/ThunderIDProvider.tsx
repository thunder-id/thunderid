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

'use client';

import {
  AllOrganizationsApiResponse,
  EmbeddedFlowExecuteRequestConfig,
  EmbeddedFlowExecuteRequestPayload,
  EmbeddedSignInFlowHandleRequestPayload,
  generateFlattenedUserProfile,
  Organization,
  UpdateMeProfileConfig,
  User,
  UserProfile,
  BrandingPreference,
  TokenResponse,
  CreateOrganizationPayload,
  ThunderIDRuntimeError,
} from '@thunderid/node';
import {
  I18nProvider,
  FlowProvider,
  UserProvider,
  ThemeProvider,
  ThunderIDProviderProps,
  OrganizationProvider,
  BrandingProvider,
  getActiveTheme,
} from '@thunderid/react';
import {ReadonlyURLSearchParams} from 'next/dist/client/components/navigation.react-server';
import {AppRouterInstance} from 'next/dist/shared/lib/app-router-context.shared-runtime';
import {useRouter, useSearchParams} from 'next/navigation';
import {FC, PropsWithChildren, RefObject, useEffect, useMemo, useRef, useState} from 'react';
import ThunderIDContext, {ThunderIDContextProps} from './ThunderIDContext';
import {RefreshResult} from '../../../server/actions/refreshToken';
import logger from '../../../utils/logger';

/**
 * Props interface of {@link ThunderIDClientProvider}
 */
export type ThunderIDClientProviderProps = Partial<Omit<ThunderIDProviderProps, 'baseUrl' | 'clientId'>> &
  Pick<ThunderIDProviderProps, 'baseUrl' | 'clientId'> & {
    applicationId: ThunderIDContextProps['applicationId'];
    brandingPreference?: BrandingPreference | null;
    clearSession: () => Promise<void>;
    createOrganization: (payload: CreateOrganizationPayload, sessionId: string) => Promise<Organization>;
    currentOrganization: Organization;
    getAllOrganizations: (options?: any, sessionId?: string) => Promise<AllOrganizationsApiResponse>;
    handleOAuthCallback: (
      code: string,
      state: string,
      sessionState?: string,
    ) => Promise<{error?: string; redirectUrl?: string; success: boolean}>;
    isSignedIn: boolean;
    myOrganizations: Organization[];
    organizationHandle: ThunderIDContextProps['organizationHandle'];
    refreshToken: () => Promise<RefreshResult>;
    revalidateMyOrganizations?: (sessionId?: string) => Promise<Organization[]>;
    signIn: ThunderIDContextProps['signIn'];
    signOut: ThunderIDContextProps['signOut'];
    signUp: ThunderIDContextProps['signUp'];
    switchOrganization: (organization: Organization, sessionId?: string) => Promise<TokenResponse | Response>;
    updateProfile: (
      requestConfig: UpdateMeProfileConfig,
      sessionId?: string,
    ) => Promise<{data: {user: User}; error: string; success: boolean}>;
    user: User | null;
    userProfile: UserProfile;
  };

const ThunderIDClientProvider: FC<PropsWithChildren<ThunderIDClientProviderProps>> = ({
  baseUrl,
  children,
  signIn,
  clearSession,
  refreshToken,
  signOut,
  signUp,
  handleOAuthCallback,
  createOrganization,
  preferences,
  isSignedIn,
  signInUrl,
  signUpUrl,
  user: _user,
  userProfile: _userProfile,
  currentOrganization,
  updateProfile,
  applicationId,
  organizationHandle,
  myOrganizations,
  revalidateMyOrganizations,
  getAllOrganizations,
  switchOrganization,
  brandingPreference,
}: PropsWithChildren<ThunderIDClientProviderProps>) => {
  const reRenderCheckRef: RefObject<boolean> = useRef(false);
  const router: AppRouterInstance = useRouter();
  const searchParams: ReadonlyURLSearchParams = useSearchParams();
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [user, setUser] = useState<User | null>(_user);
  const [userProfile, setUserProfile] = useState<UserProfile>(_userProfile);

  useEffect(() => {
    setUserProfile(_userProfile);
  }, [_userProfile]);

  useEffect(() => {
    setUser(_user);
  }, [_user]);

  // Handle OAuth callback automatically
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

    // Don't handle callback if already signed in
    if (isSignedIn) return;

    (async (): Promise<void> => {
      try {
        const code: string | null = searchParams.get('code');
        const state: string | null = searchParams.get('state');
        const sessionState: string | null = searchParams.get('session_state');
        const error: string | null = searchParams.get('error');

        // Check for OAuth errors first
        if (error) {
          logger.error('[ThunderIDClientProvider] An error was received for the initiated sign-in request.');

          return;
        }

        // Handle OAuth callback if code and state are present
        if (code && state) {
          setIsLoading(true);

          const result: {error?: string; redirectUrl?: string; success: boolean} = await handleOAuthCallback(
            code,
            state,
            sessionState || undefined,
          );

          if (result.success) {
            // Redirect to the success URL
            if (result.redirectUrl) {
              router.push(result.redirectUrl);
            } else {
              // Refresh the page to update authentication state
              window.location.reload();
            }
          } else {
            logger.error(
              `[ThunderIDClientProvider] An error occurred while signing in: ${result.error || 'Authentication failed'}`,
            );
          }
        }
      } catch (error) {
        logger.error('[ThunderIDClientProvider] Failed to handle OAuth callback:', error);
      }
    })();
  }, []);

  useEffect(() => {
    // Set loading to false when server has resolved authentication state
    setIsLoading(false);
  }, [isSignedIn, user]);

  const handleSignIn = async (
    payload: EmbeddedSignInFlowHandleRequestPayload,
    request: EmbeddedFlowExecuteRequestConfig,
  ): Promise<any> => {
    if (!signIn) {
      throw new ThunderIDRuntimeError(
        '`signIn` function is not available.',
        'ThunderIDClientProvider-handleSignIn-RuntimeError-001',
        'nextjs',
      );
    }

    const result: any = await signIn(payload, request);

    // Redirect based flow URL is sent as `signInUrl` in the response.
    if (result?.data?.signInUrl) {
      router.push(result.data.signInUrl);

      return undefined;
    }

    // After the Embedded flow is successful, the URL to navigate next is sent as `afterSignInUrl` in the response.
    if (result?.data?.afterSignInUrl) {
      router.push(result.data.afterSignInUrl);

      return undefined;
    }

    if (result?.error) {
      throw new Error(result.error);
    }

    return result?.data ?? result;
  };

  const handleSignUp = async (
    payload: EmbeddedFlowExecuteRequestPayload,
    request: EmbeddedFlowExecuteRequestConfig,
  ): Promise<any> => {
    if (!signUp) {
      throw new ThunderIDRuntimeError(
        '`signUp` function is not available.',
        'ThunderIDClientProvider-handleSignUp-RuntimeError-001',
        'nextjs',
      );
    }

    const result: any = await signUp(payload, request);

    // Redirect based flow URL is sent as `signUpUrl` in the response.
    if (result?.data?.signUpUrl) {
      router.push(result.data.signUpUrl);

      return undefined;
    }

    // After the Embedded flow is successful, the URL to navigate next is sent as `afterSignUpUrl` in the response.
    if (result?.data?.afterSignUpUrl) {
      router.push(result.data.afterSignUpUrl);

      return undefined;
    }

    if (result?.error) {
      throw new Error(result.error);
    }

    return result?.data ?? result;
  };

  const handleSignOut = async (): Promise<any> => {
    logger.debug('[ThunderIDClientProvider][handleSignOut] `handleSignOut` called.');

    try {
      const result: any = await signOut();

      logger.debug('[ThunderIDClientProvider][handleSignOut] Sign out result:', result);

      if (result?.data?.afterSignOutUrl) {
        router.push(result.data.afterSignOutUrl);

        return {location: result.data.afterSignOutUrl, redirected: true};
      }

      if (result?.error) {
        logger.error(
          '[ThunderIDClientProvider][handleSignOut] Error result was returned during signing the user out with a button click:',
          result.error,
        );
      }

      return result?.data ?? result;
    } catch (error) {
      logger.error(
        '[ThunderIDClientProvider][handleSignOut] Error occurred during signing the user out with a button click:',
        error,
      );

      return undefined;
    }
  };

  const contextValue: ThunderIDContextProps = useMemo(
    () => ({
      applicationId,
      baseUrl,
      clearSession,
      isLoading,
      isSignedIn,
      organizationHandle,
      refreshToken,
      signIn: handleSignIn,
      signInUrl,
      signOut: handleSignOut,
      signUp: handleSignUp,
      signUpUrl,
      user,
    }),
    [baseUrl, user, isSignedIn, isLoading, signInUrl, signUpUrl, applicationId, organizationHandle],
  );

  const handleProfileUpdate = (payload: User): void => {
    setUser(payload);
    setUserProfile((prev: UserProfile) => ({
      ...prev,
      flattenedProfile: generateFlattenedUserProfile(payload, prev?.schemas),
      profile: payload,
    }));
  };

  return (
    <ThunderIDContext.Provider value={contextValue}>
      <I18nProvider preferences={preferences?.i18n}>
        <BrandingProvider brandingPreference={brandingPreference}>
          <ThemeProvider
            theme={preferences?.theme?.overrides}
            mode={getActiveTheme(preferences?.theme?.mode as any)}
            inheritFromBranding
          >
            <FlowProvider>
              <UserProvider profile={userProfile} onUpdateProfile={handleProfileUpdate} updateProfile={updateProfile}>
                <OrganizationProvider
                  createOrganization={createOrganization}
                  getAllOrganizations={getAllOrganizations}
                  myOrganizations={myOrganizations}
                  currentOrganization={currentOrganization}
                  onOrganizationSwitch={switchOrganization as any}
                  revalidateMyOrganizations={revalidateMyOrganizations as any}
                >
                  {children}
                </OrganizationProvider>
              </UserProvider>
            </FlowProvider>
          </ThemeProvider>
        </BrandingProvider>
      </I18nProvider>
    </ThunderIDContext.Provider>
  );
};

export default ThunderIDClientProvider;
