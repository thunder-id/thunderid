// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentAddFormPage from '../AgentAddFormPage';

const h = vi.hoisted(() => ({
  http: {request: vi.fn<(config: unknown) => Promise<{data: unknown}>>()},
  navigate: vi.fn(),
  pathname: {current: '/agents/create'},
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
  OrganizationUnitTreePicker: ({onChange, rootOuId = undefined}: {onChange: (ouId: string) => void; rootOuId?: string}) => (
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

const defaultType = {
  id: 'type-1',
  handle: 'default',
  displayName: 'Default',
  ouId: 'ou-root',
  schema: {
    llmModel: {type: 'string', required: true},
    environment: {type: 'string'},
  },
};

function requests(): RecordedRequest[] {
  return h.http.request.mock.calls.map(([config]) => config as RecordedRequest);
}

async function fillRequired(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.type(await screen.findByRole('textbox', {name: 'agents:addForm.name'}), '  Calendar Agent ');
  await user.click(await screen.findByRole('combobox'));
  await user.click(within(await screen.findByRole('listbox')).getByText('Default'));
  await user.click(await screen.findByTestId('ou-picker'));
}

describe('AgentAddFormPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.pathname.current = '/agents/create';
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST' && url.endsWith('/agents')) {
        return Promise.resolve({data: {id: 'agent-1', name: 'Calendar Agent'}});
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

  it('posts the name, description, type, organization unit and schema attributes to /agents', async () => {
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
    // The optional attribute left empty is omitted.
    expect(post?.data).toEqual({
      name: 'Calendar Agent',
      description: 'Exposes calendar tools',
      ouId: 'ou-child',
      type: 'default',
      attributes: {llmModel: 'claude'},
    });

    await waitFor(() => {
      expect(h.navigate).toHaveBeenCalledWith('/agents/agent-1');
    });

    // No flow is involved anywhere along the way.
    expect(requests().some((request) => request.url.includes('/flow/execute'))).toBe(false);
    expect(requests().some((request) => request.url.includes('/server-config/flow'))).toBe(false);
  });

  it('omits description and attributes when none are given', async () => {
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST') return Promise.resolve({data: {id: 'agent-2'}});
      if (url.includes('/agent-types/type-1')) return Promise.resolve({data: {...defaultType, schema: {}}});
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
        name: 'Calendar Agent',
        ouId: 'ou-child',
        type: 'default',
      });
    });
  });

  it('keeps submit disabled until a name, a type and an organization unit are given', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    const submit = await screen.findByRole('button', {name: 'agents:addForm.submit'});
    expect(submit).toBeDisabled();

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Default'));
    await user.click(await screen.findByTestId('ou-picker'));
    await screen.findByRole('textbox', {name: /llmModel/});
    expect(submit).toBeDisabled();

    await user.type(screen.getByRole('textbox', {name: 'agents:addForm.name'}), 'Agent');
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
