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
let mockIsControlPlane = false;

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
      },
      isTrustedIssuerGenericOidc: () => mockIsTrustedIssuerGenericOidc,
      isControlPlane: () => mockIsControlPlane,
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
  };
});

describe('DashboardLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    mockUserData.mockReturnValue({name: 'Test User', email: 'test@example.com'});
    mockIsTrustedIssuerGenericOidc = false;
    mockIsControlPlane = false;
    mockDiscovery = undefined;
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

  it('renders navigation items', () => {
    render(<DashboardLayout />);

    // Check for navigation items using translation keys
    expect(screen.getByText('navigation:pages.users')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.userTypes')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.applications')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.connections')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.flows')).toBeInTheDocument();
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

  it('hides gateway management outside the control plane', () => {
    render(<DashboardLayout />);

    expect(screen.getByText('navigation:pages.settings')).toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.gateways')).not.toBeInTheDocument();
  });

  it('shows gateway management on the control plane', () => {
    mockIsControlPlane = true;
    render(<DashboardLayout />);

    expect(screen.getByText('navigation:pages.gateways')).toBeInTheDocument();
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
    const userMenuTrigger = screen.getByLabelText('Test User');
    await user.click(userMenuTrigger);

    // Click sign out menu item
    const signOutButton = await screen.findByText('common:userMenu.signOut');
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
    const userMenuTrigger = screen.getByLabelText('Test User');
    await user.click(userMenuTrigger);

    // Click sign out menu item
    const signOutButton = await screen.findByText('common:userMenu.signOut');
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

  it('navigates to welcome page when welcome menu item is clicked', async () => {
    const user = userEvent.setup();
    render(<DashboardLayout />);

    const userMenuTrigger = screen.getByLabelText('Test User');
    await user.click(userMenuTrigger);

    const welcomeItem = await screen.findByText('common:userMenu.welcome');
    expect(welcomeItem).toBeInTheDocument();
    await user.click(welcomeItem);
  });

  describe('generic OIDC sign out', () => {
    let originalLocation: Location;

    beforeEach(() => {
      mockIsTrustedIssuerGenericOidc = true;
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

      const userMenuTrigger = screen.getByLabelText('Test User');
      await user.click(userMenuTrigger);

      const signOutButton = await screen.findByText('common:userMenu.signOut');
      await user.click(signOutButton);

      expect(mockClearSession).toHaveBeenCalled();
      expect(mockLoggerWarn).toHaveBeenCalledWith(expect.stringContaining('end_session_endpoint missing'));
      expect(window.location.href).toBe('https://localhost:5191/console');
    });

    it('clears local session and redirects to IdP end_session_endpoint when available', async () => {
      mockDiscovery = {wellKnown: {end_session_endpoint: 'https://idp.example.com/logout'}};
      const user = userEvent.setup();

      render(<DashboardLayout />);

      const userMenuTrigger = screen.getByLabelText('Test User');
      await user.click(userMenuTrigger);

      const signOutButton = await screen.findByText('common:userMenu.signOut');
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

      const userMenuTrigger = screen.getByLabelText('Test User');
      await user.click(userMenuTrigger);

      const signOutButton = await screen.findByText('common:userMenu.signOut');
      await user.click(signOutButton);

      expect(mockLoggerError).toHaveBeenCalledWith(expect.stringContaining('Failed to clear local session'), {
        error: sessionError,
      });
    });
  });
});
