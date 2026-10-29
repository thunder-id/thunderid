// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentAddFormPage from '../AgentAddFormPage';

const h = vi.hoisted(() => ({
  http: {request: vi.fn<(config: unknown) => Promise<{data: unknown}>>()},
  navigate: vi.fn(),
  pathname: {current: '/agents/create'},
  created: {current: {id: 'agent-1', name: 'Calendar Agent'} as Record<string, unknown>},
}));

// Translations are shared across the suite, so keys are asserted rather than English copy.
vi.mock('react-i18next', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useTranslation: () => ({t: (key: string) => key}),
  };
});

vi.mock('react-router', async () => ({
  ...(await vi.importActual<typeof import('react-router')>('react-router')),
  useNavigate: () => h.navigate,
  useLocation: () => ({pathname: h.pathname.current}),
}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: h.http}),
}));

vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  OrganizationUnitTreePicker: ({
    onChange,
    rootOuId = undefined,
  }: {
    onChange: (ouId: string) => void;
    rootOuId?: string;
  }) => (
    <button type="button" data-testid="ou-picker" data-root={rootOuId} onClick={() => onChange('ou-child')}>
      pick ou
    </button>
  ),
}));

interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

const agentTypes = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  types: [{id: 'type-1', handle: 'default', displayName: 'Default', ouId: 'ou-root'}],
};

const users = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  users: [{id: 'user-1', ouId: 'ou-root', type: 'person', display: 'alice'}],
};

const defaultType = {
  id: 'type-1',
  handle: 'default',
  displayName: 'Default',
  ouId: 'ou-root',
  schema: {
    name: {type: 'string', required: true},
    llmModel: {type: 'string', required: true},
    environment: {type: 'string'},
  },
};

const ISSUED_CREDENTIALS = {
  type: 'oauth2',
  config: {grantTypes: ['client_credentials'], responseTypes: [], tokenEndpointAuthMethod: 'client_secret_basic'},
};

function requests(): RecordedRequest[] {
  return h.http.request.mock.calls.map(([config]) => config as RecordedRequest);
}

// The agent type select comes first in the form and the owner select second.
async function chooseOption(user: ReturnType<typeof userEvent.setup>, index: number, label: string): Promise<void> {
  await user.click((await screen.findAllByRole('combobox'))[index]);
  await user.click(within(await screen.findByRole('listbox')).getByText(label));
}

async function fillRequired(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await chooseOption(user, 0, 'Default');
  await user.click(await screen.findByTestId('ou-picker'));
  await user.type(await screen.findByRole('textbox', {name: /^name/}), 'Calendar Agent');
}

