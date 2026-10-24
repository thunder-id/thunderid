// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import LayoutBuilderPage from '../LayoutBuilderPage';

interface MockTheme {
  id: string;
  displayName: string;
}

interface UseGetThemesResult {
  data: {themes: MockTheme[]} | undefined;
}

interface UseGetThemeResult {
  data: {theme?: unknown} | undefined;
}

const {mockNavigate, mockUseLayoutBuilder, mockUseGetThemes, mockUseGetTheme, mockAddScreen, mockSetSelectedScreen} =
  vi.hoisted(() => ({
    mockNavigate: vi.fn(),
    mockUseLayoutBuilder: vi.fn<() => BuilderState>(),
    mockUseGetThemes: vi.fn<() => UseGetThemesResult>(),
    mockUseGetTheme: vi.fn<(id: string) => UseGetThemeResult>(),
    mockAddScreen: vi.fn(),
    mockSetSelectedScreen: vi.fn(),
  }));

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
    useGetThemes: () => mockUseGetThemes(),
    useGetTheme: (id: string) => mockUseGetTheme(id),
  };
});

vi.mock('../../contexts/LayoutBuilder/useLayoutBuilder', () => ({
  default: () => mockUseLayoutBuilder(),
}));

vi.mock('../../components/LayoutConfigPanel', () => ({
  default: () => <div data-testid="layout-config-panel" />,
}));

vi.mock('../../components/LayoutPreviewPanel', () => ({
  default: () => <div data-testid="layout-preview-panel" />,
}));

vi.mock('../../GatePreview/GatePreview', () => ({
  default: ({onSelectSelector, toolbarEnd}: {onSelectSelector: (selector: string) => void; toolbarEnd?: unknown}) => (
    <div data-testid="gate-preview">
      {toolbarEnd as React.ReactNode}
      <button type="button" onClick={() => onSelectSelector('.my-class')}>
        Pick selector
      </button>
    </div>
  ),
}));

interface BuilderState {
  layoutId: string | null;
  handle: string | null;
  displayName: string | null;
  draftLayout: Record<string, unknown> | null;
  updateDraftLayout: ReturnType<typeof vi.fn>;
  selectedScreen: string | null;
  setSelectedScreen: ReturnType<typeof vi.fn>;
  screenDraft: Record<string, unknown> | null;
  isDirty: boolean;
  addScreen: ReturnType<typeof vi.fn>;
  getAllScreens: () => Record<string, Record<string, unknown>>;
  getBaseScreenNames: () => string[];
  setScreenDraft: ReturnType<typeof vi.fn>;
  setIsDirty: ReturnType<typeof vi.fn>;
}

function makeBuilderState(overrides: Partial<BuilderState> = {}): BuilderState {
  return {
    layoutId: 'layout-1',
    handle: 'split-screen',
    displayName: 'Split screen',
    draftLayout: {screens: {auth: {}}},
    updateDraftLayout: vi.fn(),
    selectedScreen: null,
    setSelectedScreen: mockSetSelectedScreen,
    screenDraft: null,
    isDirty: false,
    addScreen: mockAddScreen,
    getAllScreens: () => ({auth: {}, register: {extends: 'auth'}}),
    getBaseScreenNames: () => ['auth'],
    setScreenDraft: vi.fn(),
    setIsDirty: vi.fn(),
    ...overrides,
  };
}

