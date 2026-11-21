// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DEFAULT_ADMIN_APP_CONFIG, type AppConfig} from '@thunderid/system-apps';
import {render, screen, userEvent} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AdminLayout from '../layouts/AdminLayout';

const mockSignOut = vi.fn(() => Promise.resolve());
const mockClearSession = vi.fn();
let mockDiscovery: {wellKnown?: {end_session_endpoint?: string}} | undefined;
let mockIsTrustedIssuerGenericOidc = false;
let mockPermissions: string[] = [];
let mockOrganization: string | undefined;
let mockAppConfig: AppConfig = DEFAULT_ADMIN_APP_CONFIG;

vi.mock('@thunderid/react', () => ({
  Attribution: () => <span>Secured by ThunderID</span>,
  SignOutButton: ({children}: {children: (props: {signOut: () => Promise<void>}) => React.ReactNode}) =>
    children({signOut: mockSignOut}),
  User: ({children}: {children: (user: unknown) => React.ReactNode}) =>
    children({email: 'admin@example.com', name: 'Admin User'}),
  useThunderID: () => ({clearSession: mockClearSession, discovery: mockDiscovery, meta: undefined}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useCapabilitiesContext: () => ({
    can: (resource: string): boolean => mockPermissions.includes(`system:${resource}:view`),
    isLoading: false,
  }),
  useConfig: () => ({
    config: {},
    getClientUrl: () => 'https://admin.example/console',
    getTrustedIssuerClientId: () => 'console-client',
    isTrustedIssuerGenericOidc: () => mockIsTrustedIssuerGenericOidc,
  }),
}));

vi.mock('../hooks/useOrganizationName', () => ({default: () => mockOrganization}));

vi.mock('../hooks/useAdminAppConfig', () => ({default: () => ({config: mockAppConfig, isLoading: false})}));

vi.mock('../hooks/useAppLanguage', () => ({default: vi.fn(), storeAppLanguage: vi.fn()}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({debug: vi.fn(), error: vi.fn(), info: vi.fn(), warn: vi.fn()}),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');

  return {
    ...actual,
    Link: ({children, to}: {children: React.ReactNode; to: string}) => <a href={to}>{children}</a>,
    Outlet: () => <div data-testid="outlet">Outlet Content</div>,
  };
});

describe('AdminLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDiscovery = {wellKnown: {end_session_endpoint: 'https://idp.example/logout'}};
    mockIsTrustedIssuerGenericOidc = false;
    mockOrganization = undefined;
    mockAppConfig = DEFAULT_ADMIN_APP_CONFIG;
    mockPermissions = ['system:user:view', 'system:group:view', 'system:role:view', 'system:agent:view'];
  });

  it('renders the nested route inside the shell', () => {
    render(<AdminLayout />);

    expect(screen.getByTestId('outlet')).toHaveTextContent('Outlet Content');
    expect(screen.getByText(/The ThunderID Authors/)).toBeInTheDocument();
  });

  it('takes its title, footer links and attribution from the Admin App configuration', () => {
    mockAppConfig = {
      ...DEFAULT_ADMIN_APP_CONFIG,
      branding: {
        ...DEFAULT_ADMIN_APP_CONFIG.branding,
        links: [{label: 'Privacy', url: 'https://example.com/privacy'}],
        showSecuredBy: true,
        title: 'Service Desk',
      },
    };
    render(<AdminLayout />);

    expect(screen.getByText('Service Desk')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Privacy'})).toHaveAttribute('href', 'https://example.com/privacy');
    expect(screen.getByText('Secured by ThunderID')).toBeInTheDocument();
  });

  it('leaves out the pages the Admin App turns off', () => {
    mockAppConfig = {
      ...DEFAULT_ADMIN_APP_CONFIG,
      pages: DEFAULT_ADMIN_APP_CONFIG.pages.map((page) => (page.key === 'agents' ? {...page, enabled: false} : page)),
    };
    render(<AdminLayout />);

    expect(screen.queryByText('navigation:pages.agents')).not.toBeInTheDocument();
    expect(screen.getByText('navigation:pages.users')).toBeInTheDocument();
  });

  it('shows the organization unit the admin manages beside the brand', () => {
    mockOrganization = 'Default';
    render(<AdminLayout />);

    expect(screen.getByText('Default')).toBeInTheDocument();
  });

  it('links every page the admin may view', async () => {
    render(<AdminLayout />);
    await screen.findByText('navigation:pages.users');

    expect(
      ['navigation:pages.users', 'navigation:pages.agents', 'navigation:pages.groups', 'navigation:pages.roles'].map(
        (label: string) => screen.getByText(label).closest('a')?.getAttribute('href'),
      ),
    ).toEqual(['/users', '/agents', '/groups', '/roles']);
  });

  it('leaves out pages the admin may not view', async () => {
    mockPermissions = ['system:role:view'];
    render(<AdminLayout />);

    expect(await screen.findByText('navigation:pages.roles')).toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.users')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.agents')).not.toBeInTheDocument();
    expect(screen.queryByText('navigation:pages.groups')).not.toBeInTheDocument();
  });

  it('signs out through the SDK', async () => {
    const user = userEvent.setup();
    render(<AdminLayout />);

    await user.click(screen.getByLabelText('Account'));
    await user.click(await screen.findByText('common:actions.signOut'));

    expect(mockSignOut).toHaveBeenCalledOnce();
  });

  it('signs out of a generic OIDC trusted issuer through its end-session endpoint', async () => {
    mockIsTrustedIssuerGenericOidc = true;
    const location: Location = window.location;
    const assigned: {href: string} = {href: ''};
    Object.defineProperty(window, 'location', {configurable: true, value: assigned});
    const user = userEvent.setup();

    render(<AdminLayout />);
    await user.click(screen.getByLabelText('Account'));
    await user.click(await screen.findByText('common:actions.signOut'));

    Object.defineProperty(window, 'location', {configurable: true, value: location});
    expect(mockSignOut).not.toHaveBeenCalled();
    expect(mockClearSession).toHaveBeenCalledOnce();
    expect(assigned.href).toBe(
      'https://idp.example/logout?client_id=console-client&post_logout_redirect_uri=https%3A%2F%2Fadmin.example%2Fconsole',
    );
  });
});
