// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import App from '../App';

vi.mock('@thunderid/react-router', () => ({
  ProtectedRoute: ({children}: {children: React.ReactNode}) => <div data-testid="protected-route">{children}</div>,
}));

vi.mock('@thunderid/configure-translations/contexts/TranslationCreate/TranslationCreateProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-translations/pages/TranslationCreatePage', () => ({
  default: () => <div data-testid="translation-create-page" />,
}));

vi.mock('@thunderid/configure-translations/pages/TranslationsEditPage', () => ({
  default: () => <div data-testid="translations-edit-page" />,
}));

vi.mock('@thunderid/configure-translations/pages/TranslationsListPage', () => ({
  default: () => <div data-testid="translations-list-page" />,
}));

vi.mock('../lib/monaco-setup', () => ({}));

let mockCan: (resource: string, action?: string) => boolean = () => true;
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useCapabilitiesContext: () => ({
    can: (r: string, a?: string) => mockCan(r, a),
    isLoading: false,
  }),
}));

vi.mock('@thunderid/configure-onboarding/pages/HomePage', () => ({default: () => <div data-testid="home-page" />}));

vi.mock('@thunderid/configure-agent-types/contexts/AgentTypeCreate/AgentTypeCreateProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-agent-types/pages/AgentTypesListPage', () => ({
  default: () => <div data-testid="agent-types-list-page">Agent Types List Page</div>,
}));

vi.mock('@thunderid/configure-agent-types/pages/CreateAgentTypePage', () => ({
  default: () => <div data-testid="create-agent-type-page">Create Agent Type Page</div>,
}));

vi.mock('@thunderid/configure-agent-types/pages/ViewAgentTypePage', () => ({
  default: () => <div data-testid="view-agent-type-page">View Agent Type Page</div>,
}));

vi.mock('@thunderid/configure-user-types/contexts/UserTypeCreate/UserTypeCreateProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-user-types/pages/UserTypesListPage', () => ({
  default: () => <div data-testid="user-types-list-page">User Types List Page</div>,
}));

vi.mock('@thunderid/configure-user-types/pages/CreateUserTypePage', () => ({
  default: () => <div data-testid="create-user-type-page">Create User Type Page</div>,
}));

vi.mock('@thunderid/configure-user-types/pages/ViewUserTypePage', () => ({
  default: () => <div data-testid="view-user-type-page">View User Type Page</div>,
}));

vi.mock('@thunderid/configure-connections/pages/ConnectionsListPage', () => ({
  default: () => <div data-testid="connections-list-page">Connections List Page</div>,
}));

vi.mock('@thunderid/configure-connections/pages/ConnectionDetailPage', () => ({
  default: () => <div data-testid="connection-detail-page">Connection Detail Page</div>,
}));

vi.mock('@thunderid/configure-connections/pages/ConnectionConfigureWizardPage', () => ({
  default: () => <div data-testid="connection-configure-wizard-page">Connection Configure Wizard Page</div>,
}));

vi.mock('@thunderid/configure-connections/pages/ConnectionCreateWizardPage', () => ({
  default: () => <div data-testid="connection-create-wizard-page">Connection Create Wizard Page</div>,
}));

vi.mock('@thunderid/configure-connections/pages/TrustedIssuerDetailPage', () => ({
  default: () => <div data-testid="trusted-issuer-detail-page">Trusted Issuer Detail Page</div>,
}));

vi.mock('@thunderid/configure-agents/pages/AgentEditPage', () => ({
  default: () => <div data-testid="agent-edit-page">Agent Edit Page</div>,
}));

vi.mock('@thunderid/configure-agents/pages/AgentAddRoute', () => ({
  default: () => <div data-testid="agent-onboard-page">Agent Onboard Page</div>,
}));

vi.mock('@thunderid/configure-agents/pages/AgentsListPage', () => ({
  default: () => <div data-testid="agents-list-page">Agents List Page</div>,
}));

vi.mock('@thunderid/configure-applications/pages/ApplicationsListPage', () => ({
  default: () => <div data-testid="applications-list-page">Applications List Page</div>,
}));

vi.mock('@thunderid/configure-applications/pages/ApplicationCreatePage', () => ({
  default: () => <div data-testid="application-create-page">Application Create Page</div>,
}));

vi.mock('@thunderid/configure-applications/pages/ApplicationEditPage', () => ({
  default: () => <div data-testid="application-edit-page">Application Edit Page</div>,
}));

vi.mock('@thunderid/configure-applications/pages/ApplicationTemplateSelectPage', () => ({
  default: () => <div data-testid="application-template-select-page">Template Select Page</div>,
}));

