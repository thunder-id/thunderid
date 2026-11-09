// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {afterEach, describe, it, expect, vi, beforeEach} from 'vitest';
import DashboardLayout from '../DashboardLayout';

const mockNavigate = vi.fn();
const mockSignIn = vi.fn();
const mockSignOut = vi.fn();
const mockClearSession = vi.fn();
const mockLoggerError = vi.fn();
const mockLoggerWarn = vi.fn();
const mockUserData = vi.fn();
interface MockUseGetApplicationsResult {
  data?: {
    applications?: {
      clientId?: string;
      name?: string;
      template?: string;
    }[];
  };
  isLoading: boolean;
}

const mockUseGetApplications = vi.fn<(params: unknown) => MockUseGetApplicationsResult>();
let mockDiscovery: {wellKnown?: {end_session_endpoint?: string}} | undefined;
let mockIsTrustedIssuerGenericOidc = false;
let mockSdkConfig: Record<string, unknown> | undefined;
let mockCapabilities: {can: (resource: string, action?: string) => boolean; canManageRoot: boolean; isLoading: boolean};
let mockPathname: string | undefined;

vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useCapabilities: () => mockCapabilities,
}));
interface MockProjectSelection {
  projects: {id: string; displayName: string}[];
  selectedProject?: {id: string; displayName: string};
  isCleared: boolean;
  isLoading: boolean;
  error: Error | null;
  clearSelection: () => void;
}

const mockClearSelection = vi.fn();
const mockUseProjectSelection = vi.fn<() => MockProjectSelection>();
const shop = {id: 'p-shop', displayName: 'Shop'};

function projectSelection(overrides: Partial<MockProjectSelection> = {}): MockProjectSelection {
  return {
    projects: [],
    isCleared: false,
    isLoading: false,
    error: null,
    clearSelection: mockClearSelection,
    ...overrides,
  };
}

/** The selection while the Shop project is open. */
const inShop = (): MockProjectSelection => projectSelection({projects: [shop], selectedProject: shop});

vi.mock('@thunderid/configure-projects', () => ({
  ProjectSwitcher: () => <div data-testid="project-switcher" />,
  useProjectSelection: () => mockUseProjectSelection(),
}));

let mockProjectChanges: Record<string, number> = {};

let mockPath = '/';
vi.mock('@thunderid/configure-gateways', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/configure-gateways')>();
  return {
    gatewayColor: actual.gatewayColor,
    gatewaySectionOf: actual.gatewaySectionOf,
    useDeployRoutes: () => actual.defaultDeployRoutePaths,
    usePipeline: () => ({
      data: {stages: [{name: 'Development', gateway: {id: 'gw-dev', name: 'dev', baseUrl: 'https://dev.test'}}]},
    }),
    useProjectChanges: () => mockProjectChanges,
    useProjectScope: actual.useProjectScope,
    useProjectStatuses: () => ({
      byGateway: {
        'gw-dev': {
          project: {versions: [{hash: 'h37', number: 37, resources: 2}], pending: 4, required: 1},
          shared: {pending: 0, byType: []},
          needed: {differing: 0},
        },
      },
      isLoading: false,
    }),
    useVersions: () => ({data: [{version: 'h42', number: 42}]}),
  };
});

vi.mock('@thunderid/configure-applications', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-applications')>()),
  useGetApplications: (params: unknown) => mockUseGetApplications(params),
}));

// Mock ThunderID
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    signIn: mockSignIn,
    clearSession: mockClearSession,
    discovery: mockDiscovery,
  }),
  User: ({children}: {children: (user: unknown) => React.ReactNode}) => children(mockUserData()),
  SignOutButton: ({children}: {children: (props: {signOut: () => void}) => React.ReactNode}) =>
    children({signOut: mockSignOut}),
}));

// Mock contexts
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: 'ThunderID',
          favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
        },
        client: {client_id: 'CONSOLE'},
        sdk: mockSdkConfig,
      },
      isTrustedIssuerGenericOidc: () => mockIsTrustedIssuerGenericOidc,
      getTrustedIssuerClientId: () => 'test-client-id',
      getClientUrl: () => 'https://localhost:5191/console',
      getServerUrl: () => 'https://localhost:8090',
    }),
  };
});

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: mockLoggerError,
    warn: mockLoggerWarn,
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

