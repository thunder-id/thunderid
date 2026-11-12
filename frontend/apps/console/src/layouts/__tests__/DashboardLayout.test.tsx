// Copyright 2025 The ThunderID Authors
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
let mockCapabilities: {
  can: (resource: string, action?: string) => boolean;
  isLoading: boolean;
  error?: Error | null;
  retry?: () => void;
};
let mockPathname: string | undefined;

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
    useCapabilitiesContext: () => mockCapabilities,
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
  return {
    ...actual,
    Outlet: () => <div data-testid="outlet">Outlet Content</div>,
    Link: ({children, to}: {children: React.ReactNode; to: string}) => (
      <a href={to} data-testid="router-link">
        {children}
      </a>
    ),
    useNavigate: () => mockNavigate,
    useLocation: () => (mockPathname ? {...actual.useLocation(), pathname: mockPathname} : actual.useLocation()),
  };
});

describe('DashboardLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    mockUserData.mockReturnValue({name: 'Test User', email: 'test@example.com'});
    mockIsTrustedIssuerGenericOidc = false;
    mockDiscovery = undefined;
    mockCapabilities = {can: () => true, isLoading: false};
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

  it('renders navigation categories', () => {
    render(<DashboardLayout />);

    // Check for category labels
    expect(screen.getByText('navigation:categories.identities')).toBeInTheDocument();
    expect(screen.getByText('navigation:categories.resources')).toBeInTheDocument();
  });

  // Deploying to gateways is a control plane's: this console, which every other server serves, offers
  // no way to it.
  it('offers no Deploy', () => {
    render(<DashboardLayout />);

    expect(screen.queryByTestId('header-deploy')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /deploy/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('link', {name: /deploy/i})).not.toBeInTheDocument();
  });

  it('renders navigation items', () => {
    render(<DashboardLayout />);

    // Check for navigation items using translation keys
    expect(screen.getByText('navigation:pages.users')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.userTypes')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.applications')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.connections')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.flows')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.authorizationDetailTypes')).toBeInTheDocument();
  });

  it('should list Agent types under Identities directly after User types', () => {
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

    const applicationIcon = screen.getByRole('button', {name: 'navigation:pages.applications'}).querySelector('svg');
    expect(applicationIcon).toBeInTheDocument();

    const navigationIcons = screen.getByRole('navigation').querySelectorAll('.MuiListItemIcon-root svg');
    expect(navigationIcons.length).toBeGreaterThan(0);
    navigationIcons.forEach((icon) => {
      expect(icon).toHaveAttribute('width', applicationIcon?.getAttribute('width'));
      expect(icon).toHaveAttribute('height', applicationIcon?.getAttribute('height'));
    });
  });

  describe('capabilities', () => {
    it('hides the pages the admin cannot view, and groups left empty', () => {
      mockCapabilities = {can: (resource) => resource === 'user', isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.users')).toBeInTheDocument();
      expect(screen.getByText('navigation:pages.home')).toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.applications')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.flows')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:categories.resources')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:pages.notifications')).not.toBeInTheDocument();
    });

    it('hides Agent types from admins who cannot view agent types', () => {
      mockCapabilities = {can: (resource) => resource !== 'agenttype', isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByRole('button', {name: 'navigation:pages.userTypes'})).toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'navigation:pages.agentTypes'})).not.toBeInTheDocument();
    });

    it('keeps a parent entry while one of its children is visible', () => {
      mockCapabilities = {can: (resource) => resource === 'vc', isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.verifiableCredentials')).toBeInTheDocument();
    });

    it('offers Import / Export only to admins who can import or export', () => {
      mockCapabilities = {can: (resource) => resource !== 'import' && resource !== 'export', isLoading: false};
      const {unmount} = render(<DashboardLayout />);

      expect(screen.queryByRole('button', {name: 'navigation:pages.importExport'})).not.toBeInTheDocument();
      unmount();

      mockCapabilities = {can: (resource, action) => resource === 'export' && action === 'create', isLoading: false};
      render(<DashboardLayout />);

      expect(screen.getByRole('button', {name: 'navigation:pages.importExport'})).toBeInTheDocument();
    });

    it('shows a notice instead of a page the admin cannot view', () => {
      mockCapabilities = {can: (resource) => resource !== 'app', isLoading: false};
      mockPathname = '/applications';
      render(<DashboardLayout />);

      expect(screen.queryByTestId('outlet')).not.toBeInTheDocument();
      expect(screen.getByText('navigation:forbidden.description')).toBeInTheDocument();
    });

    it('offers a retry instead of the page when the capabilities could not be read', () => {
      mockCapabilities = {can: () => false, isLoading: false, error: new Error('unavailable'), retry: vi.fn()};
      mockPathname = '/applications';
      render(<DashboardLayout />);

      expect(screen.queryByTestId('outlet')).not.toBeInTheDocument();
      expect(screen.queryByText('navigation:forbidden.description')).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: /refresh/i})).toBeInTheDocument();
    });

    it('shows the page while the capabilities load', () => {
      mockCapabilities = {can: () => false, isLoading: true};
      mockPathname = '/applications';
      render(<DashboardLayout />);

      expect(screen.getByTestId('outlet')).toBeInTheDocument();
    });

    it('keeps the navigation and Import / Export while the capabilities load', () => {
      mockCapabilities = {can: () => false, isLoading: true};
      render(<DashboardLayout />);

      expect(screen.getByText('navigation:pages.applications')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'navigation:pages.importExport'})).toBeInTheDocument();
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
