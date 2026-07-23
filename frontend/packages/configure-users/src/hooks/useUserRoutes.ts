/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in Storybook or a unit test), `useUserRoutes` falls back to
 * `defaultUserRoutePaths` below.
 *
 * @public
 */
export interface UserRoutePaths {
  users: {
    list: () => string;
    detail: (userId: string) => string;
    add: () => string;
    addCreate: () => string;
    addInvite: () => string;
  };
}

/**
 * Default user paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultUserRoutePaths: UserRoutePaths = {
  users: {
    list: () => '/users',
    detail: (userId) => `/users/${userId}`,
    add: () => '/users/add',
    addCreate: () => '/users/add/create',
    addInvite: () => '/users/add/invite',
  },
};

/**
 * Resolves the user route paths, preferring the host application's configuration (supplied via
 * `RoutesProvider`) and falling back to this package's own defaults.
 *
 * Components should never hardcode user destination paths; they should call this hook and build
 * the destination from the returned functions instead.
 *
 * @public
 */
export default function useUserRoutes(): UserRoutePaths['users'] {
  const routes = useRoutes<Partial<UserRoutePaths>>();
  return routes.users ?? defaultUserRoutePaths.users;
}
