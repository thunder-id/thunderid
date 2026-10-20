// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {
  toUserSelectPage,
  type BaseInviteUserRenderProps,
  type SelectPage,
  type SelectPageRequest,
} from '@thunderid/react';
import {act, render, screen, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import AgentOnboardPage from '../AgentOnboardPage';
import {defaultAgentRoutePaths} from '@/hooks/useAgentRoutes';
import {AgentOnboardingFlowProblem, type AgentOnboardingFlowResolution} from '@/utils/resolveAgentOnboardingFlow';

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
  handleSubmit: vi.fn<(action: unknown, inputs?: Record<string, string>) => Promise<void>>(),
  showToast: vi.fn<(message: string, severity?: string) => void>(),
  resolve: vi.fn<(http: unknown, serverUrl: string) => Promise<AgentOnboardingFlowResolution>>(),
  signedInUser: {current: null as {id?: string} | null},
  handleInputChange: vi.fn<(ref: string, value: string) => void>(),
  fetchUsers: vi.fn<(request: SelectPageRequest) => Promise<SelectPage>>(),
  fetchAuthFlows: vi.fn<(request: SelectPageRequest) => Promise<SelectPage>>(),
  fetchUserTypes: vi.fn<(request: SelectPageRequest) => Promise<SelectPage>>(),
  users: {current: [] as {id: string; display?: string; attributes?: Record<string, unknown>}[]},
  step: {
    additionalData: {} as Record<string, unknown>,
    components: [] as Record<string, unknown>[],
    fieldErrors: {} as Record<string, string>,
    touched: {} as Record<string, boolean>,
    values: {} as Record<string, string>,
  },
}));

// Only the flow driver, the HTTP client and the management loaders are replaced. Everything the page
// renders through, the design adapters included, stays real, because what these tests are checking is
// that the page draws a flow step correctly rather than that it calls a renderer.
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  BaseInviteUser: (props: CapturedFlowProps): ReactNode => {
    h.flowProps.current = props;

    return props.children({
      additionalData: h.step.additionalData,
      components: h.step.components,
      fieldErrors: h.step.fieldErrors,
      handleInputBlur: vi.fn(),
      handleInputChange: h.handleInputChange,
      handleSubmit: h.handleSubmit,
      isLoading: false,
      resetFlow: h.resetFlow,
      touched: h.step.touched,
      values: h.step.values,
    } as unknown as BaseInviteUserRenderProps);
  },
  useFetchAuthFlows: () => h.fetchAuthFlows,
  useFetchUserTypes: () => h.fetchUserTypes,
  useFetchUsers: () => h.fetchUsers,
  useThunderID: () => ({http: h.http, user: h.signedInUser.current}),
}));

// The resolver has its own tests; here it only decides which branch of the page renders.
vi.mock('@/utils/resolveAgentOnboardingFlow', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/utils/resolveAgentOnboardingFlow')>()),
  default: h.resolve,
}));

vi.mock('@thunderid/configure-users', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-users')>()),
  useFlowTextResolver: () => (text?: string) => text,
}));

// The OU picker fetches its own tree; the page only has to hand it the value and the change handler.
vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  OrganizationUnitTreePicker: ({
    value,
    onChange,
    rootOuId = undefined,
  }: {
    value: string;
    onChange: (id: string) => void;
    rootOuId?: string;
  }) => (
    <button
      type="button"
      data-testid="ou-picker"
      data-value={value}
      data-root-ou-id={rootOuId}
      onClick={() => onChange('ou-child-1')}
    >
      organization unit picker
    </button>
  ),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useToast: () => ({showToast: h.showToast}),
}));

vi.mock('@thunderid/logger/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/logger/react')>()),
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
        'agents:onboarding.errors.optionsFailed': 'Could not load the list. Try again.',
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

/** Serves `h.users` a page at a time, labelled the way the SDK's user loader labels them. */
const serveUsers = ({limit, offset}: SelectPageRequest): Promise<SelectPage> => {
  const users = h.users.current.slice(offset, offset + limit);

  return Promise.resolve(
    toUserSelectPage(
      {count: users.length, links: [], startIndex: offset + 1, totalResults: h.users.current.length, users} as never,
      offset,
    ),
  );
};

/** Builds a loader that serves one page of the given options. */
const serveOptions = (options: {label: string; value: string}[]) => (): Promise<SelectPage> =>
  Promise.resolve({nextOffset: null, options});

/** The users of a directory, numbered from one. */
const usersNumbered = (count: number): {display: string; id: string}[] =>
  Array.from({length: count}, (_, index) => ({display: `User ${index + 1}`, id: `user-${index + 1}`}));

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

