// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoader} from '@thunderid/components';
import type {NotificationChannel} from '@thunderid/configure-notification-templates';
import {RoutesProvider, ToastProvider} from '@thunderid/contexts';
import {ProtectedRoute} from '@thunderid/react-router';
import {lazy, Suspense, type JSX} from 'react';
import {BrowserRouter, Navigate, Outlet, Route, Routes} from 'react-router';
import RouteConfig, {ROUTE_SEGMENTS} from './configs/RouteConfig';
import DashboardLayout from './layouts/DashboardLayout';
import FullScreenLayout from './layouts/FullScreenLayout';

// The route-scoped providers are loaded lazily, like the pages they wrap. Importing them statically
// would put their packages in the entry graph, and since the pages are imported dynamically from the
// same package entry points, every page of those packages (and the packages they import) would then
// load before the first paint.
const AgentTypeCreateProvider = lazy(
  () => import('@thunderid/configure-agent-types/contexts/AgentTypeCreate/AgentTypeCreateProvider'),
);
const ApplicationCreateProvider = lazy(
  () => import('@thunderid/configure-applications/contexts/ApplicationCreate/ApplicationCreateProvider'),
);
const LayoutBuilderProvider = lazy(
  () => import('@thunderid/configure-design/contexts/LayoutBuilder/LayoutBuilderProvider'),
);
const ThemeBuilderProvider = lazy(
  () => import('@thunderid/configure-design/contexts/ThemeBuilder/ThemeBuilderProvider'),
);
const GroupCreateProvider = lazy(() => import('@thunderid/configure-groups/contexts/GroupCreate/GroupCreateProvider'));
const OrganizationUnitProvider = lazy(
  () => import('@thunderid/configure-organization-units/contexts/OrganizationUnitProvider'),
);
const RoleCreateProvider = lazy(() => import('@thunderid/configure-roles/contexts/RoleCreate/RoleCreateProvider'));
const TranslationCreateProvider = lazy(
  () => import('@thunderid/configure-translations/contexts/TranslationCreate/TranslationCreateProvider'),
);
const UserTypeCreateProvider = lazy(
  () => import('@thunderid/configure-user-types/contexts/UserTypeCreate/UserTypeCreateProvider'),
);
const AgentTypesListPage = lazy(() => import('@thunderid/configure-agent-types/pages/AgentTypesListPage'));
const CreateAgentTypePage = lazy(() => import('@thunderid/configure-agent-types/pages/CreateAgentTypePage'));
const ViewAgentTypePage = lazy(() => import('@thunderid/configure-agent-types/pages/ViewAgentTypePage'));
const CreateOrganizationUnitPage = lazy(
  () => import('@thunderid/configure-organization-units/pages/CreateOrganizationUnitPage'),
);
const OrganizationUnitEditPage = lazy(
  () => import('@thunderid/configure-organization-units/pages/OrganizationUnitEditPage'),
);
const OrganizationUnitsListPage = lazy(
  () => import('@thunderid/configure-organization-units/pages/OrganizationUnitsListPage'),
);
const OrganizationUnitDefaultFlowsSettings = lazy(
  () => import('@thunderid/configure-flows/components/OrganizationUnitDefaultFlowsSettings'),
);
const TranslationCreatePage = lazy(() => import('@thunderid/configure-translations/pages/TranslationCreatePage'));
const TranslationsEditPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('@thunderid/configure-translations/pages/TranslationsEditPage')),
);
const TranslationsListPage = lazy(() => import('@thunderid/configure-translations/pages/TranslationsListPage'));
const NotificationTemplatesListPage = lazy(
  () => import('@thunderid/configure-notification-templates/pages/NotificationTemplatesListPage'),
);
const NotificationTemplateEditPage = lazy(
  () => import('@thunderid/configure-notification-templates/pages/NotificationTemplateEditPage'),
);
const NotificationTemplateCreatePage = lazy(
  () => import('@thunderid/configure-notification-templates/pages/NotificationTemplateCreatePage'),
);
const UserAddPage = lazy(() => import('@thunderid/configure-users/pages/UserAddRoute'));
const UserEditPage = lazy(() => import('@thunderid/configure-users/pages/UserEditPage'));
const UsersListPage = lazy(() => import('@thunderid/configure-users/pages/UsersListPage'));
const ResourceServersListPage = lazy(
  () => import('@thunderid/configure-resource-servers/pages/ResourceServersListPage'),
);
const ResourceServerEditPage = lazy(() => import('@thunderid/configure-resource-servers/pages/ResourceServerEditPage'));
const CreateResourceServerPage = lazy(
  () => import('@thunderid/configure-resource-servers/pages/CreateResourceServerPage'),
);
const AuthorizationDetailTypesListPage = lazy(
  () => import('@thunderid/configure-resource-servers/pages/AuthorizationDetailTypesListPage'),
);
const AuthorizationDetailTypeEditPage = lazy(() =>
  import('./lib/monaco-setup').then(
    () => import('@thunderid/configure-resource-servers/pages/AuthorizationDetailTypeEditPage'),
  ),
);
const CreateAuthorizationDetailTypePage = lazy(() =>
  import('./lib/monaco-setup').then(
    () => import('@thunderid/configure-resource-servers/pages/CreateAuthorizationDetailTypePage'),
  ),
);

