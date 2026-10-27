// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {ThemeProvider} from '@thunderid/react';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AdminLayout from '../layouts/AdminLayout';

const mockConfig = {
  // The logo of the application the app signs in with, from the flow metadata.
  appLogoUrl: undefined as string | undefined,
  // The logo set in the app's runtime configuration.
  runtimeLogo: undefined as string | undefined,
  config: {
    branding: {logoUrl: undefined as string | undefined, showSecuredBy: true, title: 'Acme' as string | undefined},
    content: {width: 'CONTAINED'},
    navigation: {layout: 'TOP_BAR'},
  },
  visibleResources: ['users', 'roles'],
};

vi.mock('../hooks/useAdminAppConfig', () => ({default: () => mockConfig}));

vi.mock('@thunderid/logger/react', () => ({useLogger: () => ({error: vi.fn()})}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({config: {brand: {logo: mockConfig.runtimeLogo}}}),
}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useThunderID: () => ({meta: mockConfig.appLogoUrl ? {application: {logoUrl: mockConfig.appLogoUrl}} : null}),
  Attribution: (): JSX.Element => <span>Secured by ThunderID</span>,
  SignOutButton: ({children}: {children: (props: {signOut: () => Promise<void>}) => ReactNode}): ReactNode =>
    children({signOut: () => Promise.resolve()}),
  User: ({children}: {children: (user: Record<string, unknown>) => ReactNode}): ReactNode =>
    children({email: 'aiko@acme.io', name: 'Aiko Tanaka', ouHandle: 'engineering', ouName: 'Engineering'}),
}));

const renderLayout = (): void => {
  function Wrapper(): JSX.Element {
    return (
      <OxygenUIThemeProvider>
        <ThemeProvider>
          <MemoryRouter initialEntries={['/roles']}>
            <AdminLayout />
          </MemoryRouter>
        </ThemeProvider>
      </OxygenUIThemeProvider>
    );
  }

  render(<Wrapper />);
};

describe('AdminLayout', () => {
  beforeEach(() => {
    mockConfig.config.navigation.layout = 'TOP_BAR';
    mockConfig.config.branding.showSecuredBy = true;
    mockConfig.config.branding.logoUrl = undefined;
    mockConfig.appLogoUrl = undefined;
    mockConfig.runtimeLogo = undefined;
    mockConfig.config.branding.title = 'Acme';
  });

  it("shows the app's name and the organization unit, without a logo or avatar, when none is set", () => {
    mockConfig.config.branding.title = undefined;

    renderLayout();

    expect(screen.getByText('Admin')).toBeTruthy();
    expect(screen.getByLabelText('Organization unit').textContent).toBe('/EngineeringOU');
    expect(document.querySelector('header img')).toBeNull();
  });

  it('shows each page the Console turned on in the top bar, and the organization being managed', () => {
    renderLayout();

    const nav: HTMLElement = screen.getByRole('navigation', {name: 'Admin App'});
    expect(Array.from(nav.querySelectorAll('a')).map((link: HTMLAnchorElement) => link.textContent)).toEqual([
      'Users',
      'Roles',
    ]);
    expect(screen.getByRole('link', {name: 'Roles'}).getAttribute('aria-current')).toBe('page');
    expect(screen.getByText('Engineering')).toBeTruthy();
    expect(screen.getByText('Secured by ThunderID')).toBeTruthy();
  });

  it('places the pages beside the content and hides the footer when configured', () => {
    mockConfig.config.navigation.layout = 'SIDE_BAR';
    mockConfig.config.branding.showSecuredBy = false;

    renderLayout();

    const nav: HTMLElement = screen.getByRole('navigation', {name: 'Admin App'});
    expect(nav.closest('header')).toBeNull();
    expect(screen.queryByText('Secured by ThunderID')).toBeNull();
  });

  it("shows the application's logo, not ThunderID's, when the app sets none", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';

    renderLayout();

    const logo: HTMLImageElement = screen.getByRole('img', {name: 'Acme'});
    expect(logo.getAttribute('src')).toBe('https://acme.example/app-logo.svg');
    expect(document.querySelector('img[src*="assets/images/logo"]')).toBeNull();
  });

  it("prefers the logo in the app's configuration over the application logo", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';
    mockConfig.config.branding.logoUrl = 'https://acme.example/admin-logo.svg';

    renderLayout();

    expect(screen.getByRole('img', {name: 'Acme'}).getAttribute('src')).toBe('https://acme.example/admin-logo.svg');
  });

  it("prefers the logo in the app's runtime configuration over every other", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';
    mockConfig.config.branding.logoUrl = 'https://acme.example/configured-logo.svg';
    mockConfig.runtimeLogo = 'https://acme.example/runtime-logo.svg';

    renderLayout();

    expect(screen.getByRole('img', {name: 'Acme'}).getAttribute('src')).toBe('https://acme.example/runtime-logo.svg');
  });
});
