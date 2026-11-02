// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {RoutesProvider} from '@thunderid/contexts';
import {ProtectedRoute} from '@thunderid/react-router';
import type {JSX} from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router';
import RouteConfig, {MANAGED_RESOURCES, ROUTE_SEGMENTS, type ManagedResource} from './configs/RouteConfig';
import AdminLayout from './layouts/AdminLayout';
import {ResourceCreatePage, ResourceDetailsPage, ResourceListPage} from './pages/ResourcePages';

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <RoutesProvider paths={RouteConfig}>
        <Routes>
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <AdminLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<Navigate to={RouteConfig.users.list()} replace />} />
            {MANAGED_RESOURCES.map((resource: ManagedResource) => (
              <Route key={resource} path={ROUTE_SEGMENTS[resource]}>
                <Route index element={<ResourceListPage resource={resource} />} />
                <Route path={ROUTE_SEGMENTS.create} element={<ResourceCreatePage resource={resource} />} />
                <Route path=":id" element={<ResourceDetailsPage resource={resource} />} />
              </Route>
            ))}
            <Route path="*" element={<Navigate to={RouteConfig.users.list()} replace />} />
          </Route>
        </Routes>
      </RoutesProvider>
    </BrowserRouter>
  );
}