vi.mock('@thunderid/configure-applications/contexts/ApplicationCreate/ApplicationCreateProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-design/pages/LayoutBuilderPage', () => ({
  default: () => <div data-testid="layout-builder-page">Layout Builder Page</div>,
}));

vi.mock('@thunderid/configure-design/contexts/LayoutBuilder/LayoutBuilderProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-design/pages/DesignPage', () => ({
  default: () => <div data-testid="design-page">Design Page</div>,
}));

vi.mock('@thunderid/configure-design/pages/ThemeBuilderPage', () => ({
  default: () => <div data-testid="theme-builder-page">Theme Builder Page</div>,
}));

vi.mock('@thunderid/configure-design/pages/ThemeCreatePage', () => ({
  default: () => <div data-testid="theme-create-page">Theme Create Page</div>,
}));

vi.mock('@thunderid/configure-design/contexts/ThemeBuilder/ThemeBuilderProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-groups/pages/GroupsListPage', () => ({
  default: () => <div data-testid="groups-list-page">Groups List Page</div>,
}));

vi.mock('@thunderid/configure-groups/pages/GroupEditPage', () => ({
  default: () => <div data-testid="group-edit-page">Group Edit Page</div>,
}));

vi.mock('@thunderid/configure-groups/pages/CreateGroupPage', () => ({
  default: () => <div data-testid="create-group-page">Create Group Page</div>,
}));

vi.mock('@thunderid/configure-groups/contexts/GroupCreate/GroupCreateProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-import-export/pages/ExportPage', () => ({
  default: () => <div data-testid="export-page">Export Page</div>,
}));

vi.mock('@thunderid/configure-import-export/pages/ImportConfigurationSummaryPage', () => ({
  default: () => <div data-testid="import-configuration-summary-page">Import Configuration Summary Page</div>,
}));

vi.mock('@thunderid/configure-import-export/pages/ImportConfigurationUploadPage', () => ({
  default: () => <div data-testid="import-configuration-upload-page">Import Configuration Upload Page</div>,
}));

vi.mock('@thunderid/configure-import-export/pages/ImportConfigurationValidatePage', () => ({
  default: () => <div data-testid="import-configuration-validate-page">Import Configuration Validate Page</div>,
}));

vi.mock('@thunderid/configure-import-export/pages/ImportExportPage', () => ({
  default: () => <div data-testid="import-export-page">Import Export Page</div>,
}));

