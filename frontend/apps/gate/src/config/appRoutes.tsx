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

import {CallbackRoute} from '@thunderid/react-router';
import {Navigate, type RouteProps} from 'react-router';
import ROUTES from '../constants/routes';
import DefaultLayout from '../layouts/DefaultLayout';
import AcceptInvitePage from '../pages/AcceptInvitePage';
import ErrorPage from '../pages/ErrorPage';
import RecoveryPage from '../pages/RecoveryPage';
import SignInPage from '../pages/SignInPage';
import SignUpPage from '../pages/SignUpPage';

/**
 * Interface representing an application route configuration.
 * Extends React Router's RouteProps but allows nested children of the same type.
 */
export interface AppRoute extends Omit<RouteProps, 'children'> {
  /**
   * Child routes nested under this route.
   */
  children?: AppRoute[];
}

/**
 * Application routes configuration.
 * Defines the routing structure for the Gate application.
 *
 * @constant
 * @type {AppRoute[]}
 *
 * @example
 * ```tsx
 * import appRoutes from './config/appRoutes';
 *
 * // Use in React Router
 * <Routes>
 *   {appRoutes.map((route) => (
 *     <Route key={route.path} {...route} />
 *   ))}
 * </Routes>
 * ```
 */
const appRoutes: AppRoute[] = [
  {
    path: ROUTES.ROOT,
    element: <DefaultLayout />,
    children: [
      {path: ROUTES.ROOT, element: <Navigate to={ROUTES.AUTH.SIGN_IN} replace />},
      {path: ROUTES.AUTH.SIGN_IN, element: <SignInPage />},
      {path: ROUTES.AUTH.SIGN_UP, element: <SignUpPage />},
      {path: ROUTES.AUTH.INVITE, element: <AcceptInvitePage />},
      {path: ROUTES.AUTH.RECOVERY, element: <RecoveryPage />},
      {path: ROUTES.AUTH.CALLBACK, element: <CallbackRoute />},
      {path: ROUTES.AUTH.ERROR, element: <ErrorPage />},
    ],
  },
];

export default appRoutes;
