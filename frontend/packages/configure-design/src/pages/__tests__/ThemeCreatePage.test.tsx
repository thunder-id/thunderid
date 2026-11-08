// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ThemeCreatePage from '../ThemeCreatePage';

interface MockThemesResult {
  data: {themes: {id: string; displayName: string}[]} | undefined;
}

const {mockNavigate, mockMutate, mockReset, mockCreateThemeState, mockUseGetTheme, mockUseGetThemes} = vi.hoisted(
  () => ({
    mockNavigate: vi.fn(),
    mockMutate: vi.fn(),
    mockReset: vi.fn(),
    mockCreateThemeState: {isPending: false, isError: false},
    mockUseGetTheme: vi.fn<(id: string) => {data: unknown}>(),
    mockUseGetThemes: vi.fn<() => MockThemesResult>(),
  }),
);

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    useCreateTheme: () => ({
      mutate: mockMutate,
      reset: mockReset,
      isPending: mockCreateThemeState.isPending,
      isError: mockCreateThemeState.isError,
    }),
    useGetTheme: (id: string) => mockUseGetTheme(id),
    useGetThemes: () => mockUseGetThemes(),
  };
});

vi.mock('../../GatePreview/GatePreview', () => ({
  default: () => <div data-testid="preview" />,
}));

vi.mock('../../components/create-theme/ConfigureThemeName', () => ({
  default: ({
    onThemeNameChange,
    onReadyChange,
  }: {
    onThemeNameChange: (name: string) => void;
    onReadyChange?: (isReady: boolean) => void;
  }) => (
    <div data-testid="step-name">
      <button type="button" onClick={() => onThemeNameChange('My Theme')}>
        Set Name
      </button>
      <button type="button" onClick={() => onReadyChange?.(true)}>
        Set Ready
      </button>
    </div>
  ),
}));

vi.mock('../../components/create-theme/ConfigureThemeColor', () => ({
  default: ({onPrimaryColorChange}: {onPrimaryColorChange: (color: string) => void}) => (
    <div data-testid="step-color">
      <button type="button" onClick={() => onPrimaryColorChange('#123456')}>
        Set Color
      </button>
    </div>
  ),
}));

