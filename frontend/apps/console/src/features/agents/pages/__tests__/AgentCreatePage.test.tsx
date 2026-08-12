// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access */
import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {AgentCreateFlowStep} from '../../models/agent-create-flow';
import AgentCreatePage from '../AgentCreatePage';

const {
  mockNavigate,
  mockUseGetAgentTypes,
  mockUseGetAgentType,
  mockUseGetChildOrganizationUnits,
  mockMutate,
  mockUseAgentCreate,
} = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  mockUseGetAgentTypes: vi.fn(),
  mockUseGetAgentType: vi.fn(),
  mockUseGetChildOrganizationUnits: vi.fn(),
  mockMutate: vi.fn(),
  mockUseAgentCreate: vi.fn(),
}));

let mockPathname = '/agents/create';

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({pathname: mockPathname}),
  };
});

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: (id?: string) => mockUseGetAgentType(id),
}));

vi.mock('@thunderid/configure-organization-units', () => ({
  useGetChildOrganizationUnits: (ouId: string | undefined, opts: unknown) =>
    mockUseGetChildOrganizationUnits(ouId, opts),
  useGetOrganizationUnit: (id?: string) => ({
    data: id ? {id, name: 'Test Organization Unit'} : undefined,
    isLoading: false,
  }),
  OrganizationUnitPickerScreen: ({
    onChange,
    onContinue,
    onBack,
  }: {
    onChange: (id: string) => void;
    onContinue: () => void;
    onBack: () => void;
  }) => (
    <div data-testid="step-organization-unit">
      <button type="button" onClick={() => onChange('ou-2')}>
        Select OU
      </button>
      <button type="button" onClick={onContinue}>
        OU Continue
      </button>
      <button type="button" onClick={onBack}>
        OU Back
      </button>
    </div>
  ),
}));

vi.mock('../../api/useCreateAgent', () => ({
  default: () => ({
    mutate: mockMutate,
    isPending: false,
  }),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({user: {id: 'current-user', ouId: 'token-ou'}}),
}));

vi.mock('../../contexts/AgentCreate/useAgentCreate', () => ({
  default: () => mockUseAgentCreate(),
}));

vi.mock('../../components/create-agent/ConfigureName', () => ({
  default: ({
    onAgentNameChange,
    onReadyChange,
  }: {
    onAgentNameChange: (name: string) => void;
    onReadyChange?: (isReady: boolean) => void;
  }) => (
    <div data-testid="step-name">
      <button type="button" onClick={() => onAgentNameChange('My Agent')}>
        Set Name
      </button>
      <button type="button" onClick={() => onReadyChange?.(true)}>
        Set Ready
      </button>
    </div>
  ),
}));

vi.mock('../../components/create-agent/ConfigureAgentDetails', () => ({
  default: () => <div data-testid="step-profile" />,
}));

vi.mock('../../components/create-agent/ConfigureOwner', () => ({
  default: ({onReadyChange}: {onReadyChange?: (isReady: boolean) => void}) => (
    <div data-testid="step-owner">
      <button type="button" onClick={() => onReadyChange?.(true)}>
        Owner Ready
      </button>
    </div>
  ),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | {defaultValue?: string}) => {
      if (typeof fallback === 'string') return fallback || key;
      if (fallback && typeof fallback === 'object') return fallback.defaultValue ?? key;
      return key;
    },
  }),
}));