const AgentOnboardPage = lazy(() => import('@thunderid/configure-agents/pages/AgentAddRoute'));
const AgentEditPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('@thunderid/configure-agents/pages/AgentEditPage')),
);
const AgentsListPage = lazy(() => import('@thunderid/configure-agents/pages/AgentsListPage'));
const ApplicationCreatePage = lazy(() => import('@thunderid/configure-applications/pages/ApplicationCreatePage'));
const ApplicationEditPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('@thunderid/configure-applications/pages/ApplicationEditPage')),
);
const ApplicationsListPage = lazy(() => import('@thunderid/configure-applications/pages/ApplicationsListPage'));
const ApplicationTemplateSelectPage = lazy(
  () => import('@thunderid/configure-applications/pages/ApplicationTemplateSelectPage'),
);
const DesignPage = lazy(() => import('@thunderid/configure-design/pages/DesignPage'));
const LayoutBuilderPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('@thunderid/configure-design/pages/LayoutBuilderPage')),
);
const ThemeBuilderPage = lazy(() => import('@thunderid/configure-design/pages/ThemeBuilderPage'));
const ThemeCreatePage = lazy(() => import('@thunderid/configure-design/pages/ThemeCreatePage'));
const FlowCreatePage = lazy(() => import('@thunderid/configure-flows/pages/FlowCreatePage'));
const FlowsListPage = lazy(() => import('@thunderid/configure-flows/pages/FlowsListPage'));
const CreateGroupPage = lazy(() => import('@thunderid/configure-groups/pages/CreateGroupPage'));
const GroupEditPage = lazy(() => import('@thunderid/configure-groups/pages/GroupEditPage'));
const GroupsListPage = lazy(() => import('@thunderid/configure-groups/pages/GroupsListPage'));
const HomePage = lazy(() => import('@thunderid/configure-onboarding/pages/HomePage'));
const ExportPage = lazy(() =>
  import('./lib/monaco-setup').then(() => import('@thunderid/configure-import-export/pages/ExportPage')),
);
const ImportConfigurationSummaryPage = lazy(() =>
  import('./lib/monaco-setup').then(
    () => import('@thunderid/configure-import-export/pages/ImportConfigurationSummaryPage'),
  ),
);
const ImportConfigurationUploadPage = lazy(
  () => import('@thunderid/configure-import-export/pages/ImportConfigurationUploadPage'),
);
const ImportConfigurationValidatePage = lazy(
  () => import('@thunderid/configure-import-export/pages/ImportConfigurationValidatePage'),
);
const ImportExportPage = lazy(() => import('@thunderid/configure-import-export/pages/ImportExportPage'));
const ConnectionsListPage = lazy(() => import('@thunderid/configure-connections/pages/ConnectionsListPage'));
const ConnectionDetailPage = lazy(() => import('@thunderid/configure-connections/pages/ConnectionDetailPage'));
const ConnectionConfigureWizardPage = lazy(
  () => import('@thunderid/configure-connections/pages/ConnectionConfigureWizardPage'),
);
const ConnectionCreateWizardPage = lazy(
  () => import('@thunderid/configure-connections/pages/ConnectionCreateWizardPage'),
);
const FlowBuilderPage = lazy(() => import('@thunderid/configure-flows/pages/FlowBuilderPage'));
const CreateRolePage = lazy(() => import('@thunderid/configure-roles/pages/CreateRolePage'));
const RoleEditPage = lazy(() => import('@thunderid/configure-roles/pages/RoleEditPage'));
const RolesListPage = lazy(() => import('@thunderid/configure-roles/pages/RolesListPage'));
const SettingsPage = lazy(() => import('@thunderid/configure-settings/pages/SettingsPage'));
const TrustedIssuerDetailPage = lazy(() => import('@thunderid/configure-connections/pages/TrustedIssuerDetailPage'));
const VerifiablePresentationsListPage = lazy(
  () => import('@thunderid/configure-verifiable-credentials/pages/VerifiablePresentationsListPage'),
);
const VerifiablePresentationCreatePage = lazy(
  () => import('@thunderid/configure-verifiable-credentials/pages/VerifiablePresentationCreatePage'),
);
const VerifiablePresentationEditPage = lazy(
  () => import('@thunderid/configure-verifiable-credentials/pages/VerifiablePresentationEditPage'),
);
const VerifiableCredentialsListPage = lazy(
  () => import('@thunderid/configure-verifiable-credentials/pages/VerifiableCredentialsListPage'),
);
const VerifiableCredentialCreatePage = lazy(
  () => import('@thunderid/configure-verifiable-credentials/pages/VerifiableCredentialCreatePage'),
);
const VerifiableCredentialEditPage = lazy(
  () => import('@thunderid/configure-verifiable-credentials/pages/VerifiableCredentialEditPage'),
);
const CreateUserTypePage = lazy(() => import('@thunderid/configure-user-types/pages/CreateUserTypePage'));
const UserTypesListPage = lazy(() => import('@thunderid/configure-user-types/pages/UserTypesListPage'));
const ViewUserTypePage = lazy(() => import('@thunderid/configure-user-types/pages/ViewUserTypePage'));

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
                <Route index element={<HomePage />} />
                <Route path={ROUTE_SEGMENTS.home} element={<HomePage />} />
                <Route path={ROUTE_SEGMENTS.users} element={<UsersListPage />} />
                <Route path={`${ROUTE_SEGMENTS.users}/:userId`} element={<UserEditPage />} />
                <Route path={ROUTE_SEGMENTS.userTypes} element={<UserTypesListPage />} />
                <Route path={`${ROUTE_SEGMENTS.userTypes}/:id`} element={<ViewUserTypePage />} />
                <Route path={ROUTE_SEGMENTS.agentTypes} element={<AgentTypesListPage />} />
                <Route path={`${ROUTE_SEGMENTS.agentTypes}/:id`} element={<ViewAgentTypePage />} />
                <Route path={ROUTE_SEGMENTS.connections} element={<ConnectionsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.connections}/:type`} element={<ConnectionDetailPage />} />
                <Route path={`${ROUTE_SEGMENTS.connections}/:type/:id`} element={<ConnectionDetailPage />} />
                <Route
                  path={ROUTE_SEGMENTS.trustedIssuers}
                  element={<Navigate to={RouteConfig.connections.list()} replace />}
                />
                <Route path={`${ROUTE_SEGMENTS.trustedIssuers}/:id`} element={<TrustedIssuerDetailPage />} />
                <Route path={ROUTE_SEGMENTS.groups} element={<GroupsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.groups}/:groupId`} element={<GroupEditPage />} />
                <Route path={ROUTE_SEGMENTS.roles} element={<RolesListPage />} />
                <Route path={`${ROUTE_SEGMENTS.roles}/:roleId`} element={<RoleEditPage />} />
                <Route
                  path={ROUTE_SEGMENTS.verifiablePresentations}
                  element={<VerifiablePresentationsListPage showWalletQr />}
                />
                <Route
                  path={`${ROUTE_SEGMENTS.verifiablePresentations}/:vpId`}
                  element={<VerifiablePresentationEditPage />}
                />
                <Route
                  path={ROUTE_SEGMENTS.verifiableCredentials}
                  element={<VerifiableCredentialsListPage showWalletQr />}
                />
                <Route
                  path={`${ROUTE_SEGMENTS.verifiableCredentials}/:vcId`}
                  element={<VerifiableCredentialEditPage />}
                />
                <Route path={ROUTE_SEGMENTS.applications} element={<ApplicationsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.applications}/:applicationId`} element={<ApplicationEditPage />} />
                <Route path={ROUTE_SEGMENTS.agents} element={<AgentsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.agents}/:agentId`} element={<AgentEditPage />} />
                <Route path={ROUTE_SEGMENTS.flows} element={<FlowsListPage />} />
                <Route path={ROUTE_SEGMENTS.resourceServers} element={<ResourceServersListPage />} />
                <Route
                  path={`${ROUTE_SEGMENTS.resourceServers}/:resourceServerId`}
                  element={<ResourceServerEditPage />}
                />
                <Route path={ROUTE_SEGMENTS.authorizationDetailTypes} element={<AuthorizationDetailTypesListPage />} />
                <Route
                  path={`${ROUTE_SEGMENTS.authorizationDetailTypes}/:id`}
                  element={<AuthorizationDetailTypeEditPage />}
                />
                <Route path={ROUTE_SEGMENTS.settings} element={<SettingsPage />} />
              </Route>
              {/* Organization Units - wrapped in OrganizationUnitProvider to preserve tree state across navigation */}
              <Route
                path={RouteConfig.organizationUnits.list()}
                element={
                  <ProtectedRoute>
                    <OrganizationUnitProvider />
                  </ProtectedRoute>
                }
              >
                <Route element={<DashboardLayout />}>
                  <Route index element={<OrganizationUnitsListPage />} />
                  <Route
                    path=":id"
                    element={
                      <OrganizationUnitEditPage
                        renderDefaultFlowsSettings={(props) => <OrganizationUnitDefaultFlowsSettings {...props} />}
                      />
                    }
                  />
                </Route>
                <Route path="create" element={<FullScreenLayout />}>
                  <Route index element={<CreateOrganizationUnitPage />} />
                </Route>
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
                path={RouteConfig.userTypes.create()}
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
                path={RouteConfig.agentTypes.create()}
                element={
                  <ProtectedRoute>
                    <AgentTypeCreateProvider>
                      <FullScreenLayout />
                    </AgentTypeCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateAgentTypePage />} />
              </Route>
              <Route
                path={RouteConfig.verifiablePresentations.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<VerifiablePresentationCreatePage />} />
              </Route>
              <Route
                path={RouteConfig.verifiableCredentials.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<VerifiableCredentialCreatePage />} />
              </Route>
              <Route
                element={
                  <ProtectedRoute>
                    <ApplicationCreateProvider>
                      <Outlet />
                    </ApplicationCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route path={RouteConfig.applications.types()} element={<DashboardLayout />}>
                  <Route index element={<ApplicationTemplateSelectPage />} />
                </Route>
                <Route path={RouteConfig.applications.create()} element={<FullScreenLayout />}>
                  <Route index element={<ApplicationCreatePage />} />
                </Route>
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
                path={RouteConfig.resourceServers.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateResourceServerPage />} />
              </Route>
              <Route
                path={RouteConfig.authorizationDetailTypes.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateAuthorizationDetailTypePage />} />
              </Route>
              <Route
                path={RouteConfig.flows.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<FlowCreatePage />} />
              </Route>
              <Route
                path={RouteConfig.connections.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ConnectionCreateWizardPage />} />
              </Route>
              <Route
                path={`/${ROUTE_SEGMENTS.connections}/:type/configure`}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ConnectionConfigureWizardPage />} />
              </Route>
              <Route
                path={RouteConfig.flows.detail(':flowId')}
                element={
                  <ProtectedRoute>
                    <DashboardLayout collapseSidebar />
                  </ProtectedRoute>
                }
              >
                <Route index element={<FlowBuilderPage />} />
              </Route>
              <Route
                path={RouteConfig.importExport.list()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ImportExportPage />} />
              </Route>
              <Route
                path={RouteConfig.export.page()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ExportPage />} />
              </Route>
              <Route
                path={RouteConfig.importConfiguration.upload()}
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
                path={RouteConfig.design.list()}
                element={
                  <ProtectedRoute>
                    <DashboardLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<DesignPage />} />
              </Route>
              <Route
                path={RouteConfig.design.themesCreate()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ThemeCreatePage />} />
              </Route>
              <Route
                path={`/${ROUTE_SEGMENTS.design}/themes/:themeId`}
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
                path={`/${ROUTE_SEGMENTS.design}/layouts/:layoutId`}
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
                path={RouteConfig.translations.create()}
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
                path={RouteConfig.translations.list()}
                element={
                  <ProtectedRoute>
                    <DashboardLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<TranslationsListPage />} />
                <Route path=":language" element={<TranslationsEditPage />} />
              </Route>
              <Route
                path={RouteConfig.notificationTemplates.list()}
                element={
                  <ProtectedRoute>
                    <DashboardLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<Navigate to={RouteConfig.notificationTemplates.channel('email')} replace />} />
                <Route path=":channel" element={<NotificationTemplatesListPage />} />
                <Route path=":channel/:id" element={<NotificationTemplateEditPage />} />
              </Route>
              <Route
                path={RouteConfig.notificationTemplates.create(':channel' as NotificationChannel)}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<NotificationTemplateCreatePage />} />
              </Route>
            </Routes>
          </Suspense>
        </ToastProvider>
      </RoutesProvider>
    </BrowserRouter>
  );
}
