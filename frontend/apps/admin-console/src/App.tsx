// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoader} from '@thunderid/components';
import {GroupCreateProvider} from '@thunderid/configure-groups';
import {RoleCreateProvider} from '@thunderid/configure-roles';
import {RoutesProvider, ToastProvider} from '@thunderid/contexts';
import {ProtectedRoute} from '@thunderid/react-router';
import {lazy, Suspense, type JSX} from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router';
import RouteConfig, {ROUTE_SEGMENTS} from './configs/RouteConfig';
import DashboardLayout from './layouts/DashboardLayout';
import FullScreenLayout from './layouts/FullScreenLayout';

const AgentOnboardPage = lazy(() => import('@thunderid/configure-agents').then((m) => ({default: m.AgentOnboardPage})));
const AgentEditPage = lazy(() =>
  import('./lib/monaco-setup').then(() =>
    import('@thunderid/configure-agents').then((m) => ({default: m.AgentEditPage})),
  ),
);
const AgentsListPage = lazy(() => import('@thunderid/configure-agents').then((m) => ({default: m.AgentsListPage})));
const ViewAgentTypePage = lazy(() =>
  import('@thunderid/configure-agent-types').then((m) => ({default: m.ViewAgentTypePage})),
);
const CreateGroupPage = lazy(() => import('@thunderid/configure-groups').then((m) => ({default: m.CreateGroupPage})));
const GroupEditPage = lazy(() => import('@thunderid/configure-groups').then((m) => ({default: m.GroupEditPage})));
const GroupsListPage = lazy(() => import('@thunderid/configure-groups').then((m) => ({default: m.GroupsListPage})));
const CreateRolePage = lazy(() => import('@thunderid/configure-roles').then((m) => ({default: m.CreateRolePage})));
const RoleEditPage = lazy(() => import('@thunderid/configure-roles').then((m) => ({default: m.RoleEditPage})));
const RolesListPage = lazy(() => import('@thunderid/configure-roles').then((m) => ({default: m.RolesListPage})));
const UserAddPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserAddPage})));
const UserEditPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserEditPage})));
const UsersListPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UsersListPage})));

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <RoutesProvider paths={RouteConfig}>
        <ToastProvider>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route
                path="/"
                element={
                  <ProtectedRoute>
                    <DashboardLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<Navigate to={RouteConfig.users.list()} replace />} />
                <Route path={ROUTE_SEGMENTS.users} element={<UsersListPage />} />
                <Route path={`${ROUTE_SEGMENTS.users}/:userId`} element={<UserEditPage />} />
                <Route path={`${ROUTE_SEGMENTS.agentTypes}/:id`} element={<ViewAgentTypePage />} />
                <Route path={ROUTE_SEGMENTS.agents} element={<AgentsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.agents}/:agentId`} element={<AgentEditPage />} />
                <Route path={ROUTE_SEGMENTS.groups} element={<GroupsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.groups}/:groupId`} element={<GroupEditPage />} />
                <Route path={ROUTE_SEGMENTS.roles} element={<RolesListPage />} />
                <Route path={`${ROUTE_SEGMENTS.roles}/:roleId`} element={<RoleEditPage />} />
              </Route>
              <Route
                path={RouteConfig.users.add()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<UserAddPage />} />
              </Route>
              <Route
                path={RouteConfig.agents.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<AgentOnboardPage />} />
              </Route>
              <Route
                path={RouteConfig.groups.create()}
                element={
                  <ProtectedRoute>
                    <GroupCreateProvider>
                      <FullScreenLayout />
                    </GroupCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateGroupPage />} />
              </Route>
              <Route
                path={RouteConfig.roles.create()}
                element={
                  <ProtectedRoute>
                    <RoleCreateProvider>
                      <FullScreenLayout />
                    </RoleCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateRolePage />} />
              </Route>
            </Routes>
          </Suspense>
        </ToastProvider>
      </RoutesProvider>
    </BrowserRouter>
  );
}
