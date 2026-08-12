// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import type {ComponentProps, JSX} from 'react';
import {useEffect} from 'react';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type ConfigureNameComponent from '../../components/create-group/ConfigureName';
import GroupConstraints from '../../constants/group-constraints';
import GroupCreateProvider from '../../contexts/GroupCreate/GroupCreateProvider';
import CreateGroupPage from '../CreateGroupPage';

// The wizard's own gate lives in ConfigureName. Flipping this reports the step ready whatever the name
// is, which is the only way to reach the submit guards, since they exist in case that gate regresses.
const mockNameStep = {forceReady: false};
vi.mock('../../components/create-group/ConfigureName', async (importOriginal) => {
  const actual = await importOriginal<{default: typeof ConfigureNameComponent}>();

  function ConfigureNameStub({
    onReadyChange = undefined,
    ...rest
  }: ComponentProps<typeof ConfigureNameComponent>): JSX.Element {
    const {forceReady} = mockNameStep;

    useEffect((): void => {
      if (forceReady) onReadyChange?.(true);
    }, [forceReady, onReadyChange]);

    return <actual.default {...rest} onReadyChange={forceReady ? undefined : onReadyChange} />;
  }

  return {default: ConfigureNameStub};
});

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockMutateAsync = vi.fn();
vi.mock('../../api/useCreateGroup', () => ({
  default: () => ({
    mutateAsync: mockMutateAsync,
    mutate: vi.fn(),
    isPending: false,
    error: null,
  }),
}));

const mockUseHasMultipleOUs = vi.fn();
vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitTreePicker: ({value, onChange}: {value: string; onChange: (id: string) => void}) => (
    <div data-testid="ou-tree-picker">
      <span data-testid="ou-value">{value}</span>
      <button type="button" data-testid="select-ou" onClick={() => onChange('ou-123')}>
        Select OU
      </button>
    </div>
  ),
  OrganizationUnitPickerScreen: ({
    value,
    onChange,
    onBack,
    onContinue,
    backLabel,
    continueLabel,
  }: {
    value: string;
    onChange: (id: string) => void;
    onBack: () => void;
    onContinue: () => void;
    backLabel: string;
    continueLabel: string;
  }) => (
    <div data-testid="configure-organization-unit">
      <span data-testid="ou-value">{value}</span>
      <button type="button" data-testid="select-ou" onClick={() => onChange('ou-123')}>
        Select OU
      </button>
      <button type="button" onClick={onBack}>
        {backLabel}
      </button>
      <button type="button" onClick={onContinue}>
        {continueLabel}
      </button>
    </div>
  ),
  useHasMultipleOUs: (): unknown => mockUseHasMultipleOUs(),
  useGetOrganizationUnit: (id?: string) => ({
    data: id ? {id, name: 'Test Organization Unit'} : undefined,
    isLoading: false,
  }),
  OrganizationUnitTreeConstants: {
    DEFAULT_AVATAR: 'avatar:shape=rounded,variant=anonymous_entity,content=pavilion,colors=0',
  },
}));

function renderPage() {
  return renderWithProviders(
    <GroupCreateProvider>
      <CreateGroupPage />
    </GroupCreateProvider>,
  );
}

