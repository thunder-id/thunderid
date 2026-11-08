// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider, ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ThemeBuilderPage from '../ThemeBuilderPage';

const {mockNavigate, mockSetActiveSection, mockSetDraftTheme, mockSetIsDirty, mockResetDraft, mockThemeBuilderState} =
  vi.hoisted(() => ({
    mockNavigate: vi.fn(),
    mockSetActiveSection: vi.fn(),
    mockSetDraftTheme: vi.fn(),
    mockSetIsDirty: vi.fn(),
    mockResetDraft: vi.fn(),
    mockThemeBuilderState: {
      themeId: 'theme-1',
      projectId: 'project-1' as string | null,
      displayName: 'Midnight',
      isReadOnly: false,
      activeSection: 'colors' as const,
      isDirty: false,
      draftTheme: {},
    },
  }));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('../../contexts/ThemeBuilder/useThemeBuilder', () => ({
  default: () => ({
    themeId: mockThemeBuilderState.themeId,
    projectId: mockThemeBuilderState.projectId,
    displayName: mockThemeBuilderState.displayName,
    isReadOnly: mockThemeBuilderState.isReadOnly,
    activeSection: mockThemeBuilderState.activeSection,
    setActiveSection: mockSetActiveSection,
    isDirty: mockThemeBuilderState.isDirty,
    draftTheme: mockThemeBuilderState.draftTheme,
    setDraftTheme: mockSetDraftTheme,
    setIsDirty: mockSetIsDirty,
    resetDraft: mockResetDraft,
  }),
}));

vi.mock('../../components/ThemeConfigPanel', () => ({
  default: ({themeId, activeSection}: {themeId: string | null; activeSection?: string}) => (
    <div data-testid="config-panel">
      {themeId}:{activeSection}
    </div>
  ),
}));

vi.mock('../../components/ThemePreviewPanel', () => ({
  default: ({themeId}: {themeId: string | null}) => <div data-testid="preview-panel">{themeId}</div>,
}));

vi.mock('../../components/themes/ThemeBuilderLeftPanel', () => ({
  default: ({onPanelToggle}: {onPanelToggle: () => void}) => (
    <div data-testid="left-panel">
      <button type="button" onClick={onPanelToggle}>
        Toggle from left panel
      </button>
    </div>
  ),
}));