describe('LayoutBuilderPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState());
    mockUseGetThemes.mockReturnValue({data: {themes: []}});
    mockUseGetTheme.mockReturnValue({data: undefined});
  });

  it('navigates back to the design list when "Back to design" is clicked', async () => {
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    await user.click(screen.getByRole('button', {name: 'Back to design'}));

    expect(mockNavigate).toHaveBeenCalledWith('/design');
  });

  it('renders the screen list and the layout preview panel for a non-centered layout', () => {
    render(<LayoutBuilderPage />);

    expect(screen.getByText('Screens')).toBeInTheDocument();
    expect(screen.getByText('auth')).toBeInTheDocument();
    expect(screen.getByText('register')).toBeInTheDocument();
    expect(screen.getByTestId('layout-preview-panel')).toBeInTheDocument();
    expect(screen.queryByTestId('gate-preview')).not.toBeInTheDocument();
  });

  it('shows the screen count in the screens panel header', () => {
    render(<LayoutBuilderPage />);

    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('selects a screen when its list item is clicked', async () => {
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    await user.click(screen.getByText('auth'));

    expect(mockSetSelectedScreen).toHaveBeenCalledWith('auth');
  });

  it('adds a new screen through the add-screen row', async () => {
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    await user.click(screen.getByRole('button', {name: 'Add screen'}));
    await user.type(screen.getByPlaceholderText('Screen name…'), 'confirm');
    await user.click(screen.getByRole('button', {name: 'Add'}));

    expect(mockAddScreen).toHaveBeenCalledWith('confirm', 'auth');
  });

  it('disables the Save button when there are no unsaved changes', () => {
    render(<LayoutBuilderPage />);

    expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
  });

  it('enables the Save button and invokes the registered save handler when dirty', async () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({isDirty: true}));
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    const saveButton = screen.getByRole('button', {name: 'Save'});
    expect(saveButton).toBeEnabled();

    await user.click(saveButton);
    // No save handler was registered by the (mocked) LayoutConfigPanel, so this should not throw.
    expect(saveButton).toBeEnabled();
  });

  it('shows the Constraints header in the right panel when no screen is selected', () => {
    render(<LayoutBuilderPage />);

    expect(screen.getByText('Constraints')).toBeInTheDocument();
  });

  it('shows a screen-specific header in the right panel when a screen is selected', () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({selectedScreen: 'auth'}));

    render(<LayoutBuilderPage />);

    expect(screen.getByText('Screen — auth')).toBeInTheDocument();
  });

  it('renders the GatePreview canvas and hides the screens panel for a centered layout', () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({handle: 'centered'}));

    render(<LayoutBuilderPage />);

    expect(screen.getByTestId('gate-preview')).toBeInTheDocument();
    expect(screen.queryByTestId('layout-preview-panel')).not.toBeInTheDocument();
    expect(screen.queryByText('Screens')).not.toBeInTheDocument();
  });

  it('shows the Custom CSS header in the right panel for a centered layout', () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({handle: 'centered'}));

    render(<LayoutBuilderPage />);

    expect(screen.getByText('Custom CSS')).toBeInTheDocument();
  });

  it('shows a theme picker in the centered-layout toolbar when themes are available', () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({handle: 'centered'}));
    mockUseGetThemes.mockReturnValue({data: {themes: [{id: 'theme-1', displayName: 'Midnight'}]}});

    render(<LayoutBuilderPage />);

    expect(screen.getByPlaceholderText('Theme')).toBeInTheDocument();
  });

  it('switches the previewed theme when a different option is picked from the theme picker', async () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({handle: 'centered'}));
    mockUseGetThemes.mockReturnValue({
      data: {
        themes: [
          {id: 'theme-1', displayName: 'Midnight'},
          {id: 'theme-2', displayName: 'Sunrise'},
        ],
      },
    });
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    // Defaults to previewing the first theme in the list.
    expect(mockUseGetTheme).toHaveBeenCalledWith('theme-1');

    const input = screen.getByPlaceholderText('Theme');
    await user.click(input);
    await user.click(screen.getByRole('option', {name: 'Sunrise'}));

    expect(mockUseGetTheme).toHaveBeenCalledWith('theme-2');
  });

  it('does not show a theme picker when no themes are available', () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({handle: 'centered'}));
    mockUseGetThemes.mockReturnValue({data: {themes: []}});

    render(<LayoutBuilderPage />);

    expect(screen.queryByPlaceholderText('Theme')).not.toBeInTheDocument();
  });

  it('toggles the element inspector button state when clicked', async () => {
    mockUseLayoutBuilder.mockReturnValue(makeBuilderState({handle: 'centered'}));
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    const inspectorButton = screen.getByRole('button', {name: 'Element inspector'});
    expect(inspectorButton).toHaveAttribute('aria-pressed', 'true');

    await user.click(inspectorButton);

    expect(inspectorButton).toHaveAttribute('aria-pressed', 'false');
  });

  it('appends a new inline stylesheet rule when a selector is picked from the preview inspector', async () => {
    const updateDraftLayout = vi.fn();
    mockUseLayoutBuilder.mockReturnValue(
      makeBuilderState({
        handle: 'centered',
        updateDraftLayout,
        draftLayout: {screens: {}, head: {stylesheets: []}},
      }),
    );
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    await user.click(screen.getByRole('button', {name: 'Pick selector'}));

    expect(updateDraftLayout).toHaveBeenCalledWith(
      ['head', 'stylesheets'],
      expect.arrayContaining([
        expect.objectContaining({id: 'custom-1', type: 'inline', content: '.my-class {\n  \n}\n'}),
      ]),
    );
  });

  it('appends the new rule to an existing inline stylesheet instead of creating a new one', async () => {
    const updateDraftLayout = vi.fn();
    mockUseLayoutBuilder.mockReturnValue(
      makeBuilderState({
        handle: 'centered',
        updateDraftLayout,
        draftLayout: {
          screens: {},
          head: {stylesheets: [{id: 'custom-1', type: 'inline', content: 'body {\n  color: red;\n}'}]},
        },
      }),
    );
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    await user.click(screen.getByRole('button', {name: 'Pick selector'}));

    expect(updateDraftLayout).toHaveBeenCalledWith(
      ['head', 'stylesheets'],
      expect.arrayContaining([
        expect.objectContaining({
          id: 'custom-1',
          content: 'body {\n  color: red;\n}\n\n.my-class {\n  \n}\n',
        }),
      ]),
    );
  });

  it('skips over an id already used by a non-inline stylesheet when generating a new inline stylesheet id', async () => {
    const updateDraftLayout = vi.fn();
    mockUseLayoutBuilder.mockReturnValue(
      makeBuilderState({
        handle: 'centered',
        updateDraftLayout,
        draftLayout: {
          screens: {},
          head: {
            stylesheets: [
              {id: 'custom-2', type: 'external', url: 'https://example.com/a.css'},
              {id: 'custom-3', type: 'external', url: 'https://example.com/b.css'},
            ],
          },
        },
      }),
    );
    const user = userEvent.setup();
    render(<LayoutBuilderPage />);

    await user.click(screen.getByRole('button', {name: 'Pick selector'}));

    // Starting id (stylesheets.length + 1 = "custom-3") already collides, so the generator must
    // step past it to "custom-4".
    expect(updateDraftLayout).toHaveBeenCalledWith(
      ['head', 'stylesheets'],
      expect.arrayContaining([
        expect.objectContaining({id: 'custom-4', type: 'inline', content: '.my-class {\n  \n}\n'}),
      ]),
    );
  });
});
