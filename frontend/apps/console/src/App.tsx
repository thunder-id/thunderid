/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {ProtectedRoute} from '@thunderid/react-router';
import {ViewAgentTypePage} from '@thunderid/configure-agent-types';
import {
  CreateOrganizationUnitPage,
  OrganizationUnitProvider,
  OrganizationUnitEditPage,
  OrganizationUnitsListPage,
} from '@thunderid/configure-organization-units';
import {
  TranslationCreateProvider,
  TranslationCreatePage,
  TranslationsEditPage,
  TranslationsListPage,
} from '@thunderid/configure-translations';
import {
  UserCreateProvider,
  UserCreatePage,
  UserEditPage,
  UserInvitePage,
  UsersListPage,
} from '@thunderid/configure-users';
import {ToastProvider} from '@thunderid/contexts';
import type {JSX} from 'react';
import {BrowserRouter, Route, Routes} from 'react-router';
import AgentCreateProvider from './features/agents/contexts/AgentCreate/AgentCreateProvider';
import AgentCreatePage from './features/agents/pages/AgentCreatePage';
import AgentEditPage from './features/agents/pages/AgentEditPage';
import AgentsListPage from './features/agents/pages/AgentsListPage';
import ApplicationCreateProvider from './features/applications/contexts/ApplicationCreate/ApplicationCreateProvider';
import ApplicationCreatePage from './features/applications/pages/ApplicationCreatePage';
import ApplicationEditPage from './features/applications/pages/ApplicationEditPage';
import ApplicationsListPage from './features/applications/pages/ApplicationsListPage';
import LayoutBuilderProvider from './features/design/contexts/LayoutBuilder/LayoutBuilderProvider';
import ThemeBuilderProvider from './features/design/contexts/ThemeBuilder/ThemeBuilderProvider';
import DesignPage from './features/design/pages/DesignPage';
import LayoutBuilderPage from './features/design/pages/LayoutBuilderPage';
import ThemeBuilderPage from './features/design/pages/ThemeBuilderPage';
import ThemeCreatePage from './features/design/pages/ThemeCreatePage';
import FlowCreatePage from './features/flows/pages/FlowCreatePage';
import FlowsListPage from './features/flows/pages/FlowsListPage';
import GroupCreateProvider from './features/groups/contexts/GroupCreate/GroupCreateProvider';
import CreateGroupPage from './features/groups/pages/CreateGroupPage';
import GroupEditPage from './features/groups/pages/GroupEditPage';
import GroupsListPage from './features/groups/pages/GroupsListPage';
import HomePage from './features/home/pages/HomePage';
import ExportPage from './features/import-export/pages/ExportPage';
import ImportConfigurationSummaryPage from './features/import-export/pages/ImportConfigurationSummaryPage';
import ImportConfigurationUploadPage from './features/import-export/pages/ImportConfigurationUploadPage';
import ImportConfigurationValidatePage from './features/import-export/pages/ImportConfigurationValidatePage';
import IntegrationsPage from './features/integrations/pages/IntegrationsPage';
import LoginFlowBuilderPage from './features/login-flow/pages/LoginFlowPage';
import RoleCreateProvider from './features/roles/contexts/RoleCreate/RoleCreateProvider';
import CreateRolePage from './features/roles/pages/CreateRolePage';
import RoleEditPage from './features/roles/pages/RoleEditPage';
import RolesListPage from './features/roles/pages/RolesListPage';
import UserTypeCreateProvider from './features/user-types/contexts/UserTypeCreate/UserTypeCreateProvider';
import CreateUserTypePage from './features/user-types/pages/CreateUserTypePage';
import UserTypesListPage from './features/user-types/pages/UserTypesListPage';
import ViewUserTypePage from './features/user-types/pages/ViewUserTypePage';
import WelcomeRedirect from './features/welcome/components/WelcomeRedirect';
import CreateProjectPage from './features/welcome/pages/CreateProjectPage';
import WelcomePage from './features/welcome/pages/WelcomePage';
import DashboardLayout from './layouts/DashboardLayout';
import FullScreenLayout from './layouts/FullScreenLayout';

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <ToastProvider>
        <WelcomeRedirect />
        <Routes>
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<HomePage />} />
            <Route path="home" element={<HomePage />} />
            <Route path="users" element={<UsersListPage />} />
            <Route path="users/:userId" element={<UserEditPage />} />
            <Route path="user-types" element={<UserTypesListPage />} />
            <Route path="user-types/:id" element={<ViewUserTypePage />} />
            <Route path="agent-types/:id" element={<ViewAgentTypePage />} />
            <Route path="integrations" element={<IntegrationsPage />} />
            <Route path="groups" element={<GroupsListPage />} />
            <Route path="groups/:groupId" element={<GroupEditPage />} />
            <Route path="roles" element={<RolesListPage />} />
            <Route path="roles/:roleId" element={<RoleEditPage />} />
            <Route path="applications" element={<ApplicationsListPage />} />
            <Route path="applications/:applicationId" element={<ApplicationEditPage />} />
            <Route path="agents" element={<AgentsListPage />} />
            <Route path="agents/:agentId" element={<AgentEditPage />} />
            <Route path="flows" element={<FlowsListPage />} />
          </Route>
          {/* Organization Units - wrapped in OrganizationUnitProvider to preserve tree state across navigation */}
          <Route
            path="/organization-units"
            element={
              <ProtectedRoute>
                <OrganizationUnitProvider />
              </ProtectedRoute>
            }
          >
            <Route element={<DashboardLayout />}>
              <Route index element={<OrganizationUnitsListPage />} />
              <Route path=":id" element={<OrganizationUnitEditPage />} />
            </Route>
            <Route path="create" element={<FullScreenLayout />}>
              <Route index element={<CreateOrganizationUnitPage />} />
            </Route>
          </Route>
          <Route
            path="/groups/create"
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
            path="/roles/create"
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
          <Route
            path="/users/create"
            element={
              <ProtectedRoute>
                <UserCreateProvider>
                  <FullScreenLayout />
                </UserCreateProvider>
              </ProtectedRoute>
            }
          >
            <Route index element={<UserCreatePage />} />
          </Route>
          <Route
            path="/users/invite"
            element={
              <ProtectedRoute>
                <FullScreenLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<UserInvitePage />} />
          </Route>
          <Route
            path="/user-types/create"
            element={
              <ProtectedRoute>
                <UserTypeCreateProvider>
                  <FullScreenLayout />
                </UserTypeCreateProvider>
              </ProtectedRoute>
            }
          >
            <Route index element={<CreateUserTypePage />} />
          </Route>
          <Route
            path="/applications/create"
            element={
              <ProtectedRoute>
                <ApplicationCreateProvider>
                  <FullScreenLayout />
                </ApplicationCreateProvider>
              </ProtectedRoute>
            }
          >
            <Route index element={<ApplicationCreatePage />} />
          </Route>
          <Route
            path="/agents/create"
            element={
              <ProtectedRoute>
                <AgentCreateProvider>
                  <FullScreenLayout />
                </AgentCreateProvider>
              </ProtectedRoute>
            }
          >
            <Route index element={<AgentCreatePage />} />
          </Route>
          <Route
            path="/flows/create"
            element={
              <ProtectedRoute>
                <FullScreenLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<FlowCreatePage />} />
          </Route>
          <Route
            path="/flows/signin"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<LoginFlowBuilderPage />} />
          </Route>
          <Route
            path="/flows/signin/:flowId"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<LoginFlowBuilderPage />} />
          </Route>
          <Route
            path="/flows/registration"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<LoginFlowBuilderPage />} />
          </Route>
          <Route
            path="/flows/registration/:flowId"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<LoginFlowBuilderPage />} />
          </Route>
          <Route
            path="/flows/recovery"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<LoginFlowBuilderPage />} />
          </Route>
          <Route
            path="/flows/recovery/:flowId"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<LoginFlowBuilderPage />} />
          </Route>
          <Route
            path="/export"
            element={
              <ProtectedRoute>
                <FullScreenLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<ExportPage />} />
          </Route>
          <Route
            path="/welcome"
            element={
              <ProtectedRoute>
                <FullScreenLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<WelcomePage />} />
            <Route path="create-project" element={<CreateProjectPage />} />
            <Route path="open-project" element={<ImportConfigurationUploadPage />} />
            <Route path="open-project/validate" element={<ImportConfigurationValidatePage />} />
            <Route path="open-project/summary" element={<ImportConfigurationSummaryPage />} />
          </Route>
          <Route
            path="/design"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<DesignPage />} />
          </Route>
          <Route
            path="/design/themes/create"
            element={
              <ProtectedRoute>
                <FullScreenLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<ThemeCreatePage />} />
          </Route>
          <Route
            path="/design/themes/:themeId"
            element={
              <ProtectedRoute>
                <ThemeBuilderProvider>
                  <DashboardLayout />
                </ThemeBuilderProvider>
              </ProtectedRoute>
            }
          >
            <Route index element={<ThemeBuilderPage />} />
          </Route>
          <Route
            path="/design/layouts/:layoutId"
            element={
              <ProtectedRoute>
                <LayoutBuilderProvider>
                  <DashboardLayout />
                </LayoutBuilderProvider>
              </ProtectedRoute>
            }
          >
            <Route index element={<LayoutBuilderPage />} />
          </Route>
          <Route
            path="/translations/create"
            element={
              <ProtectedRoute>
                <TranslationCreateProvider>
                  <FullScreenLayout />
                </TranslationCreateProvider>
              </ProtectedRoute>
            }
          >
            <Route index element={<TranslationCreatePage />} />
          </Route>
          <Route
            path="/translations"
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            <Route index element={<TranslationsListPage />} />
            <Route path=":language" element={<TranslationsEditPage />} />
          </Route>
        </Routes>
      </ToastProvider>
    </BrowserRouter>
  );
}
