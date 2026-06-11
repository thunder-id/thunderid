/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

vi.mock('../../features/applications/api/useGetApplications', () => ({
  default: (params: unknown) => mockUseGetApplications(params),
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
    expect(screen.getByText('navigation:pages.integrations')).toBeInTheDocument();
    expect(screen.getByText('navigation:pages.flows')).toBeInTheDocument();
  });

  it('renders footer', () => {
    render(<DashboardLayout />);

    const currentYear = new Date().getFullYear();
    expect(screen.getByText(new RegExp(currentYear.toString()))).toBeInTheDocument();
  });

  it('calls signIn after successful signOut when sign out is clicked', async () => {
    const user = userEvent.setup();
    mockSignOut.mockResolvedValue(undefined);
    mockSignIn.mockResolvedValue(undefined);

    render(<DashboardLayout />);

    // Open the user menu first
    const userMenuTrigger = screen.getByLabelText('Test User');
    await user.click(userMenuTrigger);

    // Click sign out menu item
    const signOutButton = await screen.findByText('common:userMenu.signOut');
    await user.click(signOutButton);

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalled();
      expect(mockSignIn).toHaveBeenCalled();
    });
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
      expect(mockLoggerError).toHaveBeenCalledWith('Sign out/in failed', {error: signOutError});
    });
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

  it('navigates to open-project page when open project button is clicked', async () => {
    const user = userEvent.setup();
    render(<DashboardLayout />);

    const openProjectButton = screen.getByRole('button', {name: /navigation:pages\.openProject/i});
    await user.click(openProjectButton);

    expect(mockNavigate).toHaveBeenCalledWith('/open-project');
  });

  it('navigates to export page when export button is clicked', async () => {
    const user = userEvent.setup();
    render(<DashboardLayout />);

    const exportButton = screen.getByText('navigation:pages.export');
    expect(exportButton).toBeInTheDocument();
    await user.click(exportButton);
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
