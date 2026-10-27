// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, screen} from '@testing-library/react';
import {DEFAULT_ACCOUNT_APP_CONFIG} from '@thunderid/system-apps';
import type {AppConfig, AppPageConfig, AppType} from '@thunderid/system-apps';
import {renderWithProviders} from '@thunderid/test-utils';
import type {JSX, ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AccountAppBuilderPage from '../AccountAppBuilderPage';

const mockMutate = vi.fn((config: unknown) => Promise.resolve(config));
const mockSaveGate = vi.fn(({gate}: {gate: unknown}) => Promise.resolve(gate));
const mockRequestedTypes: AppType[] = [];

vi.mock('@thunderid/system-apps', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useGetAppConfig: (type: AppType) => {
    mockRequestedTypes.push(type);
    return {data: DEFAULT_ACCOUNT_APP_CONFIG, error: null, isLoading: false, refetch: vi.fn()};
  },
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
  useGetThemes: () => ({data: {themes: []}}),
}));

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  const preview = (name: string) =>
    function MockPreview(): JSX.Element {
      return <p>{name} preview</p>;
    };

  return {
    ...actual,
    Attribution: (): JSX.Element => <span>Secured by ThunderID</span>,
    ThemeProvider: ({children}: {children: JSX.Element}): JSX.Element => children,
    BaseChangeCredential: preview('security'),
    BaseUserProfile: preview('profile'),
  };
});

describe('AccountAppBuilderPage', () => {
  beforeEach(() => {
    mockMutate.mockClear();
    mockSaveGate.mockClear();
    mockRequestedTypes.length = 0;
  });

  it('builds the Account App from its own pages', () => {
    renderWithProviders(<AccountAppBuilderPage />);

    expect(mockRequestedTypes).toContain('ACCOUNT');
    expect(screen.getByRole('heading', {name: 'Account App'})).toBeInTheDocument();
    expect(screen.getByText('profile preview')).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', {name: 'Show Users in the app'})).not.toBeInTheDocument();
  });

  it('offers no permission gate for a page every user sees', () => {
    renderWithProviders(<AccountAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /Security/}));

    expect(screen.getByText('security preview')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'By permission'})).not.toBeInTheDocument();
  });

  it('saves the Account App with password changes turned off', () => {
    renderWithProviders(<AccountAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /Security/}));
    fireEvent.click(screen.getByRole('switch', {name: 'Change their password'}));
    fireEvent.click(screen.getByRole('button', {name: 'Save'}));

    const saved: AppConfig = mockMutate.mock.calls[0][0] as AppConfig;
    expect(saved.type).toBe('ACCOUNT');
    expect(saved.pages.find((page: AppPageConfig) => page.key === 'security')?.actions['password']).toBe(false);
  });

  it('starts contained and saves a full-width page from the appearance settings', () => {
    renderWithProviders(<AccountAppBuilderPage />);

    fireEvent.click(screen.getByRole('button', {name: /^App layout/}));
    expect(screen.getByRole('button', {name: 'Contained'}).getAttribute('aria-pressed')).toBe('true');
    fireEvent.click(screen.getByRole('button', {name: 'Full width'}));
    fireEvent.click(screen.getByRole('button', {name: 'Save'}));

    expect((mockMutate.mock.calls[0][0] as AppConfig).content.width).toBe('FULL');
  });
});
