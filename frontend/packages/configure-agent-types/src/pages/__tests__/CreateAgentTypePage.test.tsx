// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type useCreateAgentTypeHook from '../../api/useCreateAgentType';
import AgentTypeCreateProvider from '../../contexts/AgentTypeCreate/AgentTypeCreateProvider';
import CreateAgentTypePage from '../CreateAgentTypePage';

const mockNavigate = vi.fn();
const mockMutateAsync = vi.fn();
const mockReset = vi.fn();

const {ouState, mockUseGetAllAgentTypes} = vi.hoisted(() => ({
  ouState: {hasMultipleOUs: true},
  mockUseGetAllAgentTypes: vi.fn(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockUseCreateAgentType = vi.fn<() => ReturnType<typeof useCreateAgentTypeHook>>();

vi.mock('../../api/useCreateAgentType', () => ({
  default: () => mockUseCreateAgentType(),
}));

vi.mock('../../api/useGetAllAgentTypes', () => ({
  default: (): unknown => mockUseGetAllAgentTypes() as unknown,
}));

vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  useHasMultipleOUs: () => ({
    hasMultipleOUs: ouState.hasMultipleOUs,
    isLoading: false,
    ouList: ouState.hasMultipleOUs
      ? [
          {id: 'root-ou', name: 'Root Organization', handle: 'root', description: null, parent: null},
          {id: 'child-ou', name: 'Child Organization', handle: 'child', description: null, parent: 'root-ou'},
        ]
      : [{id: 'only-ou', name: 'Only Organization', handle: 'only', description: null, parent: null}],
  }),
  useGetOrganizationUnit: (id?: string) => ({
    data: id ? {id, name: id === 'root-ou' ? 'Root Organization' : id} : undefined,
    isLoading: false,
  }),
  OrganizationUnitTreeConstants: {
    DEFAULT_AVATAR: 'avatar:shape=rounded,variant=anonymous_entity,content=pavilion,colors=0',
  },
  OrganizationUnitPickerScreen: ({
    title,
    value,
    onChange,
    onBack,
    onContinue,
    backLabel,
    continueLabel,
    error = undefined,
    errorAction = undefined,
    autoSelectFirst = true,
  }: {
    title: string;
    value: string;
    onChange: (id: string) => void;
    onBack: () => void;
    onContinue: () => void;
    backLabel: string;
    continueLabel: string;
    error?: string;
    errorAction?: ReactNode;
    autoSelectFirst?: boolean;
  }) => (
    <div data-testid="organization-unit-picker-screen" data-auto-select-first={String(autoSelectFirst)}>
      <h1>{title}</h1>
      <span data-testid="ou-value">{value}</span>
      <button type="button" onClick={() => onChange('root-ou')}>
        Select root
      </button>
      <button type="button" onClick={() => onChange('child-ou')}>
        Select child
      </button>
      <button type="button" onClick={onBack}>
        {backLabel}
      </button>
      {error && <div role="alert">{error}</div>}
      {error && errorAction}
      <button type="button" onClick={onContinue} disabled={!value || Boolean(error)}>
        {continueLabel}
      </button>
    </div>
  ),
}));

vi.mock('@thunderid/utils', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/utils')>();
  return {
    ...actual,
    generateRandomHumanReadableIdentifiers: () => ['Swift Falcon'],
  };
});

const apiError = (code: string): Error => Object.assign(new Error('Request failed'), {response: {data: {code}}});

const mockMutation = (overrides: Partial<ReturnType<typeof useCreateAgentTypeHook>> = {}): void => {
  mockUseCreateAgentType.mockReturnValue({
    mutateAsync: mockMutateAsync,
    isPending: false,
    error: null,
    isError: false,
    reset: mockReset,
    ...overrides,
  } as unknown as ReturnType<typeof useCreateAgentTypeHook>);
};

function renderPage() {
  return render(
    <AgentTypeCreateProvider>
      <CreateAgentTypePage />
    </AgentTypeCreateProvider>,
  );
}

