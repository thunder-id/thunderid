// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoader} from '@thunderid/components';
import {CallbackRoute} from '@thunderid/react-router';
import {lazy, Suspense, type JSX} from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router';
import RouteConfig from './configs/RouteConfig';
import DefaultLayout from './layouts/DefaultLayout';

const AcceptInvitePage = lazy(() => import('./pages/AcceptInvitePage'));
const ErrorPage = lazy(() => import('./pages/ErrorPage'));
const SignOutPage = lazy(() => import('./pages/SignOutPage'));
const RecoveryPage = lazy(() => import('./pages/RecoveryPage'));
const SignInPage = lazy(() => import('./pages/SignInPage'));
const SignUpPage = lazy(() => import('./pages/SignUpPage'));
const DevicePage = lazy(() => import('./pages/DevicePage'));

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route path={RouteConfig.root()} element={<DefaultLayout />}>
            <Route path={RouteConfig.root()} element={<Navigate to={RouteConfig.signIn()} replace />} />
            <Route path={RouteConfig.signIn()} element={<SignInPage />} />
            <Route path={RouteConfig.device()} element={<DevicePage />} />
            <Route path={RouteConfig.signUp()} element={<SignUpPage />} />
            <Route path={RouteConfig.invite()} element={<AcceptInvitePage />} />
            <Route path={RouteConfig.recovery()} element={<RecoveryPage />} />
            <Route path={RouteConfig.signout()} element={<SignOutPage />} />
            <Route path={RouteConfig.callback()} element={<CallbackRoute />} />
            <Route path={RouteConfig.error()} element={<ErrorPage />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