describe('CreateGroupPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNameStep.forceReady = false;
    mockNavigate.mockResolvedValue(undefined);
    mockMutateAsync.mockResolvedValue({});
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('with single OU', () => {
    beforeEach(() => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: false,
        isLoading: false,
        ouList: [{id: 'ou-single', name: 'Default OU'}],
      });
    });

    it('should render name step with suggestions', () => {
      renderPage();

      expect(screen.getByTestId('configure-name')).toBeInTheDocument();
      expect(screen.getByText("Let's collect some details about your group")).toBeInTheDocument();
    });

    it('should have disabled button initially', () => {
      renderPage();

      const button = screen.getByRole('button', {name: 'Continue'});
      expect(button).toBeDisabled();
    });

    it('should enable button when name is entered', async () => {
      const user = userEvent.setup();
      renderPage();

      const nameInput = screen.getByPlaceholderText('Enter group name');
      await user.type(nameInput, 'Test Group');

      await waitFor(() => {
        const button = screen.getByRole('button', {name: 'Continue'});
        expect(button).not.toBeDisabled();
      });
    });

    it('should submit directly without OU step when only one OU exists', async () => {
      const user = userEvent.setup();
      renderPage();

      const nameInput = screen.getByPlaceholderText('Enter group name');
      await user.type(nameInput, 'Test Group');

      await waitFor(() => {
        const button = screen.getByRole('button', {name: 'Continue'});
        expect(button).not.toBeDisabled();
      });

      await user.click(screen.getByRole('button', {name: 'Continue'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          name: 'Test Group',
          ouId: 'ou-single',
        });
      });
    });

    it('should navigate to groups list on successful creation', async () => {
      const user = userEvent.setup();
      renderPage();

      const nameInput = screen.getByPlaceholderText('Enter group name');
      await user.type(nameInput, 'Test Group');

      await waitFor(() => {
        const button = screen.getByRole('button', {name: 'Continue'});
        expect(button).not.toBeDisabled();
      });

      await user.click(screen.getByRole('button', {name: 'Continue'}));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/groups');
      });
    });
  });

  describe('with multiple OUs', () => {
    beforeEach(() => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: true,
        isLoading: false,
        ouList: [
          {id: 'ou-1', name: 'OU 1'},
          {id: 'ou-2', name: 'OU 2'},
        ],
      });
    });

    it('should render the OU picker before the name step', () => {
      renderPage();

      expect(screen.getByTestId('configure-organization-unit')).toBeInTheDocument();
      expect(screen.queryByTestId('configure-name')).not.toBeInTheDocument();
    });

    it('should navigate to the name step after picking an OU', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByTestId('select-ou'));
      await user.click(screen.getByRole('button', {name: 'Continue'}));

      await waitFor(() => {
        expect(screen.getByTestId('configure-name')).toBeInTheDocument();
      });
    });

    it('should submit with the picked OU after entering a name', async () => {
      const user = userEvent.setup();
      renderPage();

      // Step 1: Pick the OU
      await user.click(screen.getByTestId('select-ou'));
      await user.click(screen.getByRole('button', {name: 'Continue'}));

      // Step 2: Enter name
      await waitFor(() => {
        expect(screen.getByTestId('configure-name')).toBeInTheDocument();
      });

      const nameInput = screen.getByPlaceholderText('Enter group name');
      await user.type(nameInput, 'Test Group');

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'Continue'})).not.toBeDisabled();
      });

      await user.click(screen.getByRole('button', {name: 'Continue'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          name: 'Test Group',
          ouId: 'ou-123',
        });
      });
    });

    it('should navigate to the groups list from the OU picker', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByRole('button', {name: 'Back'}));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/groups');
      });
    });
  });

  it('should handle submission error gracefully', async () => {
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [{id: 'ou-single', name: 'Default OU'}],
    });
    mockMutateAsync.mockRejectedValue(new Error('Create failed'));

    const user = userEvent.setup();
    renderPage();

    const nameInput = screen.getByPlaceholderText('Enter group name');
    await user.type(nameInput, 'Test Group');

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Continue'})).not.toBeDisabled();
    });

    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalled();
    });

    // Should not navigate since submission failed
    expect(mockNavigate).not.toHaveBeenCalledWith('/groups');
  });

  it('shows and closes validation snackbar when no OU is available', async () => {
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [],
    });

    const user = userEvent.setup();
    renderPage();

    const nameInput = screen.getByPlaceholderText('Enter group name');
    await user.type(nameInput, 'Test Group');

    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Continue'})).not.toBeDisabled();
    });

    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    const closeButtons = screen.getAllByRole('button', {name: /close/i});
    await user.click(closeButtons[closeButtons.length - 1]);

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  it('should show a loading indicator while OUs are loading, without the wizard', () => {
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: true,
      ouList: [],
    });

    renderPage();

    expect(screen.queryByTestId('configure-name')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Continue'})).not.toBeInTheDocument();
  });

  it('should navigate back when close button is clicked', async () => {
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [{id: 'ou-1', name: 'OU 1'}],
    });

    const user = userEvent.setup();
    renderPage();

    const closeButton = screen.getByRole('button', {name: 'Close'});
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/groups');
    });
  });

  it('should handle navigate rejection gracefully', async () => {
    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [{id: 'ou-1', name: 'OU 1'}],
    });
    mockNavigate.mockRejectedValue(new Error('Nav failed'));

    const user = userEvent.setup();
    renderPage();

    const closeButton = screen.getByRole('button', {name: 'Close'});
    await user.click(closeButton);

    // Should not throw - error is caught gracefully
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/groups');
    });
  });

  describe('submit guards', () => {
    const submitWithName = async (value: string): Promise<void> => {
      const user = userEvent.setup();
      renderPage();

      fireEvent.change(screen.getByPlaceholderText('Enter group name'), {target: {value}});

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'Continue'})).not.toBeDisabled();
      });

      await user.click(screen.getByRole('button', {name: 'Continue'}));
    };

    const expectSubmitRefused = async (message: string): Promise<void> => {
      await waitFor(() => {
        const alerts = screen.getAllByRole('alert');
        expect(alerts.some((alert) => alert.textContent?.includes(message))).toBe(true);
      });
      expect(mockMutateAsync).not.toHaveBeenCalled();
    };

    beforeEach(() => {
      mockNameStep.forceReady = true;
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: false,
        isLoading: false,
        ouList: [{id: 'ou-single', name: 'Default OU'}],
      });
    });

    it('should refuse a name that is only whitespace', async () => {
      await submitWithName('   ');

      await expectSubmitRefused('Group name is required');
    });

    it('should refuse a name longer than the maximum length', async () => {
      await submitWithName('a'.repeat(GroupConstraints.NAME_MAX_LENGTH + 1));

      await expectSubmitRefused(`Group name cannot exceed ${GroupConstraints.NAME_MAX_LENGTH} characters`);
    });
  });
});
