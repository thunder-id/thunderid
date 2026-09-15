// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {BaseInviteUserRenderProps} from '@thunderid/react';
import {act, render, screen, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import RouteConfig from '../../../../configs/RouteConfig';
import type {AgentOnboardingFlowResolution} from '../../utils/resolveAgentOnboardingFlow';
import {AgentOnboardingFlowProblem} from '../../utils/resolveAgentOnboardingFlow';
import AgentOnboardPage from '../AgentOnboardPage';

/** The props the page hands the flow driver, captured so a test can drive the page through them. */
interface CapturedFlowProps {
  children: (renderProps: BaseInviteUserRenderProps) => ReactNode;
  onFlowChange: (response: {flowStatus: string}) => void;
  onInitialize: (payload: Record<string, unknown>) => Promise<unknown>;
}

// Shared across the module mocks below. vi.mock is hoisted above the imports, so anything its
// factories touch has to be hoisted with it.
const h = vi.hoisted(() => ({
  flowProps: {current: null as CapturedFlowProps | null},
  http: {request: vi.fn<(config: unknown) => Promise<{data: unknown}>>()},
  logger: {debug: vi.fn(), error: vi.fn(), info: vi.fn(), warn: vi.fn()},
  navigate: vi.fn(),
  pathname: {current: '/agents/create'},
  resetFlow: vi.fn(),
  resolve: vi.fn<(http: unknown, serverUrl: string) => Promise<AgentOnboardingFlowResolution>>(),
  users: {current: [] as {id: string; display?: string; attributes?: Record<string, unknown>}[]},
  step: {
    additionalData: {} as Record<string, unknown>,
    components: [] as Record<string, unknown>[],
    values: {} as Record<string, string>,
  },
}));

// Only the flow driver and the HTTP client are replaced. Everything the page renders through,
// FlowComponentRenderer included, stays real, because what these tests are checking is that the
// page draws a flow step correctly rather than that it calls a renderer.
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  BaseInviteUser: (props: CapturedFlowProps): ReactNode => {
    h.flowProps.current = props;

    return props.children({
      additionalData: h.step.additionalData,
      components: h.step.components,
      fieldErrors: {},
      handleInputBlur: vi.fn(),
      handleInputChange: vi.fn(),
      handleSubmit: vi.fn(),
      isLoading: false,
      resetFlow: h.resetFlow,
      touched: {},
      values: h.step.values,
    } as unknown as BaseInviteUserRenderProps);
  },
  useThunderID: () => ({http: h.http}),
}));

// The resolver has its own tests; here it only decides which branch of the page renders.
vi.mock('../../utils/resolveAgentOnboardingFlow', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../utils/resolveAgentOnboardingFlow')>()),
  default: h.resolve,
}));

vi.mock('@thunderid/configure-users', () => ({
  useFlowTextResolver: () => (text?: string) => text,
  useGetUsers: () => ({data: {users: h.users.current}}),
}));

// The OU picker fetches its own tree; the page only has to hand it the value and the change handler.
vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitTreePicker: ({value, onChange}: {value: string; onChange: (id: string) => void}) => (
    <button type="button" data-testid="ou-picker" data-value={value} onClick={() => onChange('ou-child-1')}>
      organization unit picker
    </button>
  ),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => h.logger,
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useLocation: () => ({pathname: h.pathname.current}),
  useNavigate: () => h.navigate,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: string | Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'agents:listing.title': 'Agents',
        'agents:onboarding.errors.flowMissing.description': 'No administration flow carries the handle {{handle}}.',
        'agents:onboarding.errors.flowMissing.title': 'Onboarding flow not found',
        'agents:onboarding.errors.notConfigured.description': 'Configure an agent onboarding flow handle.',
        'agents:onboarding.errors.notConfigured.title': 'Onboarding flow not configured',
        'agents:onboarding.errors.unavailable.description': 'The agent onboarding flow could not be loaded.',
        'agents:onboarding.errors.unavailable.title': 'Onboarding unavailable',
        'agents:onboarding.addAnother': 'Add Another Agent',
        'agents:onboarding.selectPlaceholder': 'Select an option',
      };
      const translated = translations[key];

      if (translated === undefined) {
        return typeof options === 'string' ? options : key;
      }
      if (typeof options === 'object' && options !== null) {
        return Object.entries(options).reduce(
          (text, [name, value]) => text.replace(`{{${name}}}`, String(value)),
          translated,
        );
      }

      return translated;
    },
  }),
}));

