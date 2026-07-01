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

import {PageLoader} from '@thunderid/components';
import {OrganizationUnitProvider} from '@thunderid/configure-organization-units';
import {TranslationCreateProvider} from '@thunderid/configure-translations';
import {UserTypeCreateProvider} from '@thunderid/configure-user-types';
import {UserCreateProvider} from '@thunderid/configure-users';
import {ToastProvider} from '@thunderid/contexts';
import {ProtectedRoute} from '@thunderid/react-router';
import {lazy, Suspense, type JSX} from 'react';
import {BrowserRouter, Route, Routes} from 'react-router';
import AgentCreateProvider from './features/agents/contexts/AgentCreate/AgentCreateProvider';
import ApplicationCreateProvider from './features/applications/contexts/ApplicationCreate/ApplicationCreateProvider';
import LayoutBuilderProvider from './features/design/contexts/LayoutBuilder/LayoutBuilderProvider';
import ThemeBuilderProvider from './features/design/contexts/ThemeBuilder/ThemeBuilderProvider';
import GroupCreateProvider from './features/groups/contexts/GroupCreate/GroupCreateProvider';
import RoleCreateProvider from './features/roles/contexts/RoleCreate/RoleCreateProvider';
import WelcomeRedirect from './features/welcome/components/WelcomeRedirect';
import GetStartedPage from './features/welcome/pages/GetStartedPage';
import TryoutSecuringAIAgentsPage from './features/welcome/pages/TryoutSecuringAIAgentsPage';
import TryoutSecuringApplicationPage from './features/welcome/pages/TryoutSecuringApplicationPage';
import TryoutSecuringMCPPage from './features/welcome/pages/TryoutSecuringMCPPage';
import DashboardLayout from './layouts/DashboardLayout';
import FullScreenLayout from './layouts/FullScreenLayout';

const ViewAgentTypePage = lazy(() =>
  import('@thunderid/configure-agent-types').then((m) => ({default: m.ViewAgentTypePage})),
);
const CreateOrganizationUnitPage = lazy(() =>
  import('@thunderid/configure-organization-units').then((m) => ({default: m.CreateOrganizationUnitPage})),
);
const OrganizationUnitEditPage = lazy(() =>
  import('@thunderid/configure-organization-units').then((m) => ({default: m.OrganizationUnitEditPage})),
);
const OrganizationUnitsListPage = lazy(() =>
  import('@thunderid/configure-organization-units').then((m) => ({default: m.OrganizationUnitsListPage})),
);
const TranslationCreatePage = lazy(() =>
  import('@thunderid/configure-translations').then((m) => ({default: m.TranslationCreatePage})),
);
const TranslationsEditPage = lazy(() =>
  import('./lib/monaco-setup').then(() =>
    import('@thunderid/configure-translations').then((m) => ({default: m.TranslationsEditPage})),
  ),
);
const TranslationsListPage = lazy(() =>
  import('@thunderid/configure-translations').then((m) => ({default: m.TranslationsListPage})),
);
const UserCreatePage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserCreatePage})));
const UserEditPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserEditPage})));
const UserInvitePage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserInvitePage})));
const UsersListPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UsersListPage})));
const ResourceServersListPage = lazy(() =>
  import('@thunderid/configure-resource-servers').then((m) => ({default: m.ResourceServersListPage})),
);
const ResourceServerEditPage = lazy(() =>
  import('@thunderid/configure-resource-servers').then((m) => ({default: m.ResourceServerEditPage})),
);
const CreateResourceServerPage = lazy(() =>
  import('@thunderid/configure-resource-servers').then((m) => ({default: m.CreateResourceServerPage})),
);