/** An input step as the flow builder shapes it: the inputs sit in a block together with the action that submits them. */
const inBlock = (...inputs: Record<string, unknown>[]): Record<string, unknown>[] => [
  {
    components: [
      ...inputs,
      {eventType: 'SUBMIT', id: 'action_submit', label: 'Continue', type: 'ACTION', variant: 'PRIMARY'},
    ],
    id: 'block_step',
    type: 'BLOCK',
  },
];

/** Reports a flow status back to the page, the way the driver does on every response. */
const flowReports = async (flowStatus: string): Promise<void> => {
  await act(async () => {
    h.flowProps.current?.onFlowChange({flowStatus});
    await Promise.resolve();
  });
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('AgentOnboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.flowProps.current = null;
    h.pathname.current = '/agents/create';
    h.step.components = [];
    h.step.additionalData = {};
    h.step.values = {};
    h.step.fieldErrors = {};
    h.step.touched = {};
    h.handleSubmit.mockResolvedValue(undefined);
    h.users.current = [];
    h.fetchUsers.mockImplementation(serveUsers);
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
    // Each type the shipped flow uses is drawn by the design adapter for it. The test tree mounts no
    // DesignProvider, matching the console.
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

    // USER_SELECT carries no options: the adapter loads the candidates with the signed-in user's token
    // and shows each user the way the deployment names them, while the value submitted stays the
    // identifier.
    it('should offer users by name and submit the identifier', async () => {
      h.users.current = [
        {display: 'Ada Lovelace', id: 'user-1'},
        {attributes: {username: 'grace'}, id: 'user-2'},
      ];
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      expect(await screen.findByText('Owner')).toBeInTheDocument();
      await userEvent.click(screen.getByRole('combobox'));

      const options = await screen.findAllByRole('option');

      expect(options.map((option) => option.textContent)).toEqual(['Select an option', 'Ada Lovelace', 'grace']);
      expect(options[1]).toHaveAttribute('data-value', 'user-1');
      expect(options[2]).toHaveAttribute('data-value', 'user-2');
    });

    it('should name a user by email, then by identifier, when there is no display name or username', async () => {
      h.users.current = [{attributes: {email: 'grace@example.com'}, id: 'user-2'}, {id: 'user-3'}];
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));

      const options = await screen.findAllByRole('option');

      expect(options.map((option) => option.textContent)).toEqual(['Select an option', 'grace@example.com', 'user-3']);
    });

    it('should request no users until the owner list is opened', async () => {
      h.users.current = usersNumbered(3);
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await screen.findByText('Owner');

      expect(h.fetchUsers).not.toHaveBeenCalled();
    });

    it('should load users in pages of 30 from the first offset', async () => {
      h.users.current = usersNumbered(3);
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));
      await screen.findByRole('option', {name: 'User 1'});

      expect(h.fetchUsers).toHaveBeenCalledTimes(1);
      expect(h.fetchUsers).toHaveBeenCalledWith(expect.objectContaining({limit: 30, offset: 0}));
    });

    // The shared test setup replaces IntersectionObserver with a no-op, so the end of the list is
    // reported by hand here. Watching it is covered against a real browser in the design package.
    it('should load the rest of the users once the end of the list is reached', async () => {
      const observerCallbacks: ((entries: {isIntersecting: boolean}[]) => void)[] = [];
      vi.stubGlobal(
        'IntersectionObserver',
        class {
          constructor(callback: (entries: {isIntersecting: boolean}[]) => void) {
            observerCallbacks.push(callback);
          }

          observe = vi.fn();

          disconnect = vi.fn();
        },
      );
      h.users.current = usersNumbered(45);
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));
      await screen.findByRole('option', {name: 'User 30'});
      await waitFor(() => expect(observerCallbacks.length).toBeGreaterThan(0));
      await act(async () => {
        observerCallbacks[observerCallbacks.length - 1]([{isIntersecting: true}]);
        await Promise.resolve();
      });

      expect(await screen.findByRole('option', {name: 'User 45'})).toBeInTheDocument();
      expect(h.fetchUsers).toHaveBeenCalledTimes(2);
      expect(h.fetchUsers).toHaveBeenLastCalledWith(expect.objectContaining({limit: 30, offset: 30}));
    });

    // A picker has nowhere on the page to show a failed load, so without a toast the list would just look
    // empty and the administrator could not tell a permission problem from a directory with no users.
    it('should tell the administrator when a list cannot be loaded', async () => {
      h.fetchUsers.mockRejectedValue(new Error('forbidden'));
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));

      await waitFor(() => expect(h.showToast).toHaveBeenCalledWith('Could not load the list. Try again.', 'error'));
      expect(h.showToast).toHaveBeenCalledTimes(1);
    });

    it('should not show the server message when a list cannot be loaded', async () => {
      h.fetchUsers.mockRejectedValue(new Error('SQL logic error: no such column'));
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));

      await waitFor(() => expect(h.showToast).toHaveBeenCalled());
      expect(h.showToast.mock.calls[0][0]).not.toContain('SQL');
    });

    it('should not show a toast when the list loads', async () => {
      h.users.current = usersNumbered(3);
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));
      await screen.findByRole('option', {name: 'User 1'});

      expect(h.showToast).not.toHaveBeenCalled();
    });

    it('should submit the id of the user that is picked', async () => {
      h.users.current = usersNumbered(3);
      givenStep(inBlock({id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));
      await userEvent.click(await screen.findByRole('option', {name: 'User 2'}));

      expect(h.handleInputChange).toHaveBeenCalledWith('owner', 'user-2');
    });

    it('should offer authentication flows by name and submit the flow id', async () => {
      h.fetchAuthFlows.mockImplementation(
        serveOptions([
          {label: 'Default login', value: 'flow-1'},
          {label: 'mfa-login', value: 'flow-2'},
        ]),
      );
      givenStep(inBlock({id: 'flow_input', label: 'Authentication flow', ref: 'authFlowId', type: 'AUTH_FLOW_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));
      await userEvent.click(await screen.findByRole('option', {name: 'mfa-login'}));

      expect(h.handleInputChange).toHaveBeenCalledWith('authFlowId', 'flow-2');
    });

    it('should offer user types and submit the handle', async () => {
      h.fetchUserTypes.mockImplementation(
        serveOptions([
          {label: 'employee', value: 'employee'},
          {label: 'customer', value: 'customer'},
        ]),
      );
      givenStep(inBlock({id: 'type_input', label: 'User type', ref: 'userType', type: 'USER_TYPE_SELECT'}));

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));
      await userEvent.click(await screen.findByRole('option', {name: 'customer'}));

      expect(h.handleInputChange).toHaveBeenCalledWith('userType', 'customer');
    });

    // SELECT options arrive either as {value, label} pairs or as bare values that double as labels.
    it('should offer the options a select carries, in either shape', async () => {
      givenStep(
        inBlock({
          id: 'tier_input',
          label: 'Tier',
          options: [{label: 'Gold', value: 'gold'}, 'silver'],
          ref: 'tier',
          type: 'SELECT',
        }),
      );

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('combobox'));

      const options = await screen.findAllByRole('option');

      expect(options.map((option) => option.textContent)).toEqual(['Select an option', 'Gold', 'silver']);
      expect(options[1]).toHaveAttribute('data-value', 'gold');
      expect(options[2]).toHaveAttribute('data-value', 'silver');
    });

    // OU_SELECT is the same contract: the picker sources its own tree, so the page only wires it up.
    it('should render the organization unit picker', async () => {
      givenStep(inBlock({id: 'ou_selection_input', label: 'Organization Unit', ref: 'ouId', type: 'OU_SELECT'}));

      render(<AgentOnboardPage />);

      expect(await screen.findByTestId('ou-picker')).toBeInTheDocument();
    });

    it('should root the organization unit picker where the flow says and record the chosen unit', async () => {
      givenStep(inBlock({id: 'ou_selection_input', label: 'Organization Unit', ref: 'ouId', type: 'OU_SELECT'}), {
        rootOuId: 'ou-root-1',
      });

      render(<AgentOnboardPage />);

      const picker = await screen.findByTestId('ou-picker');

      expect(picker).toHaveAttribute('data-root-ou-id', 'ou-root-1');
      await userEvent.click(picker);
      expect(h.handleInputChange).toHaveBeenCalledWith('ouId', 'ou-child-1');
    });

    it('should render nothing for an organization unit input that has no reference', async () => {
      givenStep(inBlock({id: 'ou_selection_input', label: 'Organization Unit', type: 'OU_SELECT'}));

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('button', {name: 'Continue'})).toBeInTheDocument();
      expect(screen.queryByTestId('ou-picker')).not.toBeInTheDocument();
    });
  });

  describe('components drawn by the shared flow renderer', () => {
    it('should render a divider', async () => {
      givenStep([{id: 'divider_1', type: 'DIVIDER', variant: 'HORIZONTAL'}]);

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('separator')).toBeInTheDocument();
    });

    it('should render an image', async () => {
      givenStep([{id: 'image_1', type: 'IMAGE', src: 'https://example.com/logo.png', alt: 'Agent logo'}]);

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('img', {name: 'Agent logo'})).toBeInTheDocument();
    });

    it('should render actions side by side in a stack', async () => {
      givenStep([
        {
          components: [
            {
              components: [
                {eventType: 'SUBMIT', id: 'action_back', label: 'Back', type: 'ACTION', variant: 'SECONDARY'},
                {eventType: 'SUBMIT', id: 'action_next', label: 'Next', type: 'ACTION', variant: 'PRIMARY'},
              ],
              direction: 'row',
              id: 'stack_actions',
              type: 'STACK',
            },
          ],
          id: 'block_stack',
          type: 'BLOCK',
        },
      ]);

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('button', {name: 'Back'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Next'})).toBeInTheDocument();
    });

    it('should run a trigger action that sits outside a block', async () => {
      givenStep([{eventType: 'TRIGGER', id: 'action_help', label: 'Get help', type: 'ACTION'}]);

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('button', {name: 'Get help'}));

      expect(h.handleSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_help'}), {});
    });
  });

  describe('submitting a step', () => {
    const nameStep = () =>
      givenStep(
        inBlock({id: 'name_input', label: 'Agent name', ref: 'name', required: true, type: 'TEXT_INPUT'}),
        {},
        {name: 'billing-agent'},
      );

    it('should submit the step values with the action that was pressed', async () => {
      nameStep();

      render(<AgentOnboardPage />);

      await userEvent.click(await screen.findByRole('button', {name: 'Continue'}));

      expect(h.handleSubmit).toHaveBeenCalledTimes(1);
      expect(h.handleSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_submit'}), {
        name: 'billing-agent',
      });
    });

    it('should submit with the Enter key from an input', async () => {
      nameStep();

      render(<AgentOnboardPage />);

      await userEvent.type(await screen.findByLabelText(/Agent name/), '{Enter}');

      expect(h.handleSubmit).toHaveBeenCalledTimes(1);
    });

    it('should not submit with the Enter key while a required input is empty', async () => {
      nameStep();
      h.step.values = {};

      render(<AgentOnboardPage />);

      await userEvent.type(await screen.findByLabelText(/Agent name/), '{Enter}');

      expect(h.handleSubmit).not.toHaveBeenCalled();
    });

    it('should treat a value of only spaces as missing', async () => {
      nameStep();
      h.step.values = {name: '   '};

      render(<AgentOnboardPage />);

      expect(await screen.findByRole('button', {name: 'Continue'})).toBeDisabled();
    });

    it('should keep the inputs editable while the step is incomplete', async () => {
      nameStep();
      h.step.values = {};

      render(<AgentOnboardPage />);

      expect(await screen.findByLabelText(/Agent name/)).toBeEnabled();
    });

    // The SDK checks formats and declarative rules when a step is submitted and reports them by input.
    it('should show a validation message the flow reports under its input', async () => {
      nameStep();
      h.step.fieldErrors = {name: 'Please enter a valid email address'};
      h.step.touched = {name: true};

      render(<AgentOnboardPage />);

      expect(await screen.findByText('Please enter a valid email address')).toBeInTheDocument();
    });

    it('should not show a message for an input the user has not touched', async () => {
      nameStep();
      h.step.fieldErrors = {name: 'Please enter a valid email address'};

      render(<AgentOnboardPage />);

      await screen.findByLabelText(/Agent name/);
      expect(screen.queryByText('Please enter a valid email address')).not.toBeInTheDocument();
    });
  });

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

      expect(h.navigate).toHaveBeenCalledWith(defaultAgentRoutePaths.agents.list());
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

  // A flow that fails returns no components, so without this the screen simply goes blank and the
  // only trace is a log line.
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

      expect(h.navigate).toHaveBeenCalledWith(defaultAgentRoutePaths.agents.list());
    });

    // The same page is mounted inside the welcome experience, where the agent list is not where the
    // administrator came from.
    it('should return to the get started page when opened from the welcome flow', async () => {
      h.pathname.current = defaultAgentRoutePaths.welcome.getStartedAgentsCreate();

      render(<AgentOnboardPage />);

      (await screen.findByRole('button', {name: 'Close'})).click();

      expect(h.navigate).toHaveBeenCalledWith(defaultAgentRoutePaths.welcome.getStarted());
    });
  });
});

