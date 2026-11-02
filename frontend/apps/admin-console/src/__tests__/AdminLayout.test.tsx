// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter, Route, Routes} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AdminLayout from '../layouts/AdminLayout';

const state: {genericOidc: boolean; ouName?: string; permissions: string[]} = {genericOidc: false, permissions: []};
const signOut = vi.fn(() => Promise.resolve());

vi.mock('@thunderid/react', () => ({
  Attribution: (): JSX.Element => <p>Secured by ThunderID</p>,
  Logo: ({alt}: {alt: string}): JSX.Element => <img alt={alt} />,
  SignOutButton: ({children}: {children: (props: {signOut: () => Promise<void>}) => ReactNode}) => children({signOut}),
  User: ({children}: {children: (user: Record<string, unknown>) => ReactNode}) =>
    children({email: 'admin@example.com', name: 'admin'}),
  useGrantedPermissions: () => ({
    can: (required: string): boolean =>
      state.permissions.some((permission: string) => required === permission || required.startsWith(`${permission}:`)),
    isLoading: false,
    permissions: state.permissions,
  }),
  useThunderID: () => ({
    clearSession: vi.fn(),
    discovery: {wellKnown: {end_session_endpoint: 'https://idp.example/logout'}},
    meta: {application: {logoUrl: 'https://acme.example/logo.svg'}},
  }),
}));

vi.mock('../hooks/useOrganizationName', () => ({default: () => state.ouName}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getClientUrl: () => 'https://admin.example/console',
    getTrustedIssuerClientId: () => 'console-client',
    isTrustedIssuerGenericOidc: () => state.genericOidc,
  }),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), warn: vi.fn()}),
}));

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  const {createElement} = await import('react');
  const UserMenu = ({children}: {children: ReactNode}): JSX.Element => createElement('div', null, children);

  return {
    ...actual,
    ColorSchemeToggle: (): JSX.Element => createElement('button', {type: 'button'}, 'scheme'),
    UserMenu: Object.assign(UserMenu, {
      Divider: (): null => null,
      Header: (): null => null,
      Logout: ({label, onClick}: {label: string; onClick: () => void}): JSX.Element =>
        createElement('button', {onClick, type: 'button'}, label),
      Trigger: ({name}: {name: string}): JSX.Element => createElement('span', null, name),
    }),
  };
});

const renderAt = (path: string): void => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route element={<AdminLayout />}>
          <Route path="*" element={<p>page content</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
};

describe('AdminLayout', () => {
  beforeEach(() => {
    state.genericOidc = false;
    state.ouName = undefined;
    state.permissions = ['system:user:view', 'system:group:view', 'system:role:view', 'system:agent:view'];
    signOut.mockClear();
  });

  it('shows a tab for every page the admin may view, marking the current one', () => {
    renderAt('/groups/g-1');
    const nav: HTMLElement = screen.getByRole('navigation', {name: 'Admin Console'});
    const tabs: string[] = within(nav)
      .getAllByRole('link')
      .map((link: HTMLElement) => link.textContent ?? '');

    expect(tabs).toEqual(['Users', 'Agents', 'Groups', 'Roles']);
    expect(within(nav).getByRole('link', {name: 'Groups'})).toHaveAttribute('aria-current', 'page');
    expect(screen.getByText('page content')).toBeInTheDocument();
  });

  it('leaves out pages the admin may not view', () => {
    state.permissions = ['system:role:view'];
    renderAt('/roles');
    const nav: HTMLElement = screen.getByRole('navigation', {name: 'Admin Console'});

    expect(
      within(nav)
        .getAllByRole('link')
        .map((link: HTMLElement) => link.textContent),
    ).toEqual(['Roles']);
  });

  it('shows the brand, the organization unit and the attribution', () => {
    state.ouName = 'Default';
    renderAt('/users');

    expect(screen.getByRole('img', {name: 'Admin'})).toBeInTheDocument();
    expect(screen.getByRole('heading', {level: 1, name: 'Admin'})).toBeInTheDocument();
    expect(within(screen.getByLabelText('Organization unit')).getByText('Default')).toBeInTheDocument();
    expect(screen.getByText('Secured by ThunderID')).toBeInTheDocument();
  });

  it('signs out through the SDK', () => {
    renderAt('/users');
    fireEvent.click(screen.getByRole('button', {name: 'Sign out'}));

    expect(signOut).toHaveBeenCalledOnce();
  });

  it('signs out of a generic OIDC trusted issuer through its end-session endpoint', () => {
    state.genericOidc = true;
    const location: Location = window.location;
    const assigned: {href: string} = {href: ''};
    Object.defineProperty(window, 'location', {configurable: true, value: assigned});

    renderAt('/users');
    fireEvent.click(screen.getByRole('button', {name: 'Sign out'}));

    Object.defineProperty(window, 'location', {configurable: true, value: location});
    expect(signOut).not.toHaveBeenCalled();
    expect(assigned.href).toBe(
      'https://idp.example/logout?client_id=console-client&post_logout_redirect_uri=https%3A%2F%2Fadmin.example%2Fconsole',
    );
  });
});
