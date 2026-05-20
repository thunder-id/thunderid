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

'use server';

import {BrandingPreference, ThunderIDRuntimeError, IdToken, Organization, User, UserProfile} from '@thunderid/node';
import {ThunderIDProviderProps} from '@thunderid/react';
import {FC, PropsWithChildren, ReactElement} from 'react';
import clearSession from './actions/clearSession';
import createOrganization from './actions/createOrganization';
import getAllOrganizations from './actions/getAllOrganizations';
import getBrandingPreference from './actions/getBrandingPreference';
import getCurrentOrganizationAction from './actions/getCurrentOrganizationAction';
import getMyOrganizations from './actions/getMyOrganizations';
import getSessionId from './actions/getSessionId';
import getSessionPayload from './actions/getSessionPayload';
import getUserAction from './actions/getUserAction';
import getUserProfileAction from './actions/getUserProfileAction';
import handleOAuthCallbackAction from './actions/handleOAuthCallbackAction';
import isSignedIn from './actions/isSignedIn';
import refreshToken from './actions/refreshToken';
import signInAction from './actions/signInAction';
import signOutAction from './actions/signOutAction';
import signUpAction from './actions/signUpAction';
import switchOrganization from './actions/switchOrganization';
import updateUserProfileAction from './actions/updateUserProfileAction';
import ThunderIDClientProvider from '../client/contexts/ThunderID/ThunderIDProvider.js';
import {ThunderIDNextConfig} from '../models/config';
import getClient from './getClient';
import logger from '../utils/logger';
import {SessionTokenPayload} from '../utils/SessionManager';

/**
 * Props interface of {@link ThunderIDServerProvider}
 */
export type ThunderIDServerProviderProps = Partial<ThunderIDProviderProps> & {
  clientSecret?: string;
  /**
   * Session cookie lifetime in seconds. Determines how long the session cookie
   * remains valid in the browser after sign-in.
   *
   * Resolution order (first defined value wins):
   *   1. This prop — set here when mounting the provider.
   *   2. `THUNDERID_SESSION_COOKIE_EXPIRY_TIME` environment variable.
   *   3. Built-in default of 86400 seconds (24 hours).
   *
   * @example
   * // 8-hour session cookie
   * <ThunderIDServerProvider sessionCookieExpiryTime={28800} ... />
   */
  sessionCookieExpiryTime?: number;
};

/**
 * Server-side provider component for ThunderID authentication.
 * Wraps the client-side provider and handles server-side authentication logic.
 * Uses the singleton ThunderIDNextClient instance for consistent authentication state.
 *
 * @param props - Props injected into the component.
 *
 * @example
 * ```tsx
 * <ThunderIDServerProvider config={thunderidConfig}>
 *   <YourApp />
 * </ThunderIDServerProvider>
 * ```
 *
 * @returns ThunderIDServerProvider component.
 */