describe('ThemeCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreateThemeState.isPending = false;
    mockCreateThemeState.isError = false;
    mockUseGetTheme.mockReturnValue({data: undefined});
    mockUseGetThemes.mockReturnValue({data: {themes: []}});
  });

  it('renders the name step by default', () => {
    render(<ThemeCreatePage />);
    expect(screen.getByTestId('step-name')).toBeInTheDocument();
  });

  it('moves to the color step once the name step reports ready and continue is clicked', async () => {
    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));

    expect(screen.getByTestId('step-color')).toBeInTheDocument();
  });

  it('calls mutate with a kebab-cased handle when Create Theme is clicked', async () => {
    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByText('Set Name'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({handle: 'my-theme', displayName: 'My Theme'}),
      expect.any(Object),
    );
  });

  it('displays a resolved error message on create failure, never the raw server text', async () => {
    mockMutate.mockImplementation((_data, opts: {onError: (err: Error) => void}) => {
      opts.onError(new Error('Network error'));
    });

    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(await screen.findByText('Failed to create theme. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('Network error')).not.toBeInTheDocument();
  });

  it('clears a stale create error and resets the mutation as soon as the name field changes', async () => {
    // Mirrors the mutation's own bookkeeping: a real TanStack mutation flips isError to true
    // before onError fires, so the next render already reflects it.
    mockMutate.mockImplementation((_data, opts: {onError: (err: Error) => void}) => {
      mockCreateThemeState.isError = true;
      opts.onError(new Error('Network error'));
    });

    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(await screen.findByText('Failed to create theme. Please try again.')).toBeInTheDocument();

    // Go back to the name step to trigger the name field-change handler.
    await user.click(screen.getByRole('button', {name: /Back/i}));
    await user.click(screen.getByText('Set Name'));

    expect(mockReset).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('Failed to create theme. Please try again.')).not.toBeInTheDocument();
  });

  it('clears a stale create error and resets the mutation as soon as the color field changes', async () => {
    // Mirrors the mutation's own bookkeeping: a real TanStack mutation flips isError to true
    // before onError fires, so the next render already reflects it.
    mockMutate.mockImplementation((_data, opts: {onError: (err: Error) => void}) => {
      mockCreateThemeState.isError = true;
      opts.onError(new Error('Network error'));
    });

    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(await screen.findByText('Failed to create theme. Please try again.')).toBeInTheDocument();

    await user.click(screen.getByText('Set Color'));

    expect(mockReset).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('Failed to create theme. Please try again.')).not.toBeInTheDocument();
  });

  it('does not reset the mutation on field change while a create is still pending', async () => {
    // A mutation cannot be isPending and isError simultaneously, so isError stays false here —
    // the same state a real in-flight TanStack mutation would report.
    mockCreateThemeState.isPending = true;

    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByText('Set Color'));

    expect(mockReset).not.toHaveBeenCalled();
  });

  it('navigates back to the design list when the wizard is closed', async () => {
    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByRole('button', {name: 'Close'}));

    expect(mockNavigate).toHaveBeenCalledWith('/design');
  });

  it('jumps back to the Details step when its breadcrumb is clicked from the color step', async () => {
    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    expect(screen.getByTestId('step-color')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Details'}));

    expect(screen.getByTestId('step-name')).toBeInTheDocument();
  });

  it('navigates to the created theme detail page on a successful create', async () => {
    mockMutate.mockImplementation((_data, opts: {onSuccess: (created: {id: string}) => void}) => {
      opts.onSuccess({id: 'theme-new'});
    });

    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(mockNavigate).toHaveBeenCalledWith('/design/themes/theme-new');
  });

  it('swallows a rejected navigation after a successful create instead of throwing', async () => {
    mockNavigate.mockRejectedValueOnce(new Error('navigation failed'));
    mockMutate.mockImplementation((_data, opts: {onSuccess: (created: {id: string}) => void}) => {
      opts.onSuccess({id: 'theme-new'});
    });

    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(mockNavigate).toHaveBeenCalledWith('/design/themes/theme-new');
  });

  it('picks the theme named "Classic" (case-insensitively) as the base theme', () => {
    mockUseGetThemes.mockReturnValue({
      data: {
        themes: [
          {id: 'modern-id', displayName: 'Modern'},
          {id: 'classic-id', displayName: 'CLASSIC'},
        ],
      },
    });

    render(<ThemeCreatePage />);

    expect(mockUseGetTheme).toHaveBeenCalledWith('classic-id');
  });

  it('falls back to the first theme in the list when none is named "Classic"', () => {
    mockUseGetThemes.mockReturnValue({
      data: {
        themes: [
          {id: 'modern-id', displayName: 'Modern'},
          {id: 'bold-id', displayName: 'Bold'},
        ],
      },
    });

    render(<ThemeCreatePage />);

    expect(mockUseGetTheme).toHaveBeenCalledWith('modern-id');
  });

  it('creates the theme in the selected project', async () => {
    const user = userEvent.setup();
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <ThemeCreatePage />
      </ProjectScopeProvider>,
    );

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByText('Set Name'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({projectId: 'project-1'}), expect.any(Object));
  });

  it('creates the theme in no project outside one', async () => {
    const user = userEvent.setup();
    render(<ThemeCreatePage />);

    await user.click(screen.getByText('Set Ready'));
    await user.click(screen.getByText('Set Name'));
    await user.click(screen.getByRole('button', {name: /Continue/i}));
    await user.click(screen.getByRole('button', {name: /Create Theme/i}));

    expect(mockMutate.mock.calls[0][0]).not.toHaveProperty('projectId');
  });
});
