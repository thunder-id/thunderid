// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cleanup, render, screen} from '@testing-library/react';
import {ThemeProvider} from '@thunderid/react';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AccountLayout from '../layouts/AccountLayout';

const mockConfig = {
  // The logo of the application the app signs in with, from the flow metadata.
  appLogoUrl: undefined as string | undefined,
  // The logo set in the app's runtime configuration.
  runtimeLogo: undefined as string | undefined,
  config: {
    branding: {logoUrl: undefined as string | undefined, showSecuredBy: true, title: 'Acme'},
    content: {width: 'CONTAINED'},
    navigation: {layout: 'SIDE_BAR'},
  },
  visiblePages: ['profile', 'security'],
};

vi.mock('../hooks/useAccountAppConfig', () => ({default: () => mockConfig}));

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
    children({email: 'aiko@acme.io', name: 'Aiko Tanaka'}),
}));

const renderLayout = (): void => {
  function Wrapper(): JSX.Element {
    return (
      <OxygenUIThemeProvider>
        <ThemeProvider>
          <MemoryRouter initialEntries={['/security']}>
            <AccountLayout />
          </MemoryRouter>
        </ThemeProvider>
      </OxygenUIThemeProvider>
    );
  }

  render(<Wrapper />);
};

const navLinks = (): string[] =>
  screen
    .getAllByRole('link')
    .map((link: HTMLElement) => link.textContent ?? '')
    .filter((text: string) => text.length > 0);

describe('AccountLayout', () => {
  beforeEach(() => {
    mockConfig.config.navigation.layout = 'SIDE_BAR';
    mockConfig.config.content.width = 'CONTAINED';
    mockConfig.config.branding.showSecuredBy = true;
    mockConfig.config.branding.logoUrl = undefined;
    mockConfig.appLogoUrl = undefined;
    mockConfig.runtimeLogo = undefined;
    mockConfig.visiblePages = ['profile', 'security'];
  });

  it('lists home and each page the Console turned on, and marks the current one', () => {
    renderLayout();

    const nav: HTMLElement = screen.getByRole('navigation', {name: 'Account'});
    expect(nav).toBeTruthy();
    expect(navLinks()).toEqual(['Home', 'Personal info', 'Security']);
    expect(screen.getByRole('link', {name: 'Security'}).getAttribute('aria-current')).toBe('page');
    expect(screen.getByText('Secured by ThunderID')).toBeTruthy();
  });

  it('keeps the page in a centered frame when contained, and spans the window when full width', () => {
    renderLayout();
    const contained: HTMLElement = screen.getByRole('main').parentElement!;
    expect(getComputedStyle(contained).maxWidth).toBe('1240px');
    cleanup();

    mockConfig.config.content.width = 'FULL';
    renderLayout();
    expect(getComputedStyle(screen.getByRole('main').parentElement!).maxWidth).not.toBe('1240px');
  });

  it('leaves out pages that are off and hides the footer when configured', () => {
    mockConfig.config.branding.showSecuredBy = false;
    mockConfig.visiblePages = ['profile'];

    renderLayout();

    expect(navLinks()).toEqual(['Home', 'Personal info']);
    expect(screen.getByRole('link', {name: 'Home'}).getAttribute('aria-current')).toBe('page');
    expect(screen.queryByText('Secured by ThunderID')).toBeNull();
  });

  it("shows the application's logo, not ThunderID's, when the app sets none", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';

    renderLayout();

    expect(screen.getByRole('img', {name: 'Acme'}).getAttribute('src')).toBe('https://acme.example/app-logo.svg');
    expect(document.querySelector('img[src*="assets/images/logo"]')).toBeNull();
  });

  it("prefers the logo in the app's configuration over the application logo", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';
    mockConfig.config.branding.logoUrl = 'https://acme.example/account-logo.svg';

    renderLayout();

    expect(screen.getByRole('img', {name: 'Acme'}).getAttribute('src')).toBe('https://acme.example/account-logo.svg');
  });

  it("prefers the logo in the app's runtime configuration over every other", () => {
    mockConfig.appLogoUrl = 'https://acme.example/app-logo.svg';
    mockConfig.config.branding.logoUrl = 'https://acme.example/configured-logo.svg';
    mockConfig.runtimeLogo = 'https://acme.example/runtime-logo.svg';

    renderLayout();

    expect(screen.getByRole('img', {name: 'Acme'}).getAttribute('src')).toBe('https://acme.example/runtime-logo.svg');
  });
});