describe('AgentOnboardPage breadcrumbs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.pathname.current = '/agents/create';
    h.users.current = [];
    h.fetchUsers.mockImplementation(serveUsers);
    h.resolve.mockResolvedValue({flowId: 'flow-1', handle: 'default-agent-onboarding-flow'});
    givenStep([]);
  });

  it('should show only the page itself before any step is seen', async () => {
    render(<AgentOnboardPage />);

    await waitFor(() => expect(screen.getByText('Add Agent')).toBeInTheDocument());
    expect(screen.queryByText('Name Your Agent')).not.toBeInTheDocument();
  });

  it('should name the current step from its heading', async () => {
    givenStep([{type: 'TEXT', variant: 'HEADING_1', label: 'Name Your Agent', id: 'h'}]);

    render(<AgentOnboardPage />);

    await waitFor(() => expect(screen.getAllByText('Name Your Agent').length).toBeGreaterThan(0));
    expect(screen.getByText('Add Agent')).toBeInTheDocument();
  });

  it('should ignore a step that carries no heading', async () => {
    givenStep([{type: 'TEXT', variant: 'HEADING_6', label: 'Just a subtitle', id: 'h'}]);

    render(<AgentOnboardPage />);

    // Waiting on the subtitle, not the crumb: the layout renders before the flow resolves.
    await waitFor(() => expect(screen.getByText('Just a subtitle')).toBeInTheDocument());

    expect(screen.getByText('Add Agent')).toBeInTheDocument();
    // It renders in the body, but it must not become a crumb.
    expect(screen.getAllByText('Just a subtitle')).toHaveLength(1);
  });

  it('should restart the flow when the first crumb is used', async () => {
    givenStep([{type: 'TEXT', variant: 'HEADING_1', label: 'Name Your Agent', id: 'h'}]);

    render(<AgentOnboardPage />);

    await waitFor(() => expect(screen.getByRole('button', {name: /add agent/i})).toBeInTheDocument());
    await userEvent.click(screen.getByRole('button', {name: /add agent/i}));

    expect(h.resetFlow).toHaveBeenCalled();
  });
});