describe('AgentAddFormPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.pathname.current = '/agents/create';
    h.created.current = {id: 'agent-1', name: 'Calendar Agent'};
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST' && url.endsWith('/agents')) {
        return Promise.resolve({data: h.created.current});
      }
      if (url.includes('/users')) {
        return Promise.resolve({data: users});
      }
      if (url.includes('/agent-types/type-1')) {
        return Promise.resolve({data: defaultType});
      }
      if (url.includes('/agent-types')) {
        return Promise.resolve({data: agentTypes});
      }
      return Promise.reject(new Error(`unexpected request: ${url}`));
    });
  });

  it('posts the description, type, organization unit and schema attributes, the name among them, to /agents', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    expect(screen.getByTestId('ou-picker')).toHaveAttribute('data-root', 'ou-root');
    await user.type(screen.getByRole('textbox', {name: 'agents:addForm.description'}), 'Exposes calendar tools');
    await user.type(await screen.findByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(requests().some((request) => request.method === 'POST')).toBe(true);
    });

    const post = requests().find((request) => request.method === 'POST');
    expect(post?.url).toMatch(/\/agents$/);
    // The optional attribute left empty is omitted; OAuth client credentials are asked for by default,
    // naming only the grant and the authentication so the API generates the client ID and secret.
    expect(post?.data).toEqual({
      description: 'Exposes calendar tools',
      ouId: 'ou-child',
      type: 'default',
      attributes: {name: 'Calendar Agent', llmModel: 'claude'},
      inboundAuthConfig: [ISSUED_CREDENTIALS],
    });

    await waitFor(() => {
      expect(h.navigate).toHaveBeenCalledWith('/agents/agent-1');
    });

    // No flow is involved anywhere along the way.
    expect(requests().some((request) => request.url.includes('/flow/execute'))).toBe(false);
    expect(requests().some((request) => request.url.includes('/server-config/flow'))).toBe(false);
  });

  it('posts the chosen owner, and none when no owner is chosen', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await user.type(await screen.findByRole('textbox', {name: /llmModel/}), 'claude');
    await chooseOption(user, 1, 'alice');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toMatchObject({owner: 'user-1'});
    });
  });

  // Answers the users request with the given outcome and everything else as the other tests do.
  function answerUsersWith(outcome: () => Promise<{data: unknown}>): void {
    const base = h.http.request.getMockImplementation() as (config: unknown) => Promise<{data: unknown}>;
    h.http.request.mockImplementation((config: unknown) =>
      (config as RecordedRequest).url.includes('/users') ? outcome() : base(config),
    );
  }

  it('tells two users with the same display name apart by their IDs', async () => {
    answerUsersWith(() =>
      Promise.resolve({
        data: {
          totalResults: 3,
          startIndex: 1,
          count: 3,
          users: [
            {id: 'user-1', ouId: 'ou-root', type: 'person', display: 'sam'},
            {id: 'user-2', ouId: 'ou-root', type: 'person', display: 'sam'},
            {id: 'user-3', ouId: 'ou-root', type: 'person', display: 'alice'},
          ],
        },
      }),
    );
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await user.click((await screen.findAllByRole('combobox'))[1]);
    const listbox = await screen.findByRole('listbox');

    expect(within(listbox).getByText('sam (user-1)')).toBeInTheDocument();
    expect(within(listbox).getByText('sam (user-2)')).toBeInTheDocument();
    expect(within(listbox).getByText('alice')).toBeInTheDocument();
  });

  it('says so when the users to choose an owner from cannot be loaded, and offers a retry', async () => {
    let attempts = 0;
    answerUsersWith(() => {
      attempts += 1;
      return attempts === 1 ? Promise.reject(new Error('network down')) : Promise.resolve({data: users});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await user.click(await screen.findByRole('button', {name: 'common:actions.refresh'}));

    // Only the agent type select is left until the owners load, then the owner select is back.
    await waitFor(() => {
      expect(screen.getAllByRole('combobox')).toHaveLength(2);
    });
    expect(attempts).toBeGreaterThan(1);
  });

  it('says when only the first users are offered as owners', async () => {
    answerUsersWith(() => Promise.resolve({data: {...users, totalResults: 250}}));
    render(<AgentAddFormPage />);

    expect(await screen.findByText('agents:addForm.ownerTruncated')).toBeInTheDocument();
  });

  it('shows the issued client secret once, then opens the agent', async () => {
    h.created.current = {
      id: 'agent-1',
      inboundAuthConfig: [{type: 'oauth2', config: {clientId: 'agent-client', clientSecret: 's3cret-value'}}],
    };
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await user.type(await screen.findByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('agents:addForm.secretIssued.title')).toBeInTheDocument();
    expect(h.navigate).not.toHaveBeenCalled();

    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(h.navigate).toHaveBeenCalledWith('/agents/agent-1');
    });
  });

  it('creates the agent without OAuth credentials when switched off, and opens it at once', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await user.type(await screen.findByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('switch', {name: 'agents:addForm.issueCredentials'}));
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(h.navigate).toHaveBeenCalledWith('/agents/agent-1');
    });
    const post = requests().find((request) => request.method === 'POST');
    expect(post?.data).not.toHaveProperty('inboundAuthConfig');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('omits the description when none is given', async () => {
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST') return Promise.resolve({data: {id: 'agent-2'}});
      if (url.includes('/agent-types/type-1')) {
        return Promise.resolve({data: {...defaultType, schema: {name: {type: 'string'}}}});
      }
      return Promise.resolve({data: agentTypes});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    const submit = screen.getByRole('button', {name: 'agents:addForm.submit'});
    await waitFor(() => expect(submit).toBeEnabled());
    await user.click(submit);

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toEqual({
        ouId: 'ou-child',
        type: 'default',
        attributes: {name: 'Calendar Agent'},
        inboundAuthConfig: [ISSUED_CREDENTIALS],
      });
    });
  });

  it('asks for a full page of types and posts nothing a previously chosen type was given', async () => {
    const twoTypes = {
      ...agentTypes,
      totalResults: 2,
      count: 2,
      types: [...agentTypes.types, {id: 'type-2', handle: 'assistant', displayName: 'Assistant', ouId: 'ou-root'}],
    };
    const assistantType = {...defaultType, id: 'type-2', handle: 'assistant', displayName: 'Assistant'};
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST') return Promise.resolve({data: {id: 'agent-3'}});
      if (url.includes('/agent-types/type-1')) return Promise.resolve({data: defaultType});
      if (url.includes('/agent-types/type-2')) return Promise.resolve({data: assistantType});
      return Promise.resolve({data: twoTypes});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    const choose = (type: string): Promise<void> => chooseOption(user, 0, type);
    await screen.findAllByRole('combobox');
    await choose('Default');
    await screen.findByRole('textbox', {name: /environment/});
    await choose('Assistant');
    await user.type(await screen.findByRole('textbox', {name: /environment/}), 'staging');
    // Back to a type whose schema is already loaded, so its fields stay mounted across the change.
    await choose('Default');
    await user.click(await screen.findByTestId('ou-picker'));
    await waitFor(() => expect(screen.getByRole('textbox', {name: /environment/})).toHaveValue(''));
    await user.type(screen.getByRole('textbox', {name: /^name/}), 'Helper');
    await user.type(screen.getByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toEqual({
        ouId: 'ou-child',
        type: 'default',
        attributes: {name: 'Helper', llmModel: 'claude'},
        inboundAuthConfig: [ISSUED_CREDENTIALS],
      });
    });
    expect(requests().some((request) => /\/agent-types\?.*limit=100/.test(request.url))).toBe(true);
  });

  it('keeps submit disabled until a type and an organization unit are given', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    const submit = await screen.findByRole('button', {name: 'agents:addForm.submit'});
    expect(submit).toBeDisabled();

    await chooseOption(user, 0, 'Default');
    await user.click(await screen.findByTestId('ou-picker'));
    await screen.findByRole('textbox', {name: /llmModel/});
    expect(submit).toBeEnabled();
  });

  it('does not post when a required attribute is missing', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await screen.findByRole('textbox', {name: /llmModel/});
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await screen.findByText(/llmModel is required/);
    expect(requests().some((request) => request.method === 'POST')).toBe(false);
  });

  it('says so when no agent types exist', async () => {
    h.http.request.mockResolvedValue({data: {totalResults: 0, startIndex: 1, count: 0, types: []}});
    render(<AgentAddFormPage />);

    expect(await screen.findByText('agents:addForm.noAgentTypes')).toBeInTheDocument();
  });

  it('returns to the agent list on cancel', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await user.click(await screen.findByRole('button', {name: 'common:actions.cancel'}));

    expect(h.navigate).toHaveBeenCalledWith('/agents');
  });

  it('returns to get started on cancel when opened from the welcome journey', async () => {
    h.pathname.current = '/welcome/get-started/agents/create';
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await user.click(await screen.findByRole('button', {name: 'common:actions.cancel'}));

    expect(h.navigate).toHaveBeenCalledWith('/welcome/get-started');
  });
});
