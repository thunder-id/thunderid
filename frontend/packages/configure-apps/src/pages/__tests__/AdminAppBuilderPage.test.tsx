// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, screen, within} from '@testing-library/react';
import {DEFAULT_ADMIN_APP_CONFIG} from '@thunderid/system-apps';
import type {AppConfig, AppPageConfig} from '@thunderid/system-apps';
import {renderWithProviders} from '@thunderid/test-utils';
import type {JSX, ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AdminAppBuilderPage from '../AdminAppBuilderPage';

const mockMutate = vi.fn((config: unknown) => Promise.resolve(config));
const mockSaveGate = vi.fn(({gate}: {gate: unknown}) => Promise.resolve(gate));
const mockQuery: {data: AppConfig | undefined; error: Error | null; isLoading: boolean} = {
  data: DEFAULT_ADMIN_APP_CONFIG,
  error: null,
  isLoading: false,
};

vi.mock('@thunderid/system-apps', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useGetAppConfig: () => ({...mockQuery, refetch: vi.fn()}),
}));

vi.mock('../../api/useUpdateAppConfig', () => ({
  default: () => ({error: null, isPending: false, mutateAsync: mockMutate, reset: vi.fn()}),
}));

vi.mock('../../api/useGetGateDesign', () => ({
  default: () => ({data: {}, isError: false}),
}));

vi.mock('../../api/useGetSignInApplicationLogo', () => ({
  default: () => ({data: 'https://acme.example/app-logo.svg'}),
}));

vi.mock('../../api/useUpdateGateDesign', () => ({
  default: () => ({error: null, isPending: false, mutateAsync: mockSaveGate, reset: vi.fn()}),
}));

vi.mock('@thunderid/configure-design', () => ({
  ElementInspector: ({children}: {children: ReactNode}) => children,
  PreviewFrame: ({children}: {children: ReactNode}) => children,
  PreviewToolbar: ({extraContent, startContent}: {extraContent: ReactNode; startContent: ReactNode}) => (
    <>
      {startContent}
      {extraContent}
    </>
  ),
  GatePreview: () => <p>sign-in preview</p>,
  LayoutPresetThumbnail: () => null,
  LayoutThumbnail: () => null,
  ThemeThumbnail: ({theme}: {theme: {displayName: string}}) => <span>{theme.displayName} thumbnail</span>,
  useDesignRoutes: () => ({
    design: {
      layoutDetail: (id: string) => `/design/layouts/${id}`,
      list: () => '/design',
      themeDetail: (id: string) => `/design/themes/${id}`,
    },
  }),
}));

vi.mock('@thunderid/design', () => ({
  createThemeFromDesign: () => undefined,
  DefaultTheme: {},
  useGetLayouts: () => ({data: {layouts: [{displayName: 'Split', handle: 'split', id: 'layout-1'}]}}),
  useGetTheme: () => ({data: undefined}),
  useGetThemes: () => ({data: {themes: [{displayName: 'Acme teal', handle: 'acme', id: 'theme-1'}]}}),
}));

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  const list = (name: string) =>
    function MockList(): JSX.Element {
      return <p>{name} preview</p>;
    };

  return {
    ...actual,
    Attribution: (): JSX.Element => <span>Secured by ThunderID</span>,
    ThemeProvider: ({children}: {children: JSX.Element}): JSX.Element => children,
    BaseAgentList: list('agents'),
    BaseGroupList: list('groups'),
    BaseRoleList: list('roles'),
    BaseUserList: list('users'),
    useTranslation: () => ({t: (key: string) => key}),
  };
});

const saveButton = (): HTMLButtonElement => screen.getByRole('button', {name: 'Save'});