const progressValue = (): number => Number(screen.getByRole('progressbar').getAttribute('aria-valuenow'));

/** Hands the page a step, the way the flow driver would after a server response. */
const givenStep = (
  components: Record<string, unknown>[],
  additionalData: Record<string, unknown> = {},
  values: Record<string, string> = {},
): void => {
  h.step.components = components;
  h.step.additionalData = additionalData;
  h.step.values = values;
};

/** Reports a flow status back to the page, the way the driver does on every response. */
const flowReports = async (flowStatus: string): Promise<void> => {
  await act(async () => {
    h.flowProps.current?.onFlowChange({flowStatus});
    await Promise.resolve();
  });
};

describe('AgentOnboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.flowProps.current = null;
    h.pathname.current = '/agents/create';
    h.step.components = [];
    h.step.additionalData = {};
    h.step.values = {};
    h.users.current = [];
    h.resetFlow.mockClear();
    h.resolve.mockResolvedValue({flowId: 'flow-1', handle: 'default-agent-onboarding-flow'});
    h.http.request.mockResolvedValue({data: {flowStatus: 'INCOMPLETE'}});
  });

  describe('resolving the flow', () => {
    it('should show a spinner while the flow is being resolved', () => {
      h.resolve.mockReturnValue(new Promise<AgentOnboardingFlowResolution>(() => undefined));

      render(<AgentOnboardPage />);

      // The wizard's own bar is a progressbar too, so the spinner is the one reporting no position.
      const spinner = screen.getAllByRole('progressbar').find((bar) => !bar.hasAttribute('aria-valuenow'));

      expect(spinner).toBeDefined();
    });

    // The three failures below need different remedies from an administrator, so the page has to
    // tell them apart rather than report one generic error.
    it('should report a missing configuration when no handle is set', async () => {
      h.resolve.mockResolvedValue({handle: '', problem: AgentOnboardingFlowProblem.NotConfigured});

      render(<AgentOnboardPage />);

      expect(await screen.findByText('Onboarding flow not configured')).toBeInTheDocument();
      expect(screen.getByText('Configure an agent onboarding flow handle.')).toBeInTheDocument();
    });

    it('should name the configured handle when no flow carries it', async () => {
      h.resolve.mockResolvedValue({handle: 'missing-handle', problem: AgentOnboardingFlowProblem.FlowMissing});

      render(<AgentOnboardPage />);

      expect(await screen.findByText('Onboarding flow not found')).toBeInTheDocument();
      expect(screen.getByText('No administration flow carries the handle missing-handle.')).toBeInTheDocument();
    });

    it('should report the flow as unavailable when resolution fails', async () => {
      h.resolve.mockRejectedValue(new Error('server-config unreachable'));

      render(<AgentOnboardPage />);

      expect(await screen.findByText('Onboarding unavailable')).toBeInTheDocument();
      expect(h.logger.error).toHaveBeenCalled();
    });

    // There is deliberately no non-flow path to creating an agent, so a resolved flow must never be
    // silently replaced by a built-in wizard.
    it('should not render a flow step when the flow cannot be resolved', async () => {
      h.resolve.mockResolvedValue({handle: '', problem: AgentOnboardingFlowProblem.NotConfigured});

      render(<AgentOnboardPage />);

      await screen.findByText('Onboarding flow not configured');
      expect(h.flowProps.current).toBeNull();
    });

    it('should run the resolved flow by id', async () => {
      render(<AgentOnboardPage />);

      await waitFor(() => expect(h.flowProps.current).not.toBeNull());
      await act(async () => {
        await h.flowProps.current?.onInitialize({});
      });

      const [request] = h.http.request.mock.calls[0] as [{data: unknown; method: string; url: string}];

      expect(request.method).toBe('POST');
      expect(request.url).toContain('/flow/execute');
      expect(request.data).toMatchObject({flowId: 'flow-1', verbose: true});
    });
  });

  describe('rendering a flow step', () => {
    // The page draws every component itself rather than delegating, so each type the shipped flow
    // uses needs its own case. The test tree mounts no DesignProvider, matching the console.
    it('should render text components', async () => {
      givenStep([{id: 'title', label: 'Name your agent', type: 'TEXT', variant: 'HEADING_1'}]);

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('heading', {name: 'Name your agent'})).toBeInTheDocument();
    });

    it('should render the inputs a prompt asks for', async () => {
      givenStep([
        {
          components: [
            {id: 'name_input', label: 'Agent name', ref: 'name', required: true, type: 'TEXT_INPUT'},
            {eventType: 'SUBMIT', id: 'action_agent_name', label: 'Continue', type: 'ACTION', variant: 'PRIMARY'},
          ],
          id: 'block_name',
          type: 'BLOCK',
        },
      ]);

      render(<AgentOnboardPage />);

      expect(await screen.findByLabelText(/Agent name/)).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Continue'})).toBeInTheDocument();
    });

    // The client secret is returned once, on the completing step, so the credentials screen is the
    // only place it can be read.
    it('should show the credentials the completing step returns', async () => {
      givenStep(
        [
          {id: 'client_id', label: 'Client ID', source: 'clientId', type: 'COPYABLE_TEXT'},
          {id: 'client_secret', label: 'Client secret', masked: true, source: 'clientSecret', type: 'COPYABLE_TEXT'},
        ],
        {clientId: 'agent-client-id', clientSecret: 'agent-client-secret'},
      );

      render(<AgentOnboardPage />);

      expect(await screen.findByText('agent-client-id')).toBeInTheDocument();
      expect(screen.getByDisplayValue('agent-client-secret')).toBeInTheDocument();
    });

    // A masked credential carries the value so it can be copied, but must not sit on screen in the
    // clear. Matches how ShowClientSecret presents the same secret elsewhere in the console.
    it('should mask a credential the flow marks as secret', async () => {
      givenStep(
        [{id: 'client_secret', label: 'Client secret', masked: true, source: 'clientSecret', type: 'COPYABLE_TEXT'}],
        {clientSecret: 'agent-client-secret'},
      );

      render(<AgentOnboardPage />);

      const field = await screen.findByDisplayValue('agent-client-secret');

      expect(field).toHaveAttribute('type', 'password');
      expect(field).toHaveAttribute('readonly');
      expect(screen.queryByText('agent-client-secret')).not.toBeInTheDocument();
    });

    it('should reveal a masked credential on request', async () => {
      givenStep(
        [{id: 'client_secret', label: 'Client secret', masked: true, source: 'clientSecret', type: 'COPYABLE_TEXT'}],
        {clientSecret: 'agent-client-secret'},
      );

      render(<AgentOnboardPage />);

      await screen.findByDisplayValue('agent-client-secret');
      await userEvent.click(screen.getByRole('button', {name: 'Show'}));

      expect(screen.getByDisplayValue('agent-client-secret')).toHaveAttribute('type', 'text');
    });

    // USER_SELECT carries no options: the page fetches the candidates and shows each user the way
    // the deployment names them, while the value submitted stays the identifier.
    it('should offer users by name and submit the identifier', async () => {
      h.users.current = [
        {display: 'Ada Lovelace', id: 'user-1'},
        {attributes: {username: 'grace'}, id: 'user-2'},
      ];
      givenStep([{id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}]);

      render(<AgentOnboardPage />);

      expect(await screen.findByText('Owner')).toBeInTheDocument();
      await userEvent.click(screen.getByRole('combobox'));

      const options = await screen.findAllByRole('option');

      expect(options.map((option) => option.textContent)).toEqual(['Select an option', 'Ada Lovelace', 'grace']);
      expect(options[1]).toHaveAttribute('data-value', 'user-1');
      expect(options[2]).toHaveAttribute('data-value', 'user-2');
    });

    // OU_SELECT is the same contract: the picker sources its own tree, so the page only wires it up.
    it('should render the organization unit picker', async () => {
      givenStep([{id: 'ou_selection_input', label: 'Organization Unit', ref: 'ouId', type: 'OU_SELECT'}]);

      render(<AgentOnboardPage />);

      expect(await screen.findByTestId('ou-picker')).toBeInTheDocument();
    });
  });

  // A flow that fails returns no components, so without this the screen simply goes blank and the
  // only trace is a log line.
  // A step that posts while missing something it asked for is refused by the engine, so the screen
  // silently does not move. Gating the action is what makes the requirement visible.
  describe('required inputs', () => {
    const nameStep = (extra: Record<string, unknown> = {}) =>
      givenStep([
        {
          components: [
            {id: 'name_input', label: 'Agent name', ref: 'name', required: true, type: 'TEXT_INPUT', ...extra},
            {eventType: 'SUBMIT', id: 'action_agent_name', label: 'Continue', type: 'ACTION'},
          ],
          id: 'block_name',
          type: 'BLOCK',
        },
      ]);

    it('should not allow submitting while a required input is empty', async () => {
      nameStep();

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('button', {name: 'Continue'})).toBeDisabled();
    });

    it('should allow submitting once the required input is filled', async () => {
      nameStep();
      h.step.values = {name: 'billing-agent'};

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('button', {name: 'Continue'})).toBeEnabled();
    });

    // Every attribute on the detail step is optional, so it must stay submittable when untouched.
    it('should allow submitting a step whose inputs are all optional', async () => {
      givenStep([
        {
          components: [
            {id: 'model_input', label: 'Model', ref: 'model', required: false, type: 'TEXT_INPUT'},
            {eventType: 'SUBMIT', id: 'action_agent_details', label: 'Create Agent', type: 'ACTION'},
          ],
          id: 'block_details',
          type: 'BLOCK',
        },
      ]);

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('button', {name: 'Create Agent'})).toBeEnabled();
    });
  });

  // A step with nothing to submit is the end of the run. Leaving the header X as the only exit
  // strands the administrator on the screen they were told to copy a secret from.
  describe('finishing', () => {
    const credentialsStep = () =>
      givenStep([{id: 'client_id', label: 'Client ID', source: 'clientId', type: 'COPYABLE_TEXT'}], {
        clientId: 'agent-client-id',
      });

    it('should offer a way out of a step with nothing to submit', async () => {
      credentialsStep();

      render(<AgentOnboardPage />);

      // The header carries its own Close, so the step has to be on screen before asserting.
      await screen.findByText('agent-client-id');

      expect(screen.getByRole('button', {name: 'Add Another Agent'})).toBeInTheDocument();
      expect(screen.getAllByRole('button', {name: 'Close'}).length).toBeGreaterThan(1);
    });

    it('should return to the agent list when the terminal step is closed', async () => {
      credentialsStep();

      render(<AgentOnboardPage />);

      await screen.findByText('agent-client-id');
      // The header carries one Close and the step adds its own, so clicking the step's requires
      // both to be present; otherwise this would pass by clicking the header's.
      const closeButtons = screen.getAllByRole('button', {name: 'Close'});

      expect(closeButtons).toHaveLength(2);
      await userEvent.click(closeButtons[1]);

      expect(h.navigate).toHaveBeenCalledWith(RouteConfig.agents.list());
    });

    it('should restart the flow rather than navigate when another agent is wanted', async () => {
      credentialsStep();

      render(<AgentOnboardPage />);

      await screen.findByText('agent-client-id');
      await userEvent.click(screen.getByRole('button', {name: 'Add Another Agent'}));

      expect(h.resetFlow).toHaveBeenCalled();
      expect(h.navigate).not.toHaveBeenCalled();
    });

    // A step still collecting input has its own submit action, so a second set of controls would
    // compete with it.
    it('should not offer them while a step is still collecting input', async () => {
      givenStep([
        {
          components: [
            {id: 'name_input', label: 'Agent name', ref: 'name', required: true, type: 'TEXT_INPUT'},
            {eventType: 'SUBMIT', id: 'action_agent_name', label: 'Continue', type: 'ACTION'},
          ],
          id: 'block_name',
          type: 'BLOCK',
        },
      ]);

      render(<AgentOnboardPage />);

      await screen.findByLabelText(/Agent name/);
      expect(screen.queryByRole('button', {name: 'Add Another Agent'})).not.toBeInTheDocument();
    });
  });

  describe('flow failures', () => {
    it('should surface an error the flow reports', async () => {
      render(<AgentOnboardPage />);

      await waitFor(() => expect(h.flowProps.current).not.toBeNull());
      await act(async () => {
        h.flowProps.current?.onFlowChange({
          error: {message: {defaultValue: 'The agent already exists'}},
          flowStatus: 'ERROR',
        } as never);
        await Promise.resolve();
      });

      expect(await screen.findByText('The agent already exists')).toBeInTheDocument();
    });

    it('should clear the error once a later step succeeds', async () => {
      render(<AgentOnboardPage />);

      await waitFor(() => expect(h.flowProps.current).not.toBeNull());
      await act(async () => {
        h.flowProps.current?.onFlowChange({
          error: {message: {defaultValue: 'The agent already exists'}},
          flowStatus: 'ERROR',
        } as never);
        await Promise.resolve();
      });
      await flowReports('INCOMPLETE');

      expect(screen.queryByText('The agent already exists')).not.toBeInTheDocument();
    });
  });

  describe('progress', () => {
    // The flow owns the number of screens, so the page cannot divide by a total. The bar approaches
    // the end instead of claiming a position it cannot know.
    it('should advance towards, but not reach, the end as the flow reports steps', async () => {
      render(<AgentOnboardPage />);

      await waitFor(() => expect(h.flowProps.current).not.toBeNull());
      expect(progressValue()).toBe(0);

      await flowReports('INCOMPLETE');
      const afterFirst = progressValue();

      await flowReports('INCOMPLETE');
      const afterSecond = progressValue();

      expect(afterFirst).toBeGreaterThan(0);
      expect(afterSecond).toBeGreaterThan(afterFirst);
      expect(afterSecond).toBeLessThan(100);
    });

    it('should fill the bar only once the flow completes', async () => {
      render(<AgentOnboardPage />);

      await waitFor(() => expect(h.flowProps.current).not.toBeNull());
      await flowReports('INCOMPLETE');
      await flowReports('COMPLETE');

      expect(progressValue()).toBe(100);
    });
  });

  describe('closing', () => {
    it('should return to the agent list', async () => {
      render(<AgentOnboardPage />);

      (await screen.findByRole('button', {name: 'Close'})).click();

      expect(h.navigate).toHaveBeenCalledWith(RouteConfig.agents.list());
    });

    // The same page is mounted inside the welcome experience, where the agent list is not where the
    // administrator came from.
    it('should return to the get started page when opened from the welcome flow', async () => {
      h.pathname.current = RouteConfig.welcome.getStartedAgentsCreate();

      render(<AgentOnboardPage />);

      (await screen.findByRole('button', {name: 'Close'})).click();

      expect(h.navigate).toHaveBeenCalledWith(RouteConfig.welcome.getStarted());
    });
  });
});