describe('AgentOnboardPage owner default', () => {
  const ownerInput = {id: 'owner_input', label: 'Owner', ref: 'owner', required: false, type: 'USER_SELECT'};

  beforeEach(() => {
    vi.clearAllMocks();
    h.pathname.current = '/agents/create';
    h.signedInUser.current = null;
    h.users.current = [{display: 'Ada Lovelace', id: 'user-1'}];
    h.fetchUsers.mockImplementation(serveUsers);
    h.resolve.mockResolvedValue({flowId: 'flow-1', handle: 'default-agent-onboarding-flow'});
  });

  it('should preselect the signed-in user as the owner', async () => {
    h.signedInUser.current = {id: 'user-1'};
    givenStep(inBlock(ownerInput));

    render(<AgentOnboardPage />);

    await waitFor(() => expect(h.handleInputChange).toHaveBeenCalledWith('owner', 'user-1'));
  });

  // The profile arrives from /users/me after the id token, so there is a render with no id yet.
  it('should not preselect before the signed-in user is known', async () => {
    givenStep(inBlock(ownerInput));

    render(<AgentOnboardPage />);

    await screen.findByText('Owner');
    expect(h.handleInputChange).not.toHaveBeenCalled();
  });

  it('should leave an owner the user already chose alone', async () => {
    h.signedInUser.current = {id: 'user-1'};
    h.users.current = [
      {display: 'Ada Lovelace', id: 'user-1'},
      {display: 'Grace Hopper', id: 'user-2'},
    ];
    givenStep(inBlock(ownerInput), {}, {owner: 'user-2'});

    render(<AgentOnboardPage />);

    await screen.findByText('Owner');
    expect(h.handleInputChange).not.toHaveBeenCalled();
  });

  // The signed-in user may sit beyond the first page of a large directory, so their name is taken from
  // their own profile instead of waiting for the page that holds them.
  it('should name the signed-in user before the page that holds them has loaded', async () => {
    h.signedInUser.current = {id: 'user-99', username: 'ada'} as {id?: string};
    h.users.current = usersNumbered(3);
    givenStep(inBlock(ownerInput), {}, {owner: 'user-99'});

    render(<AgentOnboardPage />);

    expect(await screen.findByRole('combobox')).toHaveTextContent('ada');
    expect(h.fetchUsers).not.toHaveBeenCalled();
  });

  // A user picker for anything other than the owner is not about the caller.
  it('should not preselect a user picker that is not the owner', async () => {
    h.signedInUser.current = {id: 'user-1'};
    givenStep(
      inBlock({id: 'approver_input', label: 'Approver', ref: 'approver', required: false, type: 'USER_SELECT'}),
    );

    render(<AgentOnboardPage />);

    await screen.findByText('Approver');
    expect(h.handleInputChange).not.toHaveBeenCalled();
  });
});