vi.mock('../../components/themes/ThemeDeleteDialog', () => ({
  default: ({
    open,
    themeName,
    onClose,
    onSuccess,
  }: {
    open: boolean;
    themeName: string | null;
    onClose: () => void;
    onSuccess?: () => void;
  }) =>
    open ? (
      <div data-testid="delete-dialog">
        {themeName}
        <button type="button" onClick={onClose}>
          Close dialog
        </button>
        <button type="button" onClick={onSuccess}>
          Simulate delete success
        </button>
      </div>
    ) : null,
}));

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('ThemeBuilderPage', () => {
  beforeEach(() => {
    mockThemeBuilderState.projectId = 'project-1';
    vi.clearAllMocks();
    mockThemeBuilderState.themeId = 'theme-1';
    mockThemeBuilderState.displayName = 'Midnight';
    mockThemeBuilderState.isReadOnly = false;
    mockThemeBuilderState.activeSection = 'colors';
    mockThemeBuilderState.isDirty = false;
    mockThemeBuilderState.draftTheme = {};
  });

  describe('Layout', () => {
    it('renders the back to design button', () => {
      render(<ThemeBuilderPage />);
      expect(screen.getByRole('button', {name: /Back to Design/i})).toBeInTheDocument();
    });

    it('renders the left panel and preview panel', () => {
      render(<ThemeBuilderPage />);
      expect(screen.getByTestId('left-panel')).toBeInTheDocument();
      expect(screen.getByTestId('preview-panel')).toBeInTheDocument();
    });

    it('renders the config panel with the current theme id and active section', () => {
      render(<ThemeBuilderPage />);
      expect(screen.getByTestId('config-panel')).toHaveTextContent('theme-1:colors');
    });

    it('shows the active section label as the config panel header', () => {
      render(<ThemeBuilderPage />);
      expect(screen.getByText('Colors')).toBeInTheDocument();
    });

    it('falls back to the Config header label when there is no active section', () => {
      mockThemeBuilderState.activeSection = '' as unknown as 'colors';
      render(<ThemeBuilderPage />);
      expect(screen.getByText('Config')).toBeInTheDocument();
    });
  });

  describe('Navigation', () => {
    it('navigates back to the design list when Back to Design is clicked', async () => {
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await user.click(screen.getByRole('button', {name: /Back to Design/i}));

      expect(mockNavigate).toHaveBeenCalledWith('/design');
    });
  });

  describe('Delete action', () => {
    it('shows a Delete button when the theme is not read-only', () => {
      render(<ThemeBuilderPage />);
      expect(screen.getByRole('button', {name: 'Delete'})).toBeInTheDocument();
    });

    it('hides the Delete button when the theme is read-only', () => {
      mockThemeBuilderState.isReadOnly = true;
      render(<ThemeBuilderPage />);
      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    });

    it('hides Delete and disables Save when the admin may only view designs', () => {
      mockThemeBuilderState.isDirty = true;
      render(
        <CapabilitiesProvider capabilities={viewOnly}>
          <ThemeBuilderPage />
        </CapabilitiesProvider>,
      );

      expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('opens the delete dialog when Delete is clicked', async () => {
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await user.click(screen.getByRole('button', {name: 'Delete'}));

      expect(screen.getByTestId('delete-dialog')).toHaveTextContent('Midnight');
    });

    it('closes the delete dialog when its onClose is invoked', async () => {
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await user.click(screen.getByRole('button', {name: 'Delete'}));
      await user.click(screen.getByText('Close dialog'));

      expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
    });

    it('navigates to the design list when the delete dialog reports success', async () => {
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await user.click(screen.getByRole('button', {name: 'Delete'}));
      await user.click(screen.getByText('Simulate delete success'));

      expect(mockNavigate).toHaveBeenCalledWith('/design');
    });
  });

  describe('Revert action', () => {
    it('disables the Revert button when there are no unsaved changes', () => {
      mockThemeBuilderState.isDirty = false;
      render(<ThemeBuilderPage />);
      expect(screen.getByRole('button', {name: /Revert/i})).toBeDisabled();
    });

    it('enables the Revert button when there are unsaved changes', () => {
      mockThemeBuilderState.isDirty = true;
      render(<ThemeBuilderPage />);
      expect(screen.getByRole('button', {name: /Revert/i})).toBeEnabled();
    });

    it('calls resetDraft when Revert is clicked', async () => {
      mockThemeBuilderState.isDirty = true;
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await user.click(screen.getByRole('button', {name: /Revert/i}));

      expect(mockResetDraft).toHaveBeenCalledOnce();
    });
  });

  describe('Save action', () => {
    it('disables the Save button when there are no unsaved changes', () => {
      mockThemeBuilderState.isDirty = false;
      render(<ThemeBuilderPage />);
      expect(screen.getByRole('button', {name: /Save/i})).toBeDisabled();
    });

    it('enables the Save button when there are unsaved changes', () => {
      mockThemeBuilderState.isDirty = true;
      render(<ThemeBuilderPage />);
      expect(screen.getByRole('button', {name: /Save/i})).toBeEnabled();
    });

    it('does not throw when Save is clicked before the config panel has registered a save handler', async () => {
      mockThemeBuilderState.isDirty = true;
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await expect(user.click(screen.getByRole('button', {name: /Save/i}))).resolves.not.toThrow();
    });
  });

  describe('Panel toggle', () => {
    it('shows the expand button after the panel is collapsed from within the left panel', async () => {
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await user.click(screen.getByText('Toggle from left panel'));

      expect(screen.getByRole('button', {name: 'Show sections'})).toBeInTheDocument();
    });

    it('hides the expand button again once the panel is toggled back open', async () => {
      const user = userEvent.setup();
      render(<ThemeBuilderPage />);

      await user.click(screen.getByText('Toggle from left panel'));
      await user.click(screen.getByRole('button', {name: 'Show sections'}));

      expect(screen.queryByRole('button', {name: 'Show sections'})).not.toBeInTheDocument();
    });
  });

  describe('No theme selected', () => {
    it('renders with a null themeId without crashing', () => {
      mockThemeBuilderState.themeId = null as unknown as string;
      render(<ThemeBuilderPage />);
      expect(screen.getByTestId('config-panel')).toHaveTextContent(':colors');
    });
  });

  it('shows a theme of the selected project', () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <ThemeBuilderPage />
      </ProjectScopeProvider>,
    );

    expect(screen.getByTestId('left-panel')).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('returns to the list for a theme of another project', () => {
    mockThemeBuilderState.projectId = 'project-2';
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <ThemeBuilderPage />
      </ProjectScopeProvider>,
    );

    expect(screen.queryByTestId('left-panel')).not.toBeInTheDocument();
    expect(mockNavigate).toHaveBeenCalledWith(expect.any(String), {replace: true});
  });
});
