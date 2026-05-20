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

import {User, Schema, UpdateMeProfileConfig} from '@thunderid/browser';
import {Context, createContext} from 'react';

/**
 * Props interface of {@link UserContext}
 */
export interface UserContextProps {
  flattenedProfile: User | null;
  onUpdateProfile: (payload: User) => void;
  profile: User | null;
  revalidateProfile: () => Promise<void>;
  schemas: Schema[] | null;
  updateProfile: (
    requestConfig: UpdateMeProfileConfig,
    sessionId?: string,
  ) => Promise<{data: {user: User}; error: string; success: boolean}>;
}

/**
 * Context object for managing user profile data and related operations.
 */
const UserContext: Context<UserContextProps | null> = createContext<null | UserContextProps>({
  flattenedProfile: null,
  onUpdateProfile: () => null,
  profile: null,
  revalidateProfile: () => null,
  schemas: null,
  updateProfile: () => null,
});

UserContext.displayName = 'UserContext';

export default UserContext;
