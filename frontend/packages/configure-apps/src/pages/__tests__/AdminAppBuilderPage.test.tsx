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
}));

vi.mock('@thunderid/design', () => ({
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

    fireEvent.click(screen.getByRole('button', {name: /^App layout/}));
    fireEvent.click(screen.getByRole('button', {name: 'Side bar'}));
    fireEvent.click(saveButton());

    expect((mockMutate.mock.calls[0][0] as AppConfig).navigation.layout).toBe('SIDE_BAR');
  });

  it('saves a Gate theme on the application the app signs in with, not on the app', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^Gate theme/}));
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

    fireEvent.click(screen.getByRole('button', {name: /^Gate layout/}));
    fireEvent.click(screen.getByRole('button', {name: /Split/}));
    fireEvent.click(saveButton());

    expect(mockSaveGate).toHaveBeenCalledWith({
      applicationId: '01900000-0000-7000-8000-0000000000f1',
      gate: {layoutId: 'layout-1'},
    });
  });

  it('saves an App theme color with the app configuration', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^App theme/}));
    const primary: HTMLElement = screen.getByRole('textbox', {name: 'Primary'});
    fireEvent.change(primary, {target: {value: '#0f766e'}});
    fireEvent.blur(primary);
    fireEvent.click(saveButton());

    expect((mockMutate.mock.calls[0][0] as AppConfig).appTheme).toEqual({colorSchemes: {light: {primary: '#0f766e'}}});
    expect(mockSaveGate).not.toHaveBeenCalled();
  });

  it('ignores a color the apps cannot apply', () => {
    renderWithProviders(<AdminAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^App theme/}));
    const primary: HTMLElement = screen.getByRole('textbox', {name: 'Primary'});
    fireEvent.change(primary, {target: {value: 'red; background: url(x)'}});
    fireEvent.blur(primary);

    expect(saveButton()).toBeDisabled();
  });
});
