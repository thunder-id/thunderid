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

import {UpdateMeProfileConfig, User, UserProfile} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement, useMemo} from 'react';
import UserContext from './UserContext';

/**
 * Props interface of {@link UserProvider}
 */
export interface UserProviderProps {
  onUpdateProfile?: (payload: User) => void;
  profile: UserProfile;
  revalidateProfile?: () => Promise<void>;
  updateProfile?: (
    requestConfig: UpdateMeProfileConfig,
    sessionId?: string,
  ) => Promise<{data: {user: User}; error: string; success: boolean}>;
}

/**
 * UserProvider component that manages user profile data and provides it through UserContext.
 *
 * This provider:
 * - Fetches user profile data from the ME endpoint
 * - Retrieves SCIM2 schemas for profile structure
 * - Generates both nested and flattened user profiles
 * - Provides functions for refreshing and updating user data
 * - Handles loading states and errors
 *
 * @example
 * ```tsx
 * // Basic usage
 * <UserProvider>
 *   <App />
 * </UserProvider>
 *
 * // With custom error handling
 * <UserProvider onError={(error) => console.error('User error:', error)}>
 *   <App />
 * </UserProvider>
 *
 * // Disable auto-fetch (fetch manually using refreshUser)
 * <UserProvider autoFetch={false}>
 *   <App />
 * </UserProvider>
 * ```
 */
const UserProvider: FC<PropsWithChildren<UserProviderProps>> = ({
  children,
  profile,
  revalidateProfile,
  onUpdateProfile,
  updateProfile,
}: PropsWithChildren<UserProviderProps>): ReactElement => {
  const contextValue: any = useMemo(
    () => ({
      flattenedProfile: profile?.flattenedProfile,
      onUpdateProfile,
      profile: profile?.profile,
      revalidateProfile,
      schemas: profile?.schemas,
      updateProfile,
    }),
    [profile, onUpdateProfile, revalidateProfile, updateProfile],
  );

  return <UserContext.Provider value={contextValue}>{children}</UserContext.Provider>;
};

export default UserProvider;
