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

import {PageLoader} from '@thunderid/components';
import {CallbackRoute} from '@thunderid/react-router';
import {lazy, Suspense, type JSX} from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router';
import ROUTES from './constants/routes';
import DefaultLayout from './layouts/DefaultLayout';

const AcceptInvitePage = lazy(() => import('./pages/AcceptInvitePage'));
const ErrorPage = lazy(() => import('./pages/ErrorPage'));
const SignOutPage = lazy(() => import('./pages/SignOutPage'));
const RecoveryPage = lazy(() => import('./pages/RecoveryPage'));
const SignInPage = lazy(() => import('./pages/SignInPage'));
const SignUpPage = lazy(() => import('./pages/SignUpPage'));

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route path={ROUTES.ROOT} element={<DefaultLayout />}>
            <Route path={ROUTES.ROOT} element={<Navigate to={ROUTES.AUTH.SIGN_IN} replace />} />
            <Route path={ROUTES.AUTH.SIGN_IN} element={<SignInPage />} />
            <Route path={ROUTES.AUTH.SIGN_UP} element={<SignUpPage />} />
            <Route path={ROUTES.AUTH.INVITE} element={<AcceptInvitePage />} />
            <Route path={ROUTES.AUTH.RECOVERY} element={<RecoveryPage />} />
            <Route path={ROUTES.AUTH.SIGNOUT} element={<SignOutPage />} />
            <Route path={ROUTES.AUTH.CALLBACK} element={<CallbackRoute />} />
            <Route path={ROUTES.AUTH.ERROR} element={<ErrorPage />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
