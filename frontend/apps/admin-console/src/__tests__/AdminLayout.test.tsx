// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AdminLayout from '../layouts/AdminLayout';

const mockSignOut = vi.fn(() => Promise.resolve());
const mockClearSession = vi.fn();
let mockDiscovery: {wellKnown?: {end_session_endpoint?: string}} | undefined;
let mockIsTrustedIssuerGenericOidc = false;
let mockPermissions: string[] = [];
let mockOrganization: string | undefined;

vi.mock('@thunderid/react', () => ({
  SignOutButton: ({children}: {children: (props: {signOut: () => Promise<void>}) => React.ReactNode}) =>
    children({signOut: mockSignOut}),
  User: ({children}: {children: (user: unknown) => React.ReactNode}) =>
    children({email: 'admin@example.com', name: 'Admin User'}),
  useGrantedPermissions: () => ({
    can: (required: string): boolean =>
      mockPermissions.some((permission: string) => required === permission || required.startsWith(`${permission}:`)),
    isLoading: false,
    permissions: mockPermissions,
  }),
  useThunderID: () => ({clearSession: mockClearSession, discovery: mockDiscovery}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({
    getClientUrl: () => 'https://admin.example/console',
    getTrustedIssuerClientId: () => 'console-client',
    isTrustedIssuerGenericOidc: () => mockIsTrustedIssuerGenericOidc,
  }),
}));

vi.mock('../hooks/useOrganizationName', () => ({default: () => mockOrganization}));

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
    mockPermissions = ['system:user:view', 'system:group:view', 'system:role:view', 'system:agent:view'];
  });

  it('renders the nested route inside the shell', () => {
    render(<AdminLayout />);

    expect(screen.getByTestId('outlet')).toHaveTextContent('Outlet Content');
    expect(screen.getByText(/The ThunderID Authors/)).toBeInTheDocument();
  });

  it('shows the organization unit the admin manages beside the brand', () => {
    mockOrganization = 'Default';
    render(<AdminLayout />);

    expect(screen.getByText('Default')).toBeInTheDocument();
  });

  it('links every page the admin may view', () => {
    render(<AdminLayout />);

    expect(
      ['navigation:pages.users', 'navigation:pages.agents', 'navigation:pages.groups', 'navigation:pages.roles'].map(
        (label: string) => screen.getByText(label).closest('a')?.getAttribute('href'),
      ),
    ).toEqual(['/users', '/agents', '/groups', '/roles']);
  });

  it('leaves out pages the admin may not view', () => {
    mockPermissions = ['system:role:view'];
    render(<AdminLayout />);

    expect(screen.getByText('navigation:pages.roles')).toBeInTheDocument();
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