describe('AdminAppBuilderPage', () => {
  beforeEach(() => {
    mockMutate.mockClear();
    mockSaveGate.mockClear();
    mockQuery.data = DEFAULT_ADMIN_APP_CONFIG;
    mockQuery.error = null;
    mockQuery.isLoading = false;
  });

  it('starts with nothing to save and previews the first page', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    expect(saveButton().disabled).toBe(true);
    expect(screen.getByText('users preview')).toBeInTheDocument();
  });

  it('saves the configuration with a hidden page', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('checkbox', {name: 'Show Groups in the app'}));
    expect(saveButton().disabled).toBe(false);
    fireEvent.click(saveButton());

    const saved: AppConfig = mockMutate.mock.calls[0][0] as AppConfig;
    expect(saved.pages.find((page: AppPageConfig) => page.key === 'groups')?.enabled).toBe(false);
  });

  it('undoes an edit back to the saved state', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('checkbox', {name: 'Show Roles in the app'}));
    fireEvent.click(screen.getByRole('button', {name: 'Undo'}));

    expect(saveButton().disabled).toBe(true);
  });

  it('shows the selected page in the preview and its permission', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /Agents/}));
    fireEvent.click(screen.getByRole('button', {name: 'By permission'}));

    expect(screen.getByText('agents preview')).toBeInTheDocument();
    expect(screen.getByText('Only admins whose token carries system:agent:view see this page.')).toBeInTheDocument();
  });

  it('opens a page from the navigation inside the preview', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(within(screen.getByRole('region', {name: 'Admin App preview'})).getByText('Groups'));

    expect(screen.getByText('groups preview')).toBeInTheDocument();
    expect(screen.getByText('groups preview').closest('[data-preview-component]')).toHaveAttribute(
      'data-preview-component',
      '<GroupList />',
    );
  });

  it('switches the preview to a side bar from the appearance settings', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Layout/}));
    fireEvent.click(screen.getByRole('button', {name: 'Side bar'}));
    fireEvent.click(saveButton());

    expect((mockMutate.mock.calls[0][0] as AppConfig).navigation.layout).toBe('SIDE_BAR');
  });

  it('saves a Gate theme on the application the app signs in with, not on the app', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Theme/}));
    fireEvent.click(screen.getByRole('button', {name: 'Gate'}));
    fireEvent.click(screen.getByRole('button', {name: /Acme teal/}));

    expect(screen.getByText('sign-in preview')).toBeInTheDocument();

    fireEvent.click(saveButton());

    expect(mockSaveGate).toHaveBeenCalledWith({
      applicationId: '01900000-0000-7000-8000-0000000000f1',
      gate: {themeId: 'theme-1'},
    });
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('saves a Gate layout on the application', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Layout/}));

    expect(screen.getByText('Default layout')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Change'}));
    fireEvent.click(within(screen.getByRole('group', {name: 'Gate layout'})).getByRole('button', {name: /Split/}));

    expect(screen.getByText('sign-in preview')).toBeInTheDocument();

    fireEvent.click(saveButton());

    expect(mockSaveGate).toHaveBeenCalledWith({
      applicationId: '01900000-0000-7000-8000-0000000000f1',
      gate: {layoutId: 'layout-1'},
    });
  });

  it('gives the app its own Design theme, saved with the app configuration', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Theme/}));
    fireEvent.click(screen.getByRole('switch', {name: 'Same theme as the Gate'}));
    fireEvent.click(within(screen.getByRole('group', {name: 'App theme'})).getByRole('button', {name: /Acme teal/}));

    expect(screen.getByText('Used by the App')).toBeInTheDocument();
    expect(screen.getByText('Separate Gate and App themes')).toBeInTheDocument();

    fireEvent.click(saveButton());

    expect((mockMutate.mock.calls[0][0] as AppConfig).themeId).toBe('theme-1');
    expect(mockSaveGate).not.toHaveBeenCalled();
  });

  it('starts an own theme from the Gate theme, and follows the Gate again', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Theme/}));
    fireEvent.click(within(screen.getByRole('group', {name: 'Gate theme'})).getByRole('button', {name: /Acme teal/}));

    expect(screen.getByText('Follows the Gate theme')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('switch', {name: 'Same theme as the Gate'}));

    expect(screen.getByText('Used by the App')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Edit Acme teal in Themes'})).toHaveAttribute(
      'href',
      '/design/themes/theme-1',
    );

    fireEvent.click(screen.getByRole('switch', {name: 'Same theme as the Gate'}));

    expect(screen.getByText('One theme for Gate and App')).toBeInTheDocument();
    expect(screen.getByText('Follows the Gate theme')).toBeInTheDocument();
  });

  it('shows the shared Gate theme once one is chosen, and offers the themes again to change it', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Theme/}));
    fireEvent.click(screen.getByRole('button', {name: /Acme teal/}));

    expect(screen.getByText('Used by the Gate and the App')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Default theme/})).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Change'}));

    expect(screen.getByRole('button', {name: /Default theme/})).toBeInTheDocument();
  });

  it("shows the application's logo by default, and saves light and dark logos of the app's own", () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Branding/}));

    expect(screen.getByText('Application logo')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('switch', {name: 'Same logo as the application'}));
    const light: HTMLElement = screen.getByRole('textbox', {name: 'Light'});
    fireEvent.change(light, {target: {value: 'https://cdn.example.com/admin-logo.svg'}});
    fireEvent.blur(light);
    const dark: HTMLElement = screen.getByRole('textbox', {name: 'Dark'});
    fireEvent.change(dark, {target: {value: 'https://cdn.example.com/admin-logo-dark.svg'}});
    fireEvent.blur(dark);
    fireEvent.click(saveButton());

    expect((mockMutate.mock.calls[0][0] as AppConfig).branding.logo).toEqual({
      dark: 'https://cdn.example.com/admin-logo-dark.svg',
      light: 'https://cdn.example.com/admin-logo.svg',
    });
    expect(mockSaveGate).not.toHaveBeenCalled();
  });

  it('refuses a logo URL the app cannot load', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Branding/}));
    fireEvent.click(screen.getByRole('switch', {name: 'Same logo as the application'}));
    const light: HTMLElement = screen.getByRole('textbox', {name: 'Light'});
    fireEvent.change(light, {target: {value: 'javascript:alert(1)'}});
    fireEvent.blur(light);

    expect(screen.getByText('Use an http or https URL.')).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
  });

  it("saves a favicon of the app's own", () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Branding/}));
    fireEvent.click(screen.getByRole('switch', {name: "Same icon as the app's own"}));
    const light: HTMLElement = screen.getByRole('textbox', {name: 'Light'});
    fireEvent.change(light, {target: {value: 'https://cdn.example.com/admin-icon.png'}});
    fireEvent.blur(light);
    fireEvent.click(saveButton());

    expect((mockMutate.mock.calls[0][0] as AppConfig).branding.favicon).toEqual({
      light: 'https://cdn.example.com/admin-icon.png',
    });
  });

  it('turns on a language menu that opens in the first language offered', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Layout/}));
    fireEvent.click(screen.getByRole('switch', {name: /Language menu/}));

    expect(screen.getByRole('group', {name: 'Languages offered'})).toBeInTheDocument();

    fireEvent.click(saveButton());

    const {languages} = mockMutate.mock.calls[0][0] as AppConfig;

    expect(languages?.menu).toBe(true);
    expect(languages?.offered).toEqual([languages?.default]);
  });

  it('saves the app name, and footer links once they have a label and a URL', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Branding/}));
    const name: HTMLElement = screen.getByRole('textbox', {name: 'App name'});
    fireEvent.change(name, {target: {value: 'Acme Admin'}});
    fireEvent.blur(name);
    fireEvent.click(screen.getByRole('button', {name: 'Add link'}));
    const label: HTMLElement = screen.getByRole('textbox', {name: 'Label'});
    fireEvent.change(label, {target: {value: 'Privacy'}});
    fireEvent.blur(label);
    const url: HTMLElement = screen.getByRole('textbox', {name: 'URL'});
    fireEvent.change(url, {target: {value: 'https://acme.example/privacy'}});
    fireEvent.blur(url);
    fireEvent.click(saveButton());

    const {branding} = mockMutate.mock.calls[0][0] as AppConfig;

    expect(branding.title).toBe('Acme Admin');
    expect(branding.links).toEqual([{label: 'Privacy', url: 'https://acme.example/privacy'}]);
  });

  it('keeps a footer link out of the configuration until its URL is one the app can open', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Branding/}));
    fireEvent.click(screen.getByRole('button', {name: 'Add link'}));
    fireEvent.change(screen.getByRole('textbox', {name: 'Label'}), {target: {value: 'Terms'}});
    const url: HTMLElement = screen.getByRole('textbox', {name: 'URL'});
    fireEvent.change(url, {target: {value: 'terms.html'}});
    fireEvent.blur(url);

    expect(screen.getByText('Use an http or https URL.')).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
  });

  it('moves the "Secured by ThunderID" switch to the branding footer', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Branding/}));
    fireEvent.click(screen.getByRole('checkbox', {name: 'Show "Secured by ThunderID"'}));
    fireEvent.click(saveButton());

    expect((mockMutate.mock.calls[0][0] as AppConfig).branding.showSecuredBy).toBe(
      !DEFAULT_ADMIN_APP_CONFIG.branding.showSecuredBy,
    );
  });
});
