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

import {useRouter, useRouterState} from '@tanstack/react-router';
import {Callback} from '@thunderid/react';
import {FC} from 'react';

/**
 * Props for the CallbackRoute component.
 */
export interface CallbackRouteProps {
  /**
   * Callback function called when an error occurs during OAuth processing.
   * @param error - The error that occurred
   */
  onError?: (error: Error) => void;

  /**
   * Optional custom navigation handler.
   * If provided, this will be called instead of the default navigate() behavior.
   * Useful for apps that need custom navigation logic.
   * @param path - The path to navigate to
   */
  onNavigate?: (path: string) => void;
}

/**
 * Handles OAuth callback redirects for TanStack Router applications.
 * Processes authorization code, validates CSRF state, and navigates back to the original path.
 * Automatically handles TanStack Router basepath when configured.
 *
 * @example
 * ```tsx
 * const callbackRoute = createRoute({
 *   getParentRoute: () => rootRoute,
 *   path: '/callback',
 *   component: CallbackRoute,
 * });
 * ```
 */
const CallbackRoute: FC<CallbackRouteProps> = ({onError, onNavigate}: CallbackRouteProps) => {
  const router: ReturnType<typeof useRouter> = useRouter();
  const routerState: ReturnType<typeof useRouterState> = useRouterState();
  const {pathname}: {pathname: string} = routerState.location;

  const handleNavigate = (path: string): void => {
    if (onNavigate) {
      onNavigate(path);
      return;
    }

    const fullPath: string = window.location.pathname;
    const basename: string = fullPath.endsWith(pathname) ? fullPath.slice(0, -pathname.length).replace(/\/$/, '') : '';

    const navigationPath: string = basename && path.startsWith(basename) ? path.slice(basename.length) || '/' : path;

    router.navigate({to: navigationPath}).catch(() => {});
  };

  return (
    <Callback
      onNavigate={handleNavigate}
      onError={
        onError ||
        ((error: Error): void => {
          // eslint-disable-next-line no-console
          console.error('OAuth callback error:', error);
        })
      }
    />
  );
};

export default CallbackRoute;
