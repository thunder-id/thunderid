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

import {ThunderIDError, User} from '@thunderid/browser';
import {FC, ReactElement, useState} from 'react';
// eslint-disable-next-line import/no-named-as-default
import BaseUserProfile, {BaseUserProfileProps} from './BaseUserProfile';
import updateMeProfile from '../../../api/updateMeProfile';
import useThunderID from '../../../contexts/ThunderID/useThunderID';
import useUser from '../../../contexts/User/useUser';
import useTranslation from '../../../hooks/useTranslation';

/**
 * Props for the UserProfile component.
 * Extends BaseUserProfileProps but makes the user prop optional since it will be obtained from useThunderID
 */
export type UserProfileProps = Omit<BaseUserProfileProps, 'user' | 'profile' | 'flattenedProfile' | 'schemas'>;

/**
 * UserProfile component displays the authenticated user's profile information in a
 * structured and styled format. It shows user details such as display name, email,
 * username, and other available profile information from ThunderID.
 *
 * This component is the React-specific implementation that uses the BaseUserProfile
 * and automatically retrieves the user data from ThunderID context if not provided.
 *
 * @example
 * ```tsx
 * // Basic usage - will use user from ThunderID context
 * <UserProfile />
 *
 * // With explicit user data
 * <UserProfile user={specificUser} />
 *
 * // With card layout and custom fallback
 * <UserProfile
 *   cardLayout={true}
 *   fallback={<div>Please sign in to view your profile</div>}
 * />
 *
 * // With field filtering - only show specific fields
 * <UserProfile
 *   showFields={['name.givenName', 'name.familyName', 'emails']}
 * />
 *
 * // With field hiding - hide specific fields
 * <UserProfile
 *   hideFields={['phoneNumbers', 'addresses']}
 * />
 * ```
 */
const UserProfile: FC<UserProfileProps> = ({preferences, ...rest}: UserProfileProps): ReactElement => {
  const {baseUrl, instanceId} = useThunderID();
  const {profile, flattenedProfile, schemas, onUpdateProfile} = useUser();
  const {t} = useTranslation(preferences?.i18n);

  const [error, setError] = useState<string | null>(null);

  const handleProfileUpdate = async (payload: any): Promise<void> => {
    setError(null);

    try {
      const response: User = await updateMeProfile({baseUrl, instanceId, payload});
      onUpdateProfile(response);
    } catch (caughtError: unknown) {
      let message: string = t('user.profile.update.generic.error');

      if (caughtError instanceof ThunderIDError) {
        message = caughtError?.message;
      }

      setError(message);
    }
  };

  return (
    <BaseUserProfile
      profile={profile}
      flattenedProfile={flattenedProfile}
      schemas={schemas}
      onUpdate={handleProfileUpdate}
      error={error}
      preferences={preferences}
      {...rest}
    />
  );
};

export default UserProfile;
