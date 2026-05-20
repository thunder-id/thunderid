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

import {ThunderIDContextProps as ReactContextProps} from '@thunderid/react';
import {Context, createContext} from 'react';
import {RefreshResult} from '../../../server/actions/refreshToken';

/**
 * Props interface of {@link ThunderIDContext}
 */
export type ThunderIDContextProps = Partial<ReactContextProps> & {
  clearSession?: () => Promise<void>;
  refreshToken?: () => Promise<RefreshResult>;
};

/**
 * Context object for managing the Authentication flow builder core context.
 */
const ThunderIDContext: Context<ThunderIDContextProps | null> = createContext<null | ThunderIDContextProps>({
  afterSignInUrl: undefined,
  applicationId: undefined,
  baseUrl: undefined,
  clearSession: () => Promise.resolve(),
  isInitialized: false,
  isLoading: true,
  isSignedIn: false,
  organizationHandle: undefined,
  refreshToken: () => Promise.resolve({expiresAt: 0}),
  signIn: () => Promise.resolve({} as any),
  signInUrl: undefined,
  signOut: () => Promise.resolve({} as any),
  signUp: () => Promise.resolve({} as any),
  signUpUrl: undefined,
  user: null,
});

ThunderIDContext.displayName = 'ThunderIDContext';

export default ThunderIDContext;