const ThunderIDServerProvider: FC<PropsWithChildren<ThunderIDServerProviderProps>> = async ({
  children,
  afterSignInUrl,
  afterSignOutUrl,
  ..._config
}: PropsWithChildren<ThunderIDServerProviderProps>): Promise<ReactElement> => {
  const thunderIDClient = getClient();
  let config: Partial<ThunderIDNextConfig> = {};

  try {
    await thunderIDClient.initialize(_config as ThunderIDNextConfig);

    logger.debug('[ThunderIDServerProvider] ThunderID client initialized successfully.');

    config = await thunderIDClient.getConfiguration();
  } catch (error) {
    logger.error('[ThunderIDServerProvider] Failed to initialize ThunderID client:', error?.toString());

    throw new ThunderIDRuntimeError(
      `Failed to initialize ThunderID client: ${error?.toString()}`,
      'next-ConfigurationError-001',
      'next',
      'An error occurred while initializing the ThunderID client. Please check your configuration.',
    );
  }

  if (!thunderIDClient.isInitialized) {
    return <></>;
  }

  // Try to get session information from JWT first, then fall back to legacy
  const sessionPayload: SessionTokenPayload | undefined = await getSessionPayload();
  const sessionId: string = sessionPayload?.sessionId || (await getSessionId()) || '';
  const signedIn: boolean = await isSignedIn(sessionId);

  let user: User = {};
  let userProfile: UserProfile = {
    flattenedProfile: {},
    profile: {},
    schemas: [],
  };
  let currentOrganization: Organization = {
    id: '',
    name: '',
    orgHandle: '',
  };
  let myOrganizations: Organization[] = [];
  let brandingPreference: BrandingPreference | null = null;

  if (signedIn) {
    let updatedBaseUrl: string | undefined = config?.baseUrl;

    if (sessionPayload?.organizationId) {
      updatedBaseUrl = `${config?.baseUrl}/o`;
      config = {...config, baseUrl: updatedBaseUrl};
    } else if (sessionId) {
      try {
        const idToken: IdToken = await thunderIDClient.getDecodedIdToken(sessionId);
        if (idToken?.['user_org']) {
          updatedBaseUrl = `${config?.baseUrl}/o`;
          config = {...config, baseUrl: updatedBaseUrl};
        }
      } catch {
        // Continue without organization info
      }
    }

    // Check if user profile fetching is enabled (default: true)
    const shouldFetchUserProfile: boolean = config?.preferences?.user?.fetchUserProfile !== false;
    // Check if organization fetching is enabled (default: true)
    const shouldFetchOrganizations: boolean = config?.preferences?.user?.fetchOrganizations !== false;

    if (shouldFetchUserProfile) {
      try {
        const userResponse: {
          data: {user: User | null};
          error: string | null;
          success: boolean;
        } = await getUserAction(sessionId);
        const userProfileResponse: {
          data: {userProfile: UserProfile};
          error: string | null;
          success: boolean;
        } = await getUserProfileAction(sessionId);

        user = userResponse.data?.user || {};
        userProfile = userProfileResponse.data?.userProfile ?? userProfile;
      } catch (error) {
        logger.warn('[ThunderIDServerProvider] Failed to fetch user profile from SCIM2:', error?.toString());
      }
    }

    if (shouldFetchOrganizations) {
      try {
        const currentOrganizationResponse: {
          data: {organization?: Organization; user?: Record<string, unknown>};
          error: string | null;
          success: boolean;
        } = await getCurrentOrganizationAction(sessionId);

        if (sessionId) {
          myOrganizations = await getMyOrganizations({}, sessionId);
        } else {
          logger.warn('[ThunderIDServerProvider] No session ID available, skipping organization fetch');
        }

        currentOrganization = currentOrganizationResponse?.data?.organization!;
      } catch (error) {
        logger.warn('[ThunderIDServerProvider] Failed to fetch organization info:', error?.toString());
      }
    }
  }

  // Fetch branding preference if branding is enabled in config
  if (config?.preferences?.theme?.inheritFromBranding !== false) {
    try {
      brandingPreference = await getBrandingPreference(
        {
          baseUrl: config?.baseUrl!,
          locale: 'en-US',
          name: config.applicationId || config.organizationHandle,
          type: config.applicationId ? 'APP' : 'ORG',
        },
        sessionId,
      );
    } catch (error) {
      // eslint-disable-next-line no-console
      console.warn('[ThunderIDServerProvider] Failed to fetch branding preference:', error);
    }
  }

  return (
    <ThunderIDClientProvider
      organizationHandle={config?.organizationHandle}
      applicationId={config?.applicationId}
      baseUrl={config?.baseUrl}
      signIn={signInAction}
      clearSession={clearSession}
      refreshToken={refreshToken}
      signOut={signOutAction}
      signUp={signUpAction}
      handleOAuthCallback={handleOAuthCallbackAction}
      signInUrl={config?.signInUrl}
      signUpUrl={config?.signUpUrl}
      preferences={config?.preferences}
      clientId={config?.clientId}
      user={user}
      currentOrganization={currentOrganization}
      userProfile={userProfile}
      updateProfile={updateUserProfileAction}
      isSignedIn={signedIn}
      myOrganizations={myOrganizations}
      getAllOrganizations={getAllOrganizations}
      switchOrganization={switchOrganization}
      brandingPreference={brandingPreference}
      createOrganization={createOrganization}
    >
      {children}
    </ThunderIDClientProvider>
  );
};

export default ThunderIDServerProvider;