async function pickOrganizationUnit(user: ReturnType<typeof userEvent.setup>, ouLabel = 'Select root') {
  await screen.findByTestId('organization-unit-picker-screen');
  await user.click(screen.getByRole('button', {name: ouLabel}));
  await user.click(screen.getByRole('button', {name: 'Continue'}));
  await screen.findByTestId('configure-name');
}

async function goToPropertiesStep(user: ReturnType<typeof userEvent.setup>, name = 'Research Assistant') {
  await pickOrganizationUnit(user);
  await user.type(screen.getByLabelText(/Agent type name/), name);
  await user.click(screen.getByRole('button', {name: 'Continue'}));
  await screen.findByTestId('configure-properties');
}

async function addProperty(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(screen.getByRole('button', {name: 'Add property'}));
  const inputs = screen.getAllByLabelText('Property name');
  await user.type(inputs[inputs.length - 1], name);
}

describe('CreateAgentTypePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    ouState.hasMultipleOUs = true;
    mockNavigate.mockResolvedValue(undefined);
    mockMutateAsync.mockResolvedValue({id: 'new-type'});
    mockMutation();
    mockUseGetAllAgentTypes.mockReturnValue({
      data: [{id: 'existing', handle: 'default', displayName: 'Default', ouId: 'child-ou'}],
      isLoading: false,
    });
  });

  describe('organization unit step', () => {
    it('should start by asking which organization unit owns the agent type', () => {
      renderPage();

      expect(screen.getByRole('heading', {name: 'Where should this agent type belong?'})).toBeInTheDocument();
    });

    it('should return to the agent types list on Back', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByRole('button', {name: 'Back'}));

      expect(mockNavigate).toHaveBeenCalledWith('/agent-types');
    });

    it('should skip the step when there is only one organization unit', async () => {
      ouState.hasMultipleOUs = false;
      renderPage();

      expect(await screen.findByTestId('configure-name')).toBeInTheDocument();
      expect(screen.queryByTestId('organization-unit-picker-screen')).not.toBeInTheDocument();
    });
  });

  describe('details step', () => {
    it('should suggest a handle from the name', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.type(screen.getByLabelText(/Agent type name/), 'Research Assistant');

      expect(screen.getByLabelText(/Handle/)).toHaveValue('research-assistant');
    });

    it('should stop following the name once the handle is edited', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.type(screen.getByLabelText(/Handle/), 'custom');
      await user.type(screen.getByLabelText(/Agent type name/), 'Research');

      expect(screen.getByLabelText(/Handle/)).toHaveValue('custom');
    });

    it('should fill the name and handle from the suggestion', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.click(screen.getByRole('button', {name: 'Swift Falcon'}));

      expect(screen.getByLabelText(/Agent type name/)).toHaveValue('Swift Falcon');
      expect(screen.getByLabelText(/Handle/)).toHaveValue('swift-falcon');
    });

    it('should block Continue until a name and a valid handle are given', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();

      await user.type(screen.getByLabelText(/Agent type name/), 'Assistant');

      expect(screen.getByRole('button', {name: 'Continue'})).toBeEnabled();
    });

    it('should explain an invalid handle', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.type(screen.getByLabelText(/Agent type name/), 'Assistant');
      await user.clear(screen.getByLabelText(/Handle/));
      await user.type(screen.getByLabelText(/Handle/), 'bad-');

      expect(screen.getByText(/Handle must start and end with a lowercase letter or number/)).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
    });

    it('should show the chosen organization unit and return to it on change', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      expect(screen.getByText('Root Organization')).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: /change/i}));

      expect(await screen.findByTestId('organization-unit-picker-screen')).toBeInTheDocument();
    });

    it('should close the wizard to the agent types list', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.click(screen.getByRole('button', {name: 'Close'}));

      expect(mockNavigate).toHaveBeenCalledWith('/agent-types');
    });

    it('should refuse a handle another agent type already has', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.type(screen.getByLabelText(/Agent type name/), 'Default');

      expect(
        screen.getByText('An agent type with this handle already exists. Choose another handle.'),
      ).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
    });

    it('should accept a handle no agent type has', async () => {
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.type(screen.getByLabelText(/Agent type name/), 'Research Assistant');

      expect(
        screen.queryByText('An agent type with this handle already exists. Choose another handle.'),
      ).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Continue'})).toBeEnabled();
    });
  });

  describe('properties step', () => {
    it('should block creating until a property has a name', async () => {
      const user = userEvent.setup();
      renderPage();
      await goToPropertiesStep(user);

      expect(screen.getByRole('button', {name: 'Create agent type'})).toBeDisabled();

      await addProperty(user, 'name');

      expect(screen.getByRole('button', {name: 'Create agent type'})).toBeEnabled();
    });

    it('should create the agent type in the chosen organization unit and return to the list', async () => {
      const user = userEvent.setup();
      renderPage();
      await goToPropertiesStep(user);

      await addProperty(user, 'name');
      await user.click(screen.getByRole('button', {name: 'Create agent type'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          handle: 'research-assistant',
          displayName: 'Research Assistant',
          ouId: 'root-ou',
          systemAttributes: {display: 'name'},
          schema: {name: {type: 'string', required: false}},
        });
      });
      expect(mockNavigate).toHaveBeenCalledWith('/agent-types');
    });

    it('should create the agent type in the only organization unit when there is one', async () => {
      const user = userEvent.setup();
      ouState.hasMultipleOUs = false;
      renderPage();

      await screen.findByTestId('configure-name');
      await user.type(screen.getByLabelText(/Agent type name/), 'Assistant');
      await user.click(screen.getByRole('button', {name: 'Continue'}));
      await screen.findByTestId('configure-properties');
      await addProperty(user, 'name');
      await user.click(screen.getByRole('button', {name: 'Create agent type'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(expect.objectContaining({ouId: 'only-ou'}));
      });
    });

    it('should leave the display attribute out when it is cleared', async () => {
      const user = userEvent.setup();
      renderPage();
      await goToPropertiesStep(user);

      await addProperty(user, 'name');
      await user.click(screen.getByRole('combobox', {name: 'Display attribute'}));
      await user.click(await screen.findByRole('option', {name: 'None'}));
      await user.click(screen.getByRole('button', {name: 'Create agent type'}));

      await waitFor(() => expect(mockMutateAsync).toHaveBeenCalled());
      const [request] = mockMutateAsync.mock.calls[0] as [Record<string, unknown>];
      expect(request).not.toHaveProperty('systemAttributes');
    });

    it('should reject duplicate property names before sending the request', async () => {
      const user = userEvent.setup();
      renderPage();
      await goToPropertiesStep(user);

      await addProperty(user, 'model');
      await addProperty(user, 'model');
      await user.click(screen.getByRole('button', {name: 'Create agent type'}));

      expect(await screen.findByText('Duplicate property names found: model')).toBeInTheDocument();
      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it('should stay on the step when the request fails', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockRejectedValueOnce(apiError('USRS-1021'));
      renderPage();
      await goToPropertiesStep(user);

      await addProperty(user, 'name');
      await user.click(screen.getByRole('button', {name: 'Create agent type'}));

      await waitFor(() => expect(mockMutateAsync).toHaveBeenCalled());
      expect(mockNavigate).not.toHaveBeenCalled();
      expect(screen.getByTestId('configure-properties')).toBeInTheDocument();
    });
  });

  describe('organization unit that already has an agent type', () => {
    const takenMessage =
      'This organization unit already has the agent type Default. Choose another organization unit, or edit Default instead.';

    it('should refuse it on the organization unit step', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByRole('button', {name: 'Select child'}));

      expect(screen.getByRole('alert')).toHaveTextContent(takenMessage);
      expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
    });

    it('should not suggest editing an agent type that is read-only', async () => {
      const user = userEvent.setup();
      mockUseGetAllAgentTypes.mockReturnValue({
        data: [{id: 'existing', handle: 'default', displayName: 'Default', ouId: 'child-ou', isReadOnly: true}],
        isLoading: false,
      });
      renderPage();

      await user.click(screen.getByRole('button', {name: 'Select child'}));

      expect(screen.getByRole('alert')).toHaveTextContent(
        'This organization unit already has the agent type Default, which is read-only. Choose another organization unit.',
      );
    });

    it('should open the agent type that holds the organization unit', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByRole('button', {name: 'Select child'}));
      await user.click(screen.getByRole('button', {name: 'Open agent type'}));

      expect(mockNavigate).toHaveBeenCalledWith('/agent-types/existing');
    });

    it('should let the wizard continue once a free organization unit is chosen', async () => {
      const user = userEvent.setup();
      renderPage();

      await user.click(screen.getByRole('button', {name: 'Select child'}));
      await user.click(screen.getByRole('button', {name: 'Select root'}));

      expect(screen.queryByText(takenMessage)).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Continue'})).toBeEnabled();
    });

    // The first organization unit is usually the Default one, which holds the bootstrap agent type.
    it('should not preselect an organization unit', () => {
      renderPage();

      expect(screen.getByTestId('organization-unit-picker-screen')).toHaveAttribute('data-auto-select-first', 'false');
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('should refuse the only organization unit on the details step', async () => {
      const user = userEvent.setup();
      ouState.hasMultipleOUs = false;
      mockUseGetAllAgentTypes.mockReturnValue({
        data: [{id: 'existing', handle: 'default', displayName: 'Default', ouId: 'only-ou'}],
        isLoading: false,
      });
      renderPage();

      await screen.findByTestId('configure-name');
      await user.type(screen.getByLabelText(/Agent type name/), 'Assistant');

      expect(
        screen.getByText(
          'The only organization unit already has the agent type Default. Create another organization unit first, or edit Default instead.',
        ),
      ).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();

      await user.click(screen.getByRole('button', {name: 'Open agent type'}));

      expect(mockNavigate).toHaveBeenCalledWith('/agent-types/existing');
    });

    it('should not block an organization unit while the agent types are unknown', async () => {
      const user = userEvent.setup();
      mockUseGetAllAgentTypes.mockReturnValue({data: undefined, isLoading: true});
      renderPage();

      await user.click(screen.getByRole('button', {name: 'Select child'}));

      expect(screen.queryByText(takenMessage)).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Continue'})).toBeEnabled();
    });
  });

  describe('create failures', () => {
    it('should explain that the organization unit already has an agent type', async () => {
      mockMutation({error: apiError('USRS-1021'), isError: true} as Partial<ReturnType<typeof useCreateAgentTypeHook>>);
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      expect(
        screen.getByText(
          'This organization unit already has an agent type. Choose another organization unit, or edit the existing agent type.',
        ),
      ).toBeInTheDocument();
    });

    it('should explain that the handle is taken', async () => {
      mockMutation({error: apiError('USRS-1003'), isError: true} as Partial<ReturnType<typeof useCreateAgentTypeHook>>);
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      expect(screen.getByText('An agent type with the same handle already exists.')).toBeInTheDocument();
    });

    it('should fall back to a generic message without the server text', async () => {
      mockMutation({error: new Error('raw server text'), isError: true} as Partial<
        ReturnType<typeof useCreateAgentTypeHook>
      >);
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      expect(screen.getByText('Failed to create agent type. Please try again.')).toBeInTheDocument();
      expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    });

    it('should clear the failure once a field changes', async () => {
      mockMutation({error: apiError('USRS-1021'), isError: true} as Partial<ReturnType<typeof useCreateAgentTypeHook>>);
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      await user.type(screen.getByLabelText(/Agent type name/), 'A');

      expect(mockReset).toHaveBeenCalled();
    });

    it('should clear the failure when the organization unit changes', async () => {
      mockMutation({error: apiError('USRS-1021'), isError: true} as Partial<ReturnType<typeof useCreateAgentTypeHook>>);
      const user = userEvent.setup();
      renderPage();

      await screen.findByTestId('organization-unit-picker-screen');
      await user.click(screen.getByRole('button', {name: 'Select child'}));

      expect(mockReset).toHaveBeenCalled();
    });

    it('should disable the buttons while the request runs', async () => {
      mockMutation({isPending: true});
      const user = userEvent.setup();
      renderPage();
      await pickOrganizationUnit(user);

      expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
    });
  });
});