const AgentCreatePage = lazy(() => import('./features/agents/pages/AgentCreatePage'));
const AgentEditPage = lazy(() => import('./features/agents/pages/AgentEditPage'));
const AgentsListPage = lazy(() => import('./features/agents/pages/AgentsListPage'));
const ApplicationCreatePage = lazy(() => import('./features/applications/pages/ApplicationCreatePage'));
const ApplicationEditPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('./features/applications/pages/ApplicationEditPage')),
);
const ApplicationsListPage = lazy(() => import('./features/applications/pages/ApplicationsListPage'));
const DesignPage = lazy(() => import('./features/design/pages/DesignPage'));
const LayoutBuilderPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('./features/design/pages/LayoutBuilderPage')),
);
const ThemeBuilderPage = lazy(() => import('./features/design/pages/ThemeBuilderPage'));
const ThemeCreatePage = lazy(() => import('./features/design/pages/ThemeCreatePage'));
const FlowCreatePage = lazy(() => import('./features/flows/pages/FlowCreatePage'));
const FlowsListPage = lazy(() => import('./features/flows/pages/FlowsListPage'));
const CreateGroupPage = lazy(() => import('./features/groups/pages/CreateGroupPage'));
const GroupEditPage = lazy(() => import('./features/groups/pages/GroupEditPage'));
const GroupsListPage = lazy(() => import('./features/groups/pages/GroupsListPage'));
const HomePage = lazy(() => import('./features/home/pages/HomePage'));
const ExportPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('./features/import-export/pages/ExportPage')),
);
const ImportConfigurationSummaryPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('./features/import-export/pages/ImportConfigurationSummaryPage')),
);
const ImportConfigurationUploadPage = lazy(
  () => import('./features/import-export/pages/ImportConfigurationUploadPage'),
);
const ImportConfigurationValidatePage = lazy(
  () => import('./features/import-export/pages/ImportConfigurationValidatePage'),
);
const IntegrationsPage = lazy(() => import('./features/integrations/pages/IntegrationsPage'));
const LoginFlowBuilderPage = lazy(() => import('./features/login-flow/pages/LoginFlowPage'));
const CreateRolePage = lazy(() => import('./features/roles/pages/CreateRolePage'));
const RoleEditPage = lazy(() => import('./features/roles/pages/RoleEditPage'));
const RolesListPage = lazy(() => import('./features/roles/pages/RolesListPage'));
const VerifiablePresentationsListPage = lazy(
  () => import('./features/verifiable-presentations/pages/VerifiablePresentationsListPage'),
);
const VerifiablePresentationCreatePage = lazy(
  () => import('./features/verifiable-presentations/pages/VerifiablePresentationCreatePage'),
);
const VerifiablePresentationEditPage = lazy(
  () => import('./features/verifiable-presentations/pages/VerifiablePresentationEditPage'),
);
const VerifiableCredentialsListPage = lazy(
  () => import('./features/verifiable-credentials/pages/VerifiableCredentialsListPage'),
);
const VerifiableCredentialCreatePage = lazy(
  () => import('./features/verifiable-credentials/pages/VerifiableCredentialCreatePage'),
);
const VerifiableCredentialEditPage = lazy(
  () => import('./features/verifiable-credentials/pages/VerifiableCredentialEditPage'),
);
const CreateUserTypePage = lazy(() =>
  import('@thunderid/configure-user-types').then((m) => ({default: m.CreateUserTypePage})),
);
const UserTypesListPage = lazy(() =>
  import('@thunderid/configure-user-types').then((m) => ({default: m.UserTypesListPage})),
);
const ViewUserTypePage = lazy(() =>
  import('@thunderid/configure-user-types').then((m) => ({default: m.ViewUserTypePage})),
);
const CreateProjectPage = lazy(() => import('./features/welcome/pages/CreateProjectPage'));
const WelcomePage = lazy(() => import('./features/welcome/pages/WelcomePage'));

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <ToastProvider>
        <WelcomeRedirect />
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
              <Route path="verifiable-presentations" element={<VerifiablePresentationsListPage />} />
              <Route path="verifiable-presentations/:vpId" element={<VerifiablePresentationEditPage />} />
              <Route path="verifiable-credentials" element={<VerifiableCredentialsListPage />} />
              <Route path="verifiable-credentials/:vcId" element={<VerifiableCredentialEditPage />} />
              <Route path="applications" element={<ApplicationsListPage />} />
              <Route path="applications/:applicationId" element={<ApplicationEditPage />} />
              <Route path="agents" element={<AgentsListPage />} />
              <Route path="agents/:agentId" element={<AgentEditPage />} />
              <Route path="flows" element={<FlowsListPage />} />
              <Route path="resource-servers" element={<ResourceServersListPage />} />
              <Route path="resource-servers/:resourceServerId" element={<ResourceServerEditPage />} />
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
              path="/verifiable-presentations/create"
              element={
                <ProtectedRoute>
                  <FullScreenLayout />
                </ProtectedRoute>
              }
            >
              <Route index element={<VerifiablePresentationCreatePage />} />
            </Route>
            <Route
              path="/verifiable-credentials/create"
              element={
                <ProtectedRoute>
                  <FullScreenLayout />
                </ProtectedRoute>
              }
            >
              <Route index element={<VerifiableCredentialCreatePage />} />
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
              path="/resource-servers/create"
              element={
                <ProtectedRoute>
                  <FullScreenLayout />
                </ProtectedRoute>
              }
            >
              <Route index element={<CreateResourceServerPage />} />
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
              path="/import-configuration"
              element={
                <ProtectedRoute>
                  <FullScreenLayout />
                </ProtectedRoute>
              }
            >
              <Route index element={<ImportConfigurationUploadPage />} />
              <Route path="validate" element={<ImportConfigurationValidatePage />} />
              <Route path="summary" element={<ImportConfigurationSummaryPage />} />
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
              <Route path="import-configuration" element={<ImportConfigurationUploadPage />} />
              <Route path="import-configuration/validate" element={<ImportConfigurationValidatePage />} />
              <Route path="import-configuration/summary" element={<ImportConfigurationSummaryPage />} />
              <Route path="get-started" element={<GetStartedPage />} />
              <Route
                path="get-started/applications/create"
                element={
                  <ApplicationCreateProvider>
                    <ApplicationCreatePage />
                  </ApplicationCreateProvider>
                }
              />
              <Route path="tryout/securing-application" element={<TryoutSecuringApplicationPage />} />
              <Route path="tryout/ai-agents" element={<TryoutSecuringAIAgentsPage />} />
              <Route path="tryout/mcp" element={<TryoutSecuringMCPPage />} />
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
        </Suspense>
      </ToastProvider>
    </BrowserRouter>
  );
}