// Mock Outlet
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  const {useCapabilitiesContext} = await vi.importActual<typeof import('@thunderid/contexts')>('@thunderid/contexts');
  return {
    ...actual,
    Outlet: () => (
      <div data-testid="outlet" data-can-delete-app={String(useCapabilitiesContext().can('app', 'delete'))}>
        Outlet Content
      </div>
    ),
    Link: ({children, to, onClick = undefined}: {children: React.ReactNode; to: string; onClick?: () => void}) => (
      <a href={to} data-testid="router-link" onClick={onClick}>
        {children}
      </a>
    ),
    useNavigate: () => mockNavigate,
    useLocation: () => (mockPathname ? {...actual.useLocation(), pathname: mockPathname} : actual.useLocation()),
    useMatch: (pattern: string) => actual.matchPath(pattern, mockPath),
  };
});

describe('DashboardLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    mockUserData.mockReturnValue({name: 'Test User', email: 'test@example.com'});
    mockIsTrustedIssuerGenericOidc = false;
    mockDiscovery = undefined;
    mockUseProjectSelection.mockReturnValue(projectSelection());
    mockProjectChanges = {};
    mockPath = '/';
    mockCapabilities = {can: () => true, canManageRoot: true, isLoading: false};
    mockPathname = undefined;
    mockUseGetApplications.mockReturnValue({
      data: {applications: []},
      isLoading: false,
    });
  });

  it('renders AppShell layout', () => {
    const {rerender} = render(<DashboardLayout />);
    rerender(<DashboardLayout />);

    // Check that the outlet is rendered
    expect(screen.getByTestId('outlet')).toBeInTheDocument();
  });

  it('renders Outlet for nested routes', () => {
    render(<DashboardLayout />);

    expect(screen.getByTestId('outlet')).toBeInTheDocument();
    expect(screen.getByTestId('outlet')).toHaveTextContent('Outlet Content');
  });

  it('offers no project selector while there are no projects', () => {
    render(<DashboardLayout />);

    expect(screen.queryByTestId('project-switcher')).not.toBeInTheDocument();
  });

  it('shows the project selector in the header', () => {
    mockUseProjectSelection.mockReturnValue(inShop());

    render(<DashboardLayout />);

    expect(screen.getByTestId('project-switcher')).toBeInTheDocument();
  });

  it("lists the selected project's applications, agents, resource servers, flows, design and notifications under it, with what is not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {application: 2};
    render(<DashboardLayout />);

    expect(screen.getByText('navigation:categories.project')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.deploymentPipeline')).toBeInTheDocument();
    expect(screen.getByText('navigation:categories.build')).toBeInTheDocument();
    expect(screen.getAllByText('navigation:pages.applications')).toHaveLength(1);
    expect(screen.getAllByText('navigation:pages.agents')).toHaveLength(1);
    expect(screen.getAllByText('navigation:pages.resourceServers')).toHaveLength(1);
    expect(screen.getAllByText('navigation:pages.flows')).toHaveLength(1);
    expect(screen.getAllByText('navigation:pages.design')).toHaveLength(1);
    expect(screen.getAllByText('navigation:pages.notifications')).toHaveLength(1);
    // Nothing of the organization's own is listed inside a project.
    expect(screen.queryByText('navigation:pages.home')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.administrators')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.userTypes')).not.toBeInTheDocument();
    expect(screen.getByText('navigation:badges.undeployed')).toBeInTheDocument();
  });

  it("lists the project's users, groups, organization units, design and verifiable credentials", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    render(<DashboardLayout />);

    expect(screen.getByText('navigation:categories.identities')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.users').closest('a')).toHaveAttribute('href', '/users');
    expect(screen.getByText('navigation:pages.groups').closest('a')).toHaveAttribute('href', '/groups');
    expect(screen.getByText('navigation:pages.organizationUnits').closest('a')).toHaveAttribute(
      'href',
      '/organization-units',
    );
    expect(screen.getByText('navigation:categories.customize')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.design').closest('a')).toHaveAttribute('href', '/design');
    expect(screen.getByText('navigation:pages.verifiableCredentials')).toBeInTheDocument();
  });

  it("badges the selected project's resource servers that are not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {resource_server: 1};
    render(<DashboardLayout />);

    const item = screen.getByText('navigation:pages.resourceServers').closest('.MuiListItemButton-root')!;
    expect(item).toHaveTextContent('navigation:badges.undeployed');
  });

  it("lists the selected project's roles under it, with what is not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {role: 1};
    render(<DashboardLayout />);

    expect(screen.getAllByText('navigation:pages.roles')).toHaveLength(1);
    const item = screen.getByText('navigation:pages.roles').closest('.MuiListItemButton-root')!;
    expect(item).toHaveTextContent('navigation:badges.undeployed');
  });

  it("lists the selected project's themes under Design, with what is not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {theme: 2};
    render(<DashboardLayout />);

    expect(screen.getAllByText('navigation:pages.design')).toHaveLength(1);
    const item = screen.getByText('navigation:pages.design').closest('.MuiListItemButton-root')!;
    expect(item).toHaveTextContent('navigation:badges.undeployed');
  });

  it('lists every role in the organization, outside any project', () => {
    mockUseProjectSelection.mockReturnValue(projectSelection({projects: [shop], isCleared: true}));
    mockPath = '/home';
    render(<DashboardLayout />);

    expect(screen.getAllByText('navigation:pages.roles')).toHaveLength(1);
  });

  it("badges the selected project's flows that are not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {flow: 2};
    render(<DashboardLayout />);

    const item = screen.getByText('navigation:pages.flows').closest('.MuiListItemButton-root')!;
    expect(item).toHaveTextContent('navigation:badges.undeployed');
  });

  it("badges the selected project's design when its layouts are not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {layout: 1};
    render(<DashboardLayout />);

    const item = screen.getByText('navigation:pages.design').closest('.MuiListItemButton-root')!;
    expect(item).toHaveTextContent('navigation:badges.undeployed');
  });

  it("counts the selected project's organization units that are not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {organization_unit: 2};
    render(<DashboardLayout />);

    const item = screen.getByText('navigation:pages.organizationUnits').closest('.MuiListItemButton-root')!;
    expect(item.querySelector('.MuiChip-root')).toHaveTextContent(/^2$/);
    expect(item).not.toHaveTextContent('navigation:badges.undeployed');
  });

  it("counts the selected project's undeployed verifiable credentials on the collapsed entry", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {credential_configuration: 1, presentation_definition: 2};
    render(<DashboardLayout />);

    const item = screen.getByText('navigation:pages.verifiableCredentials').closest('.MuiListItemButton-root')!;
    expect(item).toHaveTextContent('3');
    expect(item).not.toHaveTextContent('navigation:badges.undeployed');
  });

  it('badges the undeployed credential templates and presentation definitions once the entry is expanded', async () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {credential_configuration: 1, presentation_definition: 2};
    render(<DashboardLayout />);

    const parent = screen.getByText('navigation:pages.verifiableCredentials').closest('.MuiListItemButton-root')!;
    await userEvent.click(parent);

    await waitFor(() => {
      const templates = screen.getByText('navigation:pages.credentials').closest('.MuiListItemButton-root')!;
      expect(templates).toHaveTextContent('navigation:badges.undeployed');
    });
    const presentations = screen.getByText('navigation:pages.presentations').closest('.MuiListItemButton-root')!;
    expect(presentations).toHaveTextContent('navigation:badges.undeployed');
    expect(parent).not.toHaveTextContent('3');
  });

  it("badges the selected project's notifications when its templates are not deployed yet", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {notification_template: 1};
    render(<DashboardLayout />);

    const item = screen.getByText('navigation:pages.notifications').closest('.MuiListItemButton-root')!;
    expect(item).toHaveTextContent('navigation:badges.undeployed');
  });

  it('keeps the whole label beside an undeployed badge, the badge after it', () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockProjectChanges = {application: 1};
    render(<DashboardLayout />);

    const label = screen.getByText('navigation:pages.applications').closest('.MuiListItemText-root')!;
    const badge = screen.getByText('navigation:badges.undeployed').closest('.MuiChip-root')!;
    expect(getComputedStyle(label).flexShrink).toBe('0');
    expect(getComputedStyle(label).overflow).toBe('visible');
    expect(getComputedStyle(badge).flexShrink).toBe('0');
    expect(label.compareDocumentPosition(badge) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("shows the gateway's own navigation on its pages, back to the project", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockPath = '/deploy/gateways/gw-dev/values';
    render(<DashboardLayout />);

    // The way back names the project.
    expect(screen.getByText('Shop')).toBeInTheDocument();
    expect(screen.getByText('navigation:categories.gateway')).toBeInTheDocument();
    for (const page of [
      'navigation:pages.overview',
      'navigation:pages.variablesAndSecrets',
      'navigation:pages.endpoints',
      'navigation:pages.deployedConfiguration',
      'navigation:pages.deploymentHistory',
    ]) {
      expect(screen.getByText(page)).toBeInTheDocument();
    }
    expect(screen.queryByText('navigation:pages.applications')).not.toBeInTheDocument();
  });

  it("shows a gateway's own navigation without a project, back to the organization's overview", () => {
    mockPath = '/deploy/gateways/gw-dev';
    render(<DashboardLayout />);

    expect(screen.getByText('navigation:categories.gateway')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.deploymentHistory')).toBeInTheDocument();
    expect(screen.getAllByText('navigation:pages.overview')).toHaveLength(2);
    expect(screen.queryByText('navigation:pages.applications')).not.toBeInTheDocument();
  });

  it("offers switching gateways from the header on a gateway's pages only", () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    const {unmount} = render(<DashboardLayout />);

    expect(screen.queryByTestId('gateway-switcher')).not.toBeInTheDocument();
    expect(screen.queryByTestId('header-read-only')).not.toBeInTheDocument();
    unmount();

    mockPath = '/deploy/gateways/gw-dev';
    render(<DashboardLayout />);
    expect(screen.getByTestId('gateway-switcher')).toBeInTheDocument();
  });

  it('leaves Projects out of the sidebar, and keeps settings in its own group at the end', () => {
    mockUseProjectSelection.mockReturnValue(projectSelection({projects: [shop], isCleared: true}));
    mockPath = '/home';

    render(<DashboardLayout />);

    expect(screen.queryByText('navigation:pages.projects')).not.toBeInTheDocument();
    expect(screen.queryByTestId('gateway-switcher')).not.toBeInTheDocument();
    const system = screen.getByText('navigation:categories.system');
    const settings = screen.getByText('navigation:pages.settings');
    expect(system.compareDocumentPosition(settings) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('names the gateway open, and says its configuration is read-only there', () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    mockPath = '/deploy/gateways/gw-dev';
    render(<DashboardLayout />);

    expect(screen.getByTestId('gateway-switcher')).toHaveTextContent('DEVELOPMENT');
    expect(screen.getByTestId('header-read-only')).toHaveTextContent('common:header.readOnly');
  });

  it('leads out of a project to the organization from the top of its navigation', async () => {
    mockUseProjectSelection.mockReturnValue(inShop());
    render(<DashboardLayout />);

    const organization = screen.getByText('navigation:pages.organizationSettings');
    expect(organization.closest('a')).toHaveAttribute('href', '/home');
    await userEvent.click(organization);

    expect(mockClearSelection).toHaveBeenCalled();
  });

  it('renders navigation categories', () => {
    render(<DashboardLayout />);

    // The organization's navigation, grouped as its overview lays it out.
    for (const category of [
      'navigation:categories.administration',
      'navigation:categories.identitySetup',
      'navigation:categories.signInExperience',
      'navigation:categories.apisAndCredentials',
    ]) {
      expect(screen.getByText(category)).toBeInTheDocument();
    }
    expect(screen.getByText('navigation:pages.overview')).toBeInTheDocument();
  });

  it('offers no Deploy in the header or the navigation', () => {
    render(<DashboardLayout />);

    expect(screen.queryByRole('link', {name: /deploy/i})).not.toBeInTheDocument();
    expect(screen.queryByTestId('header-deploy')).not.toBeInTheDocument();
  });

  it('renders navigation items', () => {
    render(<DashboardLayout />);

    // Check for navigation items using translation keys
    expect(screen.getByText('navigation:pages.administrators')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.userTypes')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.connections')).toBeInTheDocument();
    // Applications, agents, flows, themes and notification templates belong to projects, so the organization
    // lists none of them.
    expect(screen.queryByText('navigation:pages.applications')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.agents')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.flows')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.design')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.notifications')).not.toBeInTheDocument();
    expect(screen.getByText('navigation:pages.resourceServers')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.gateways')).toBeInTheDocument();
    // Users and groups are listed in projects as well as the organization.
    expect(screen.getByText('navigation:pages.groups')).toBeInTheDocument();
    // Organization units, design and verifiable credentials belong to projects.
    expect(screen.queryByText('navigation:pages.organizationUnits')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.design')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.verifiableCredentials')).not.toBeInTheDocument();
  });

  it('should list Agent types under Identity setup directly after User types', () => {
    render(<DashboardLayout />);

    const userTypes = screen.getByRole('button', {name: 'navigation:pages.userTypes'});
    const agentTypes = screen.getByRole('button', {name: 'navigation:pages.agentTypes'});
    const items = screen.getAllByRole('button').filter((item) => item.textContent?.startsWith('navigation:pages.'));

    expect(items.indexOf(agentTypes)).toBe(items.indexOf(userTypes) + 1);
  });

  // Agent types pair with Agents the way User types pair with Users: the bot with a cog, beside the plain bot.
  it('should mark Agent types with the agent type icon', () => {
    render(<DashboardLayout />);

    expect(
      screen.getByRole('button', {name: 'navigation:pages.agentTypes'}).querySelector('svg.lucide-bot-cog'),
    ).toBeInTheDocument();
  });

  it.each([false, true])('keeps navigation icon sizes consistent when collapseSidebar is %s', (collapseSidebar) => {
    render(<DashboardLayout collapseSidebar={collapseSidebar} />);

    const applicationIcon = screen.getByRole('button', {name: 'navigation:pages.connections'}).querySelector('svg');
    expect(applicationIcon).toBeInTheDocument();

    const navigationIcons = screen.getByRole('navigation').querySelectorAll('.MuiListItemIcon-root svg');
    expect(navigationIcons.length).toBeGreaterThan(0);
    navigationIcons.forEach((icon) => {
      expect(icon).toHaveAttribute('width', applicationIcon?.getAttribute('width'));
      expect(icon).toHaveAttribute('height', applicationIcon?.getAttribute('height'));
    });
  });

  describe('capabilities', () => {
    it('hands the capabilities to the page', () => {
      mockCapabilities = {
        can: (_resource, action = 'view') => action === 'view',
        canManageRoot: false,
        isLoading: false,
      };
      render(<DashboardLayout />);

      expect(screen.getByTestId('outlet')).toHaveAttribute('data-can-delete-app', 'false');
    });

    it('hides the pages the admin cannot view, and groups left empty', () => {
      mockCapabilities = {can: (resource) => resource === 'user', canManageRoot: false, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.administrators')).toBeInTheDocument();
      expect(screen.getByText('navigation:pages.overview')).toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.gateways')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:categories.system')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:categories.identitySetup')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.notifications')).not.toBeInTheDocument();
    });

    it('hides Agent types from admins who cannot view agent types', () => {
      mockCapabilities = {can: (resource) => resource !== 'agenttype', canManageRoot: false, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByRole('button', {name: 'navigation:pages.userTypes'})).toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'navigation:pages.agentTypes'})).not.toBeInTheDocument();
    });

    it('keeps the navigation while the capabilities load', () => {
      mockCapabilities = {can: () => false, canManageRoot: false, isLoading: true};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.administrators')).toBeInTheDocument();
      expect(screen.getByText('navigation:pages.gateways')).toBeInTheDocument();
    });

    it('keeps a parent entry while one of its children is visible', () => {
      mockUseProjectSelection.mockReturnValue(inShop());
      mockCapabilities = {can: (resource) => resource === 'vc', canManageRoot: false, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.verifiableCredentials')).toBeInTheDocument();
    });

    it("hides the project's pages the admin cannot view", () => {
      mockUseProjectSelection.mockReturnValue(inShop());
      mockCapabilities = {can: (resource) => resource === 'app', canManageRoot: false, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.applications')).toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.agents')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.deploymentPipeline')).not.toBeInTheDocument();
      expect(screen.getByText('navigation:pages.organizationSettings')).toBeInTheDocument();
    });

    it("offers a gateway's variables and secrets only to admins who can manage root-only APIs", () => {
      mockUseProjectSelection.mockReturnValue(inShop());
      mockPath = '/deploy/gateways/gw-dev';
      mockCapabilities = {can: () => true, canManageRoot: false, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.overview')).toBeInTheDocument();
      expect(screen.getByText('navigation:pages.deploymentHistory')).toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.variablesAndSecrets')).not.toBeInTheDocument();
    });

    it("hides a gateway's releases from admins who cannot view configuration versions", () => {
      mockUseProjectSelection.mockReturnValue(inShop());
      mockPath = '/deploy/gateways/gw-dev';
      mockCapabilities = {can: (resource) => resource !== 'configversion', canManageRoot: true, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.queryByText('navigation:categories.releases')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.deploymentHistory')).not.toBeInTheDocument();
    });

    it('offers Import / Export only to admins who can manage root-only APIs', () => {
      mockCapabilities = {can: () => true, canManageRoot: false, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.queryByRole('button', {name: 'navigation:pages.importExport'})).not.toBeInTheDocument();
    });

    it('shows a notice instead of a page the admin cannot view', () => {
      mockCapabilities = {can: (resource) => resource !== 'gateway', canManageRoot: false, isLoading: false};
      mockPathname = '/gateways';
      render(<DashboardLayout />);

      expect(screen.queryByTestId('outlet')).not.toBeInTheDocument();
      expect(screen.getByText('navigation:forbidden.description')).toBeInTheDocument();
    });

    it('shows a notice instead of a root-only page the admin cannot manage', () => {
      mockUseProjectSelection.mockReturnValue(inShop());
      mockPath = '/deploy/gateways/gw-dev/values';
      mockPathname = mockPath;
      mockCapabilities = {can: () => true, canManageRoot: false, isLoading: false};
      render(<DashboardLayout />);

      expect(screen.queryByTestId('outlet')).not.toBeInTheDocument();
      expect(screen.getByText('navigation:forbidden.description')).toBeInTheDocument();
    });

    it('shows the page while the capabilities load', () => {
      mockCapabilities = {can: () => false, canManageRoot: false, isLoading: true};
      mockPathname = '/gateways';
      render(<DashboardLayout />);

      expect(screen.getByTestId('outlet')).toBeInTheDocument();
    });
  });

  it('renders footer', () => {
    render(<DashboardLayout />);

    const currentYear = new Date().getFullYear();
    expect(screen.getByText(new RegExp(currentYear.toString()))).toBeInTheDocument();
  });

  it('does not start a second sign-in after signing out', async () => {
    const user = userEvent.setup();
    mockSignOut.mockResolvedValue(undefined);

    render(<DashboardLayout />);

    // Open the user menu first
    const userMenuTrigger = screen.getByLabelText('Account');
    await user.click(userMenuTrigger);

    // Click sign out menu item
    const signOutButton = await screen.findByText('common:actions.signOut');
    await user.click(signOutButton);

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalled();
    });
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it('logs error when signOut fails', async () => {
    const user = userEvent.setup();
    const signOutError = new Error('Sign out failed');
    mockSignOut.mockRejectedValue(signOutError);

    render(<DashboardLayout />);

    // Open the user menu first
    const userMenuTrigger = screen.getByLabelText('Account');
    await user.click(userMenuTrigger);

    // Click sign out menu item
    const signOutButton = await screen.findByText('common:actions.signOut');
    await user.click(signOutButton);

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalled();
      expect(mockLoggerError).toHaveBeenCalledWith('Sign out failed', {error: signOutError});
    });
  });

  it('renders the user profile picture in the account menu when available', () => {
    mockUserData.mockReturnValue({
      name: 'Test User',
      email: 'test@example.com',
      picture: 'https://example.com/avatar.png',
    });

    render(<DashboardLayout />);

    const avatarImages = screen
      .getAllByRole<HTMLImageElement>('img')
      .filter((img) => img.src === 'https://example.com/avatar.png');
    expect(avatarImages.length).toBeGreaterThan(0);
  });

  it('renders two-letter initials in the account menu when there is no profile picture', () => {
    render(<DashboardLayout />);

    expect(screen.getAllByText('TU').length).toBeGreaterThan(0);
  });

  it('renders with fallback values when user data is missing', () => {
    mockUserData.mockReturnValue(null);

    render(<DashboardLayout />);

    expect(screen.getByTestId('outlet')).toBeInTheDocument();
  });

  it('renders with undefined user name and email', () => {
    mockUserData.mockReturnValue({name: undefined, email: undefined});

    render(<DashboardLayout />);

    expect(screen.getByTestId('outlet')).toBeInTheDocument();
  });

  describe('generic OIDC sign out', () => {
    let originalLocation: Location;

    beforeEach(() => {
      mockIsTrustedIssuerGenericOidc = true;
      mockSdkConfig = undefined;
      originalLocation = window.location;
      Object.defineProperty(window, 'location', {
        value: {...originalLocation, href: ''},
        writable: true,
        configurable: true,
      });
    });

    afterEach(() => {
      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
        configurable: true,
      });
    });

    it('clears local session and redirects to client URL when end_session_endpoint is missing', async () => {
      mockDiscovery = {wellKnown: {}};
      const user = userEvent.setup();

      render(<DashboardLayout />);

      const userMenuTrigger = screen.getByLabelText('Account');
      await user.click(userMenuTrigger);

      const signOutButton = await screen.findByText('common:actions.signOut');
      await user.click(signOutButton);

      expect(mockClearSession).toHaveBeenCalled();
      expect(mockLoggerWarn).toHaveBeenCalledWith(expect.stringContaining('end_session_endpoint missing'));
      expect(window.location.href).toBe('https://localhost:5191/console');
    });

    it('clears local session and redirects to IdP end_session_endpoint when available', async () => {
      mockDiscovery = {wellKnown: {end_session_endpoint: 'https://idp.example.com/logout'}};
      const user = userEvent.setup();

      render(<DashboardLayout />);

      const userMenuTrigger = screen.getByLabelText('Account');
      await user.click(userMenuTrigger);

      const signOutButton = await screen.findByText('common:actions.signOut');
      await user.click(signOutButton);

      expect(mockClearSession).toHaveBeenCalled();
      expect(window.location.href).toContain('https://idp.example.com/logout');
      expect(window.location.href).toContain('client_id=test-client-id');
    });

    it('logs error when clearSession throws during generic OIDC sign out', async () => {
      mockDiscovery = {wellKnown: {end_session_endpoint: 'https://idp.example.com/logout'}};
      const sessionError = new Error('session clear failed');
      mockClearSession.mockImplementation(() => {
        throw sessionError;
      });
      const user = userEvent.setup();

      render(<DashboardLayout />);

      const userMenuTrigger = screen.getByLabelText('Account');
      await user.click(userMenuTrigger);

      const signOutButton = await screen.findByText('common:actions.signOut');
      await user.click(signOutButton);

      expect(mockLoggerError).toHaveBeenCalledWith(expect.stringContaining('Failed to clear local session'), {
        error: sessionError,
      });
    });

    it('signs out through the backend when the SDK runs behind a BFF', async () => {
      mockSdkConfig = {endpoints: {backend: {signOut: '/_auth/iam/logout'}}};
      mockDiscovery = {wellKnown: {end_session_endpoint: 'https://console.example.com/oauth2/logout'}};
      mockSignOut.mockResolvedValue(undefined);
      const user = userEvent.setup();

      render(<DashboardLayout />);

      const userMenuTrigger = screen.getByLabelText('Account');
      await user.click(userMenuTrigger);

      const signOutButton = await screen.findByText('common:actions.signOut');
      await user.click(signOutButton);

      expect(mockSignOut).toHaveBeenCalled();
      expect(mockClearSession).not.toHaveBeenCalled();
      expect(window.location.href).toBe('');
    });
  });
});
