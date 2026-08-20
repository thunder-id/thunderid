// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import App from '../App';

vi.mock('@thunderid/react-router', () => ({
  ProtectedRoute: ({children}: {children: React.ReactNode}) => <div data-testid="protected-route">{children}</div>,
}));

// App is rendered here without the ConfigProvider that wraps it in the running app, so useConfig is
// stubbed. PlaneRouteGuard reads the plane through it on every render. The rest of the module, in
// particular ToastProvider, stays real.
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({config: {plane: 'hybrid'}}),
}));

vi.mock('@thunderid/configure-translations', () => ({
  TranslationCreateProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
  TranslationCreatePage: () => <div data-testid="translation-create-page" />,
  TranslationsEditPage: () => <div data-testid="translations-edit-page" />,
  TranslationsListPage: () => <div data-testid="translations-list-page" />,
}));

vi.mock('../lib/monaco-setup', () => ({}));

vi.mock('../features/home/pages/HomePage', () => ({
  default: () => <div data-testid="home-page" />,
}));

vi.mock('../features/users/pages/UsersListPage', () => ({
  default: () => <div data-testid="users-list-page">Users List Page</div>,
}));

vi.mock('../features/users/pages/UserCreatePage', () => ({
  default: () => <div data-testid="create-user-page">Create User Page</div>,
}));

vi.mock('../features/users/pages/UserEditPage', () => ({
  default: () => <div data-testid="user-edit-page">User Edit Page</div>,
}));

vi.mock('@thunderid/configure-user-types', () => ({
  UserTypeCreateProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
  UserTypesListPage: () => <div data-testid="user-types-list-page">User Types List Page</div>,
  CreateUserTypePage: () => <div data-testid="create-user-type-page">Create User Type Page</div>,
  ViewUserTypePage: () => <div data-testid="view-user-type-page">View User Type Page</div>,
}));

vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  ConnectionsListPage: () => <div data-testid="connections-list-page">Connections List Page</div>,
  ConnectionDetailPage: () => <div data-testid="connection-detail-page">Connection Detail Page</div>,
  ConnectionConfigureWizardPage: () => (
    <div data-testid="connection-configure-wizard-page">Connection Configure Wizard Page</div>
  ),
  ConnectionCreateWizardPage: () => (
    <div data-testid="connection-create-wizard-page">Connection Create Wizard Page</div>
  ),
  TrustedIssuerDetailPage: () => <div data-testid="trusted-issuer-detail-page">Trusted Issuer Detail Page</div>,
}));

vi.mock('../features/agents/pages/AgentEditPage', () => ({
  default: () => <div data-testid="agent-edit-page">Agent Edit Page</div>,
}));

vi.mock('../features/applications/pages/ApplicationsListPage', () => ({
  default: () => <div data-testid="applications-list-page">Applications List Page</div>,
}));

vi.mock('../features/applications/pages/ApplicationCreatePage', () => ({
  default: () => <div data-testid="application-create-page">Application Create Page</div>,
}));

vi.mock('../features/applications/pages/ApplicationEditPage', () => ({
  default: () => <div data-testid="application-edit-page">Application Edit Page</div>,
}));

vi.mock('@thunderid/configure-design', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-design')>()),
  LayoutBuilderPage: () => <div data-testid="layout-builder-page">Layout Builder Page</div>,
  LayoutBuilderProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
  DesignPage: () => <div data-testid="design-page">Design Page</div>,
  ThemeBuilderPage: () => <div data-testid="theme-builder-page">Theme Builder Page</div>,
  ThemeCreatePage: () => <div data-testid="theme-create-page">Theme Create Page</div>,
  ThemeBuilderProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-groups', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-groups')>()),
  GroupsListPage: () => <div data-testid="groups-list-page">Groups List Page</div>,
  GroupEditPage: () => <div data-testid="group-edit-page">Group Edit Page</div>,
  CreateGroupPage: () => <div data-testid="create-group-page">Create Group Page</div>,
  GroupCreateProvider: ({children}: {children: React.ReactNode}) => children as React.ReactElement,
}));

vi.mock('@thunderid/configure-import-export', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-import-export')>()),
  ExportPage: () => <div data-testid="export-page">Export Page</div>,
  ImportConfigurationSummaryPage: () => (
    <div data-testid="import-configuration-summary-page">Import Configuration Summary Page</div>
  ),
  ImportConfigurationUploadPage: () => (
    <div data-testid="import-configuration-upload-page">Import Configuration Upload Page</div>
  ),
  ImportConfigurationValidatePage: () => (
    <div data-testid="import-configuration-validate-page">Import Configuration Validate Page</div>
  ),
  ImportExportPage: () => <div data-testid="import-export-page">Import Export Page</div>,
}));

vi.mock('@thunderid/configure-resource-servers', () => ({
  ResourceServersListPage: () => <div data-testid="resource-servers-list-page">Resource Servers List Page</div>,
  ResourceServerEditPage: () => <div data-testid="resource-server-edit-page">Resource Server Edit Page</div>,
  CreateResourceServerPage: () => <div data-testid="create-resource-server-page">Create Resource Server Page</div>,
}));

vi.mock('../layouts/DashboardLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('../layouts/FullScreenLayout', async () => {
  const {Outlet} = await import('react-router');
  return {default: () => <Outlet />};
});

vi.mock('../features/welcome/components/WelcomeRedirect', () => ({
  default: () => null,
}));

describe('App', () => {
  afterEach(() => {
    window.history.pushState({}, '', '/');
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
});
