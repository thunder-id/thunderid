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

import {Navigate} from '@tanstack/react-router';
import {useThunderID, ThunderIDRuntimeError} from '@thunderid/react';
import {FC, ReactElement, ReactNode} from 'react';

/**
 * Props for the ProtectedRoute component.
 */
export interface ProtectedRouteProps {
  /**
   * The element to render when the user is authenticated.
   */
  children: ReactElement;
  /**
   * Custom fallback element to render when the user is not authenticated.
   * If provided, this takes precedence over redirectTo.
   */
  fallback?: ReactElement;
  /**
   * Custom loading element to render while authentication status is being determined.
   */
  loader?: ReactNode;
  /**
   * URL to redirect to when the user is not authenticated.
   * Required unless a fallback element is provided.
   */
  redirectTo?: string;
}

/**
 * A protected route component that requires authentication to access.
 *
 * This component should be used as the component prop of a TanStack Router route.
 * It checks authentication status and either renders the protected content,
 * shows a loading state, redirects, or shows a fallback.
 *
 * Either a `redirectTo` prop or a `fallback` prop must be provided to handle
 * unauthenticated users.
 *
 * @example Basic usage with redirect
 * ```tsx
 * const dashboardRoute = createRouteConfig({
 *   path: '/dashboard',
 *   component: () => (
 *     <ProtectedRoute redirectTo="/signin">
 *       <Dashboard />
 *     </ProtectedRoute>
 *   )
 * })
 * ```
 *
 * @example With custom fallback
 * ```tsx
 * const adminRoute = createRouteConfig({
 *   path: '/admin',
 *   component: () => (
 *     <ProtectedRoute fallback={<div>Access denied</div>}>
 *       <AdminPanel />
 *     </ProtectedRoute>
 *   )
 * })
 * ```
 */
const ProtectedRoute: FC<ProtectedRouteProps> = ({
  children,
  fallback,
  redirectTo,
  loader = null,
}: ProtectedRouteProps) => {
  const {isSignedIn, isLoading} = useThunderID();

  if (isLoading) {
    return loader;
  }

  if (isSignedIn) {
    return children;
  }

  if (fallback) {
    return fallback;
  }

  if (redirectTo) {
    return <Navigate to={redirectTo} />;
  }

  throw new ThunderIDRuntimeError(
    '"fallback" or "redirectTo" prop is required.',
    'ProtectedRoute-ValidationError-001',
    'tanstack-router',
    'Either "fallback" or "redirectTo" prop must be provided to handle unauthenticated users.',
  );
};

export default ProtectedRoute;
