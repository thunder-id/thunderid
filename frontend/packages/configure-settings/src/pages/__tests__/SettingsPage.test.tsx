// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, type RenderResult} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {ConfigProvider} from '@thunderid/contexts';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {MemoryRouter, useLocation} from 'react-router';
import {afterAll, beforeAll, beforeEach, describe, it, expect, vi} from 'vitest';

/** A product name no default carries, so a title that ignored the runtime branding would fail. */
const PRODUCT_NAME = 'Acme Identity';

/** Every section a control plane serves. */
const ALL_SECTIONS = ['cors', 'defaultResourceServer', 'session', 'flow', 'csp', 'accountAccess', 'notification'];

const sections = vi.hoisted(() => ({
  data: undefined as string[] | undefined,
  isLoading: false,
  error: null as Error | null,
  refetch: vi.fn(),
}));

vi.mock('../../api/useGetServerConfigSections', () => ({
  default: () => sections,
}));

vi.mock('../../components/sessions/SessionTimeoutsCard', () => ({
  default: () => (
    <div data-testid="session-section">
      <input aria-label="Session draft" />
    </div>
  ),
}));

vi.mock('../../components/cors/CorsSection', () => ({
  default: () => (
    <div data-testid="cors-section">
      <input aria-label="CORS draft" />
    </div>
  ),
}));

vi.mock('../../components/accountSecurity/AccountSecuritySection', () => ({
  default: () => <div data-testid="account-security-section" />,
}));

const {default: SettingsPage} = await import('../SettingsPage');

/** Shows the query of the current URL, where the page keeps the active tab. */
function Search(): JSX.Element {
  return <p data-testid="search">{useLocation().search}</p>;
}

/** The page at a URL, which renderWithProviders' own router cannot start from. */
const pageAt = (path: string): JSX.Element => (
  <MemoryRouter initialEntries={[path]}>
    <ConfigProvider>
      <OxygenUIThemeProvider>
        <SettingsPage />
        <Search />
      </OxygenUIThemeProvider>
    </ConfigProvider>
  </MemoryRouter>
);

const renderAt = (path = '/settings'): RenderResult => render(pageAt(path));

const tabNames = (): string[] => screen.getAllByRole('tab').map((tab) => tab.textContent ?? '');

describe('SettingsPage', () => {
  const runtimeConfig = window.__THUNDERID_RUNTIME_CONFIG__;

  beforeAll(() => {
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {product_name: PRODUCT_NAME, favicon: {light: '', dark: ''}},
      client: {base: '/console', client_id: 'CONSOLE'},
    };
  });

  afterAll(() => {
    window.__THUNDERID_RUNTIME_CONFIG__ = runtimeConfig;
  });

  beforeEach(() => {
    sections.data = ALL_SECTIONS;
    sections.isLoading = false;
    sections.error = null;
    sections.refetch.mockReset();
  });

  it('renders the title and the subtitle', () => {
    renderAt();

    expect(screen.getByText('Configurations')).toBeInTheDocument();
    expect(screen.getByText('Configurations that apply across your system.')).toBeInTheDocument();
  });

  it('sets the document title with the product name the runtime configuration brands', async () => {
    renderAt();
    await waitFor(() => expect(document.title).toBe(`Configurations | ${PRODUCT_NAME}`));
  });

  it('shows a tab for each section the server serves, in order', () => {
    renderAt();

    expect(tabNames()).toEqual(['Account security', 'CORS', 'Sign-in session']);
    expect(screen.getByRole('tab', {name: 'Account security'})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('account-security-section')).toBeVisible();
  });

  it('leaves out the tab of a section the server does not serve', () => {
    sections.data = ['cors', 'accountAccess'];
    renderAt();

    expect(tabNames()).toEqual(['Account security', 'CORS']);
    expect(screen.queryByTestId('session-section')).not.toBeInTheDocument();
    expect(screen.getByTestId('account-security-section')).toBeVisible();
  });

  it('opens the tab the URL names', () => {
    renderAt('/settings?tab=cors');

    expect(screen.getByRole('tab', {name: 'CORS'})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('cors-section')).toBeVisible();
    expect(screen.getByTestId('session-section')).not.toBeVisible();
  });

  it.each(['/settings', '/settings?tab=unknown'])('opens the first tab at %s', (path) => {
    renderAt(path);

    expect(screen.getByRole('tab', {name: 'Account security'})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('account-security-section')).toBeVisible();
  });

  it('opens the first tab shown when the URL names one the server does not serve', () => {
    sections.data = ['cors', 'accountAccess'];
    renderAt('/settings?tab=sessions');

    expect(screen.getByRole('tab', {name: 'Account security'})).toHaveAttribute('aria-selected', 'true');
  });

  it('puts the tab chosen in the URL, keeping the rest of the query', async () => {
    renderAt('/settings?from=home');

    await userEvent.click(screen.getByRole('tab', {name: 'CORS'}));

    expect(screen.getByTestId('search')).toHaveTextContent('?from=home&tab=cors');
    expect(screen.getByTestId('cors-section')).toBeVisible();
    expect(screen.getByTestId('account-security-section')).not.toBeVisible();
  });

  it('keeps an unsaved edit when switching tabs and back', async () => {
    const user = userEvent.setup();
    renderAt('/settings?tab=cors');

    await user.type(screen.getByRole('textbox', {name: 'CORS draft'}), 'https://example.com');
    await user.click(screen.getByRole('tab', {name: 'Sign-in session'}));
    expect(screen.getByTestId('cors-section')).not.toBeVisible();
    expect(screen.getByTestId('session-section')).toBeVisible();

    await user.click(screen.getByRole('tab', {name: 'CORS'}));
    expect(screen.getByRole('textbox', {name: 'CORS draft'})).toHaveValue('https://example.com');
  });

  // A refresh that fails says so above the tabs, and leaves every panel mounted, so no draft is lost.
  it('keeps the tabs and every draft when a refresh of the section list fails', async () => {
    const user = userEvent.setup();
    const view = renderAt('/settings?tab=cors');
    await user.type(screen.getByRole('textbox', {name: 'CORS draft'}), 'https://example.com');

    sections.error = new Error('boom');
    view.rerender(pageAt('/settings?tab=cors'));

    expect(
      screen.getByText('Failed to refresh the configurations. The tabs show what was loaded before.'),
    ).toBeVisible();
    expect(tabNames()).toEqual(['Account security', 'CORS', 'Sign-in session']);
    expect(screen.getByRole('textbox', {name: 'CORS draft'})).toHaveValue('https://example.com');
    expect(screen.getByTestId('session-section')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(sections.refetch).toHaveBeenCalled();
  });

  it('shows neither tabs nor sections while the section list loads', () => {
    sections.data = undefined;
    sections.isLoading = true;
    renderAt();

    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByTestId('session-section')).not.toBeInTheDocument();
  });

  it('reports a failed section list in place of the tabs, with a retry', async () => {
    sections.data = undefined;
    sections.error = new Error('boom');
    renderAt();

    expect(screen.getByText('Failed to load the configurations')).toBeVisible();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(sections.refetch).toHaveBeenCalled();
  });
});
