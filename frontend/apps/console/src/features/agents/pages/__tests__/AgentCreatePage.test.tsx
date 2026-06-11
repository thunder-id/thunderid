/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access, react/require-default-props */
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

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: (id?: string) => mockUseGetAgentType(id),
}));

vi.mock('@thunderid/configure-organization-units', () => ({
  useGetChildOrganizationUnits: (ouId: string | undefined, opts: unknown) =>
    mockUseGetChildOrganizationUnits(ouId, opts),
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

vi.mock('../../components/create-agent/ShowClientSecret', () => ({
  default: ({clientSecret, onContinue}: {clientSecret: string; onContinue: () => void}) => (
    <div data-testid="step-complete">
      <span data-testid="complete-secret">{clientSecret}</span>
      <button type="button" onClick={onContinue}>
        Continue Done
      </button>
    </div>
  ),
}));

vi.mock('@thunderid/configure-users', () => ({
  ConfigureOrganizationUnit: ({onReadyChange}: {onReadyChange?: (isReady: boolean) => void}) => (
    <div data-testid="step-organization-unit">
      <button type="button" onClick={() => onReadyChange?.(true)}>
        OU Ready
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
      }),
      expect.any(Object),
    );
  });

  it('shows the complete screen with the new client secret on success', async () => {
    const user = userEvent.setup();
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;

    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'agent-1',
        ouId: 'ou-1',
        type: 'default',
        name: 'My Agent',
        inboundAuthConfig: [
          {type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: [], clientSecret: 'shh'}},
        ],
      });
    });

    render(<AgentCreatePage />);

    await user.click(screen.getByRole('button', {name: /Create agent/i}));

    await waitFor(() => {
      expect(screen.getByTestId('step-complete')).toBeInTheDocument();
      expect(screen.getByTestId('complete-secret')).toHaveTextContent('shh');
    });
  });

  it('navigates to the agent details page from the complete screen', async () => {
    const user = userEvent.setup();
    agentCreateState.currentStep = AgentCreateFlowStep.OWNER;

    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'agent-1',
        ouId: 'ou-1',
        type: 'default',
        name: 'My Agent',
        inboundAuthConfig: [
          {type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: [], clientSecret: 'shh'}},
        ],
      });
    });

    render(<AgentCreatePage />);

    await user.click(screen.getByRole('button', {name: /Create agent/i}));

    await waitFor(() => {
      expect(screen.getByTestId('step-complete')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Continue Done'));

    expect(mockNavigate).toHaveBeenCalledWith('/agents/agent-1');
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

    expect(setError).toHaveBeenCalledWith('Create failed');
  });

  it('renders the OU step when child organization units exist', () => {
    mockUseGetChildOrganizationUnits.mockReturnValue({
      data: {totalResults: 3},
      isLoading: false,
      error: null,
    });
    agentCreateState.currentStep = AgentCreateFlowStep.ORGANIZATION_UNIT;

    render(<AgentCreatePage />);

    expect(screen.getByTestId('step-organization-unit')).toBeInTheDocument();
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