vi.mock('@thunderid/configure-organization-units/contexts/OrganizationUnitProvider', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('@thunderid/configure-organization-units/pages/CreateOrganizationUnitPage', () => ({
  default: () => <div data-testid="create-organization-unit-page">Create OU Page</div>,
}));

vi.mock('@thunderid/configure-organization-units/pages/OrganizationUnitEditPage', () => ({
  default: ({
    renderDefaultFlowsSettings,
  }: {
    renderDefaultFlowsSettings: (props: Record<string, never>) => React.ReactNode;
  }) => <div data-testid="organization-unit-edit-page">{renderDefaultFlowsSettings({})}</div>,
}));

vi.mock('@thunderid/configure-organization-units/pages/OrganizationUnitsListPage', () => ({
  default: () => <div data-testid="organization-units-list-page">Organization Units List</div>,
}));

vi.mock('@thunderid/configure-flows/components/OrganizationUnitDefaultFlowsSettings', () => ({
  default: () => <div data-testid="ou-default-flows-settings" />,
}));

vi.mock('@thunderid/configure-flows/pages/FlowCreatePage', () => ({
  default: () => <div data-testid="flow-create-page">Flow Create Page</div>,
}));

vi.mock('@thunderid/configure-flows/pages/FlowsListPage', () => ({
  default: () => <div data-testid="flows-list-page">Flows List Page</div>,
}));

vi.mock('@thunderid/configure-flows/pages/FlowBuilderPage', () => ({
  default: () => <div data-testid="flow-builder-page">Flow Builder Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers/pages/ResourceServersListPage', () => ({
  default: () => <div data-testid="resource-servers-list-page">Resource Servers List Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers/pages/ResourceServerEditPage', () => ({
  default: () => <div data-testid="resource-server-edit-page">Resource Server Edit Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers/pages/CreateResourceServerPage', () => ({
  default: () => <div data-testid="create-resource-server-page">Create Resource Server Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers/pages/AuthorizationDetailTypesListPage', () => ({
  default: () => <div data-testid="authorization-detail-types-list-page">Authorization Detail Types List Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers/pages/AuthorizationDetailTypeEditPage', () => ({
  default: () => <div data-testid="authorization-detail-type-edit-page">Authorization Detail Type Edit Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers/pages/CreateAuthorizationDetailTypePage', () => ({
  default: () => <div data-testid="create-authorization-detail-type-page">Create Authorization Detail Type Page</div>,
}));

vi.mock('../layouts/DashboardLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('../layouts/FullScreenLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

describe('App', () => {
  afterEach(() => {
    window.history.pushState({}, '', '/');
    mockCan = () => true;
  });

  it('shows a notice instead of import / export to admins who may do neither', async () => {
    mockCan = (resource) => resource !== 'import' && resource !== 'export';
    window.history.pushState({}, '', '/import-export');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByText('You do not have permission to view this page.')).toBeInTheDocument();
    });
  });

  it('opens export to an admin who may export but not import', async () => {
    mockCan = (resource) => resource !== 'import';
    window.history.pushState({}, '', '/export');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('export-page')).toBeInTheDocument();
    });
  });

  it('renders without crashing', () => {
    const {container} = render(<App />);
    expect(container).toBeInTheDocument();
  });

  it('loads TranslationsEditPage lazily via the monaco-setup chain', async () => {
    window.history.pushState({}, '', '/translations/en');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('translations-edit-page')).toBeInTheDocument();
    });
  });

  it('loads ApplicationEditPage lazily via the monaco-setup chain', async () => {
    window.history.pushState({}, '', '/applications/app-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('application-edit-page')).toBeInTheDocument();
    });
  });

  it('loads AgentEditPage lazily via the monaco-setup chain', async () => {
    window.history.pushState({}, '', '/agents/agent-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('agent-edit-page')).toBeInTheDocument();
    });
  });

  it('loads LayoutBuilderPage lazily via the monaco-setup chain', async () => {
    window.history.pushState({}, '', '/design/layouts/layout-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('layout-builder-page')).toBeInTheDocument();
    });
  });

  it('loads ExportPage lazily via the monaco-setup chain', async () => {
    window.history.pushState({}, '', '/export');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('export-page')).toBeInTheDocument();
    });
  });

  it('loads ImportConfigurationSummaryPage lazily via the monaco-setup chain', async () => {
    window.history.pushState({}, '', '/import-configuration/summary');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('import-configuration-summary-page')).toBeInTheDocument();
    });
  });

  it('loads ResourceServersListPage lazily at /resource-servers', async () => {
    window.history.pushState({}, '', '/resource-servers');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('resource-servers-list-page')).toBeInTheDocument();
    });
  });

  it('loads ResourceServerEditPage lazily at /resource-servers/:id', async () => {
    window.history.pushState({}, '', '/resource-servers/rs-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('resource-server-edit-page')).toBeInTheDocument();
    });
  });

  it('loads CreateResourceServerPage lazily at /resource-servers/create', async () => {
    window.history.pushState({}, '', '/resource-servers/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-resource-server-page')).toBeInTheDocument();
    });
  });

  it('loads AuthorizationDetailTypesListPage lazily at /authorization-detail-types', async () => {
    window.history.pushState({}, '', '/authorization-detail-types');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('authorization-detail-types-list-page')).toBeInTheDocument();
    });
  });

  it('loads AuthorizationDetailTypeEditPage lazily at /authorization-detail-types/:id', async () => {
    window.history.pushState({}, '', '/authorization-detail-types/type-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('authorization-detail-type-edit-page')).toBeInTheDocument();
    });
  });

  it('loads CreateAuthorizationDetailTypePage lazily at /authorization-detail-types/create', async () => {
    window.history.pushState({}, '', '/authorization-detail-types/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-authorization-detail-type-page')).toBeInTheDocument();
    });
  });

  it('loads UserTypesListPage lazily at /user-types', async () => {
    window.history.pushState({}, '', '/user-types');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('user-types-list-page')).toBeInTheDocument();
    });
  });

  it('loads ViewUserTypePage lazily at /user-types/:id', async () => {
    window.history.pushState({}, '', '/user-types/ut-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('view-user-type-page')).toBeInTheDocument();
    });
  });

  it('loads CreateUserTypePage lazily at /user-types/create', async () => {
    window.history.pushState({}, '', '/user-types/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-user-type-page')).toBeInTheDocument();
    });
  });

  it('should load AgentTypesListPage lazily at /agent-types', async () => {
    window.history.pushState({}, '', '/agent-types');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('agent-types-list-page')).toBeInTheDocument();
    });
  });

  it('should load ViewAgentTypePage lazily at /agent-types/:id', async () => {
    window.history.pushState({}, '', '/agent-types/at-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('view-agent-type-page')).toBeInTheDocument();
    });
  });

  it('should load CreateAgentTypePage lazily at /agent-types/create', async () => {
    window.history.pushState({}, '', '/agent-types/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-agent-type-page')).toBeInTheDocument();
    });
  });

  it('loads ConnectionsListPage lazily at /connections', async () => {
    window.history.pushState({}, '', '/connections');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('connections-list-page')).toBeInTheDocument();
    });
  });

  it('loads ConnectionDetailPage lazily at /connections/:type', async () => {
    window.history.pushState({}, '', '/connections/google');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('connection-detail-page')).toBeInTheDocument();
    });
  });

  it('loads ConnectionCreateWizardPage lazily at /connections/create', async () => {
    window.history.pushState({}, '', '/connections/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('connection-create-wizard-page')).toBeInTheDocument();
    });
  });

  it('loads ConnectionConfigureWizardPage lazily at /connections/:type/configure', async () => {
    window.history.pushState({}, '', '/connections/google/configure');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('connection-configure-wizard-page')).toBeInTheDocument();
    });
  });

  it('loads TrustedIssuerDetailPage lazily at /trusted-issuers/:id', async () => {
    window.history.pushState({}, '', '/trusted-issuers/ti-1');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('trusted-issuer-detail-page')).toBeInTheDocument();
    });
  });

  it('loads OrganizationUnitEditPage and its default flows settings at /organization-units/:id', async () => {
    window.history.pushState({}, '', '/organization-units/ou-1');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('organization-unit-edit-page')).toBeInTheDocument();
    });
    expect(screen.getByTestId('ou-default-flows-settings')).toBeInTheDocument();
  });

  it('loads FlowsListPage lazily at /flows', async () => {
    window.history.pushState({}, '', '/flows');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('flows-list-page')).toBeInTheDocument();
    });
  });

  it('loads FlowCreatePage lazily at /flows/create', async () => {
    window.history.pushState({}, '', '/flows/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('flow-create-page')).toBeInTheDocument();
    });
  });

  it('loads FlowBuilderPage lazily at /flows/:flowId', async () => {
    window.history.pushState({}, '', '/flows/flow-1');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('flow-builder-page')).toBeInTheDocument();
    });
  });

  it('loads DesignPage lazily at /design', async () => {
    window.history.pushState({}, '', '/design');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('design-page')).toBeInTheDocument();
    });
  });

  it('loads ThemeCreatePage lazily at /design/themes/create', async () => {
    window.history.pushState({}, '', '/design/themes/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('theme-create-page')).toBeInTheDocument();
    });
  });

  it('loads ThemeBuilderPage lazily at /design/themes/:themeId', async () => {
    window.history.pushState({}, '', '/design/themes/theme-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('theme-builder-page')).toBeInTheDocument();
    });
  });

  it('loads GroupsListPage lazily at /groups', async () => {
    window.history.pushState({}, '', '/groups');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('groups-list-page')).toBeInTheDocument();
    });
  });

  it('loads GroupEditPage lazily at /groups/:groupId', async () => {
    window.history.pushState({}, '', '/groups/group-123');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('group-edit-page')).toBeInTheDocument();
    });
  });

  it('loads CreateGroupPage lazily at /groups/create', async () => {
    window.history.pushState({}, '', '/groups/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('create-group-page')).toBeInTheDocument();
    });
  });

  it('loads ImportExportPage lazily at /import-export', async () => {
    window.history.pushState({}, '', '/import-export');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('import-export-page')).toBeInTheDocument();
    });
  });

  it('loads ImportConfigurationUploadPage lazily at /import-configuration', async () => {
    window.history.pushState({}, '', '/import-configuration');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('import-configuration-upload-page')).toBeInTheDocument();
    });
  });

  it('loads ImportConfigurationValidatePage lazily at /import-configuration/validate', async () => {
    window.history.pushState({}, '', '/import-configuration/validate');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('import-configuration-validate-page')).toBeInTheDocument();
    });
  });

  it('loads ApplicationsListPage lazily at /applications', async () => {
    window.history.pushState({}, '', '/applications');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('applications-list-page')).toBeInTheDocument();
    });
  });

  it('loads ApplicationTemplateSelectPage lazily at /applications/types', async () => {
    window.history.pushState({}, '', '/applications/types');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('application-template-select-page')).toBeInTheDocument();
    });
  });

  it('loads ApplicationCreatePage lazily at /applications/create', async () => {
    window.history.pushState({}, '', '/applications/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('application-create-page')).toBeInTheDocument();
    });
  });

  it('loads AgentsListPage lazily at /agents', async () => {
    window.history.pushState({}, '', '/agents');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('agents-list-page')).toBeInTheDocument();
    });
  });

  it('loads AgentOnboardPage lazily at /agents/create', async () => {
    window.history.pushState({}, '', '/agents/create');
    render(<App />);
    await waitFor(() => {
      expect(screen.getByTestId('agent-onboard-page')).toBeInTheDocument();
    });
  });
});