describe('AgentCreatePage', () => {
  let agentCreateState: {
    currentStep: AgentCreateFlowStep;
    selectedSchema: {id: string; name: string; ouId: string} | null;
    selectedOuId: string | null;
    agentName: string;
    formValues: Record<string, unknown>;
    selectedOwnerId: string | null;
    error: string | null;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockPathname = '/agents/create';
    agentCreateState = {
      currentStep: AgentCreateFlowStep.NAME,
      selectedSchema: {id: 'schema-1', name: 'default', ouId: 'ou-1'},
      selectedOuId: null,
      agentName: 'My Agent',
      formValues: {},
      selectedOwnerId: 'user-1',
      error: null,
    };

    mockUseAgentCreate.mockImplementation(() => ({
      ...agentCreateState,
      setCurrentStep: (step: AgentCreateFlowStep) => {
        agentCreateState.currentStep = step;
      },
      setSelectedSchema: (schema: {id: string; name: string; ouId: string} | null) => {
        agentCreateState.selectedSchema = schema;
      },
      setSelectedOuId: (id: string | null) => {
        agentCreateState.selectedOuId = id;
      },
      setAgentName: (name: string) => {
        agentCreateState.agentName = name;
      },
      setFormValues: (values: Record<string, unknown>) => {
        agentCreateState.formValues = values;
      },
      setSelectedOwnerId: (id: string | null) => {
        agentCreateState.selectedOwnerId = id;
      },
      setError: (err: string | null) => {
        agentCreateState.error = err;
      },
    }));

    mockUseGetAgentTypes.mockReturnValue({
      data: {types: [{id: 'schema-1', name: 'default', ouId: 'ou-1'}]},
    });

    mockUseGetAgentType.mockReturnValue({
      data: {id: 'schema-1', name: 'default', ouId: 'ou-1', schema: {}},
      isLoading: false,
    });

    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {totalResults: 0},
      isLoading: false,
      error: null,
    });
  });

  it('renders the name step by default', () => {
    render(<AgentCreatePage />);

    expect(screen.getByTestId('step-name')).toBeInTheDocument();
  });

  it('navigates back to /agents when close button is clicked', async () => {
    const user = userEvent.setup();
    render(<AgentCreatePage />);

    const closeButton = screen.getAllByRole('button')[0];
    await user.click(closeButton);

    expect(mockNavigate).toHaveBeenCalledWith('/agents');
  });

  it('renders the Agents breadcrumb outside the welcome flow', () => {
    render(<AgentCreatePage />);

    expect(screen.getByText('Agents')).toBeInTheDocument();
  });

  describe('welcome flow', () => {
    beforeEach(() => {
      mockPathname = '/welcome/get-started/agents/create';
    });

    it('renders the welcome breadcrumb trail', () => {
      render(<AgentCreatePage />);

      expect(screen.getByText('common:welcome.header')).toBeInTheDocument();
      expect(screen.getByText('common:welcome.createProject.breadcrumb')).toBeInTheDocument();
      expect(screen.getByText('common:welcome.getStarted.breadcrumb')).toBeInTheDocument();
      expect(screen.queryByText('Agents')).not.toBeInTheDocument();
    });

    it('navigates to the Get Started page from the breadcrumb', async () => {
      const user = userEvent.setup();
      render(<AgentCreatePage />);

      await user.click(screen.getByText('common:welcome.getStarted.breadcrumb'));

      expect(mockNavigate).toHaveBeenCalledWith('/welcome/get-started');
    });

    it('navigates to /home when close button is clicked', async () => {
      const user = userEvent.setup();
      render(<AgentCreatePage />);

      const closeButton = screen.getAllByRole('button')[0];
      await user.click(closeButton);

      expect(mockNavigate).toHaveBeenCalledWith('/home');
    });

    it('goes back to the Get Started page from the organization unit step', async () => {
      const user = userEvent.setup();
      agentCreateState.currentStep = AgentCreateFlowStep.ORGANIZATION_UNIT;
      mockUseGetChildOrganizationUnits.mockReturnValue({
        data: {totalResults: 2},
        isLoading: false,
        error: null,
      });

      render(<AgentCreatePage />);

      await user.click(screen.getByRole('button', {name: 'OU Back'}));

      expect(mockNavigate).toHaveBeenCalledWith('/welcome/get-started');
    });
  });

  it('disables the continue button until the step reports ready', () => {
    render(<AgentCreatePage />);

    const continueButton = screen.getByRole('button', {name: /continue/i});
    expect(continueButton).toBeDisabled();
  });

  it('triggers create on the last step when Create agent is clicked', async () => {
    const user = userEvent.setup();
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;
    render(<AgentCreatePage />);

    const createButton = screen.getByRole('button', {name: /Create agent/i});
    await user.click(createButton);

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        ouId: 'ou-1',
        type: 'default',
        name: 'My Agent',
        owner: 'user-1',
        inboundAuthConfig: [
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({
              grantTypes: ['client_credentials'],
              pkceRequired: false,
            }) as Record<string, unknown>,
          }),
        ],
      }),
      expect.any(Object),
    );
  });

  it('navigates to the agent details page with justCreatedSecret state on success', async () => {
    const user = userEvent.setup();
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;

    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'agent-1',
        ouId: 'ou-1',
        type: 'default',
        name: 'My Agent',
        inboundAuthConfig: [
          {
            type: 'oauth2',
            config: {
              grantTypes: ['client_credentials'],
              responseTypes: [],
              clientId: 'agent-client-id',
              clientSecret: 'shh',
            },
          },
        ],
      });
    });

    render(<AgentCreatePage />);

    await user.click(screen.getByRole('button', {name: /Create agent/i}));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/agents/agent-1', {
        state: {
          justCreatedSecret: {
            agentName: 'My Agent',
            clientId: 'agent-client-id',
            clientSecret: 'shh',
          },
        },
      });
    });
  });

  it('navigates to the agent details page without navigation state when no client secret is returned', async () => {
    const user = userEvent.setup();
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;

    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'agent-2',
        ouId: 'ou-1',
        type: 'default',
        name: 'My Agent',
        inboundAuthConfig: [],
      });
    });

    render(<AgentCreatePage />);

    await user.click(screen.getByRole('button', {name: /Create agent/i}));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/agents/agent-2');
    });
  });

  it('triggers an error path when create fails', async () => {
    const user = userEvent.setup();
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;
    const setError = vi.fn();

    mockUseAgentCreate.mockImplementation(() => ({
      ...agentCreateState,
      setCurrentStep: () => null,
      setSelectedSchema: () => null,
      setSelectedOuId: () => null,
      setAgentName: () => null,
      setFormValues: () => null,
      setSelectedOwnerId: () => null,
      setError,
    }));

    mockMutate.mockImplementation((_data, opts) => {
      opts.onError(new Error('Create failed'));
    });

    render(<AgentCreatePage />);

    await user.click(screen.getByRole('button', {name: /Create agent/i}));

    expect(setError).toHaveBeenCalledWith('Failed to create agent. Please try again.');
  });

  it('clears a stale create error as soon as a field changes', async () => {
    const user = userEvent.setup();
    agentCreateState.currentStep = AgentCreateFlowStep.NAME;
    agentCreateState.error = 'Failed to create agent. Please try again.';
    const setError = vi.fn();

    mockUseAgentCreate.mockImplementation(() => ({
      ...agentCreateState,
      setCurrentStep: () => null,
      setSelectedSchema: () => null,
      setSelectedOuId: () => null,
      setAgentName: () => null,
      setFormValues: () => null,
      setSelectedOwnerId: () => null,
      setError,
    }));

    render(<AgentCreatePage />);

    await user.click(screen.getByRole('button', {name: 'Set Name'}));

    expect(setError).toHaveBeenCalledWith(null);
  });

  it('renders the OU picker on the organization unit step when child organization units exist', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {totalResults: 3},
      isLoading: false,
      error: null,
    });
    agentCreateState.currentStep = AgentCreateFlowStep.ORGANIZATION_UNIT;

    render(<AgentCreatePage />);

    expect(screen.getByTestId('step-organization-unit')).toBeInTheDocument();
    expect(screen.queryByTestId('step-name')).not.toBeInTheDocument();
  });

  it('renders the name step directly when currentStep is NAME, even with child organization units', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {totalResults: 3},
      isLoading: false,
      error: null,
    });

    render(<AgentCreatePage />);

    expect(screen.getByTestId('step-name')).toBeInTheDocument();
    expect(screen.queryByTestId('step-organization-unit')).not.toBeInTheDocument();
  });

  it('shows a loading indicator on the organization unit step while child OUs are loading', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    });
    agentCreateState.currentStep = AgentCreateFlowStep.ORGANIZATION_UNIT;

    render(<AgentCreatePage />);

    expect(screen.queryByTestId('step-organization-unit')).not.toBeInTheDocument();
    expect(screen.queryByTestId('step-name')).not.toBeInTheDocument();
  });

  it('renders the profile step when schema has fields', () => {
    mockUseGetAgentType.mockReturnValue({
      data: {id: 'schema-1', name: 'default', ouId: 'ou-1', schema: {email: {type: 'string'}}},
      isLoading: false,
    });
    agentCreateState.currentStep = AgentCreateFlowStep.PROFILE;

    render(<AgentCreatePage />);

    expect(screen.getByTestId('step-profile')).toBeInTheDocument();
  });

  it('shows a loading indicator on the profile step while schema is loading', () => {
    mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true});
    agentCreateState.currentStep = AgentCreateFlowStep.PROFILE;

    render(<AgentCreatePage />);

    // Loading text from common:status.loading
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('renders the owner step', () => {
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;

    render(<AgentCreatePage />);

    expect(screen.getByTestId('step-owner')).toBeInTheDocument();
  });

  it('shows a Back button on steps after Name', () => {
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;

    render(<AgentCreatePage />);

    expect(screen.getByRole('button', {name: /Back/i})).toBeInTheDocument();
  });

  it('does not show a Back button on the Name step', () => {
    agentCreateState.currentStep = AgentCreateFlowStep.NAME;

    render(<AgentCreatePage />);

    expect(screen.queryByRole('button', {name: /Back/i})).not.toBeInTheDocument();
  });

  it('auto-selects the default agent type when none is selected yet', () => {
    agentCreateState.selectedSchema = null;
    render(<AgentCreatePage />);
    // The provider's setSelectedSchema is called during effect; we wired it through to update state
    expect(agentCreateState.selectedSchema).toEqual({id: 'schema-1', name: 'default', ouId: 'ou-1'});
  });
});
