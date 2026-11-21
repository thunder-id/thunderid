// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cleanup, render, screen} from '@testing-library/react';
import {ThemeProvider} from '@thunderid/react';
import type {AppConfig, AppLink} from '@thunderid/system-apps';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AccountLayout from '../layouts/AccountLayout';

const mockConfig = {
  // The logo of the application the app signs in with, from the flow metadata.
  appLogoUrl: undefined as string | undefined,
  // The logo set in the app's runtime configuration (`brand.logo` in config.js).
  runtimeLogo: undefined as string | undefined,
  config: {
    branding: {links: [] as AppLink[], logo: undefined as AppConfig['branding']['logo'], showSecuredBy: true},
    content: {width: 'CONTAINED'},
    navigation: {layout: 'SIDE_BAR'},
  },
  visiblePages: ['profile', 'security'],
};

vi.mock('../hooks/useAccountAppConfig', () => ({default: () => mockConfig}));

vi.mock('../hooks/useAppLanguage', () => ({default: () => undefined}));

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
    children({email: 'aiko@acme.io', name: 'Aiko Tanaka', ouId: '01900000-0000-7000-8000-000000000001'}),
}));

const renderLayout = (): void => {
  render(
    <OxygenUIThemeProvider>
      <ThemeProvider>
        <MemoryRouter initialEntries={['/security']}>
          <AccountLayout />
        </MemoryRouter>
      </ThemeProvider>
    </OxygenUIThemeProvider>,
  );
};

const navLinks = (): string[] =>
  Array.from(screen.getByRole('navigation', {name: 'MyAccount'}).querySelectorAll('a')).map(
    (link: HTMLAnchorElement) => link.textContent ?? '',
  );

describe('AccountLayout', () => {
  beforeEach(() => {
    mockConfig.config.navigation.layout = 'SIDE_BAR';
    mockConfig.config.content.width = 'CONTAINED';
    mockConfig.config.branding.showSecuredBy = true;
    mockConfig.config.branding.links = [];
    mockConfig.config.branding.logo = undefined;
    mockConfig.appLogoUrl = undefined;
    mockConfig.runtimeLogo = undefined;
    mockConfig.visiblePages = ['profile', 'security'];
  });

  it('lists home and each page the Console turned on, and marks the current one', () => {
    renderLayout();

    expect(navLinks()).toEqual(['Home', 'Personal info', 'Security']);
    expect(screen.getByRole('link', {name: 'Security'}).getAttribute('aria-current')).toBe('page');
    expect(screen.getByRole('contentinfo')).toHaveTextContent('Secured by ThunderID');
  });

  it('runs the navigation as tabs under the top bar with the top bar layout', () => {
    mockConfig.config.navigation.layout = 'TOP_BAR';

    renderLayout();

    expect(screen.getByRole('banner')).toContainElement(screen.getByRole('navigation', {name: 'MyAccount'}));
    expect(navLinks()).toEqual(['Home', 'Personal info', 'Security']);
  });

  it('keeps the page in a centered frame when contained, and spans the window when full width', () => {
    renderLayout();
    expect(getComputedStyle(screen.getByRole('main').parentElement!).maxWidth).toBe('1240px');
    cleanup();

    mockConfig.config.content.width = 'FULL';
    renderLayout();
    expect(getComputedStyle(screen.getByRole('main').parentElement!).maxWidth).not.toBe('1240px');
  });

  it('leaves out pages that are off, and the footer when it has nothing to show', () => {
    mockConfig.config.branding.showSecuredBy = false;
    mockConfig.visiblePages = ['profile'];

    renderLayout();

    expect(navLinks()).toEqual(['Home', 'Personal info']);
    expect(screen.getByRole('link', {name: 'Home'}).getAttribute('aria-current')).toBe('page');
    expect(screen.queryByRole('contentinfo')).toBeNull();
  });

  it('shows the footer links the Console set', () => {
    mockConfig.config.branding.showSecuredBy = false;
    mockConfig.config.branding.links = [{label: 'Privacy', url: 'https://acme.example/privacy'}];

    renderLayout();

    const privacy: HTMLElement = screen.getByRole('link', {name: 'Privacy'});
    expect(screen.getByRole('contentinfo')).toContainElement(privacy);
    expect(privacy.getAttribute('href')).toBe('https://acme.example/privacy');
    expect(screen.queryByText('Secured by ThunderID')).toBeNull();
  });

  it("shows the app's name without the user's organization unit", () => {
    renderLayout();

    expect(screen.getByText('MyAccount')).toBeTruthy();
    expect(screen.queryByLabelText('Organization unit')).toBeNull();
  });

  it("shows the application's logo when nothing else sets one", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';

    renderLayout();

    expect(screen.getByRole('img', {name: 'MyAccount'}).getAttribute('src')).toBe('https://acme.example/app-logo.svg');
  });

  it("prefers the logo in the app's runtime configuration over every other", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';
    mockConfig.config.branding.logo = {light: 'https://acme.example/configured-logo.svg'};
    mockConfig.runtimeLogo = 'https://acme.example/runtime-logo.svg';

    renderLayout();

    expect(screen.getByRole('img', {name: 'MyAccount'}).getAttribute('src')).toBe(
      'https://acme.example/runtime-logo.svg',
    );
  });
});
