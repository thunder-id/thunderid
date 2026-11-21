// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProtectedRoute} from '@thunderid/react-router';
import type {JSX} from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router';
import RouteConfig from './configs/RouteConfig';
import AccountLayout from './layouts/AccountLayout';
import {HomePage, ProfilePage, SecurityPage} from './pages/AccountPages';

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <Routes>
        <Route
          element={
            <ProtectedRoute>
              <AccountLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<HomePage />} />
          <Route path={RouteConfig.profile()} element={<ProfilePage />} />
          <Route path={RouteConfig.security()} element={<SecurityPage />} />
          <Route path="*" element={<Navigate to={RouteConfig.home()} replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
