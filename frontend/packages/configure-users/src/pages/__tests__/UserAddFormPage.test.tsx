// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserAddFormPage from '../UserAddFormPage';

const mockNavigate = vi.fn();
const mockHttpRequest = vi.fn<(config: RecordedRequest) => Promise<{data: unknown}>>();

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
  useNavigate: () => mockNavigate,
}));

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useThunderID: () => ({http: {request: mockHttpRequest}}),
  };
});

vi.mock('@thunderid/configure-organization-units', () => ({
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

const userTypes = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  types: [{id: 'type-1', handle: 'customer', displayName: 'Customer', ouId: 'ou-root'}],
};

const customerType = {
  id: 'type-1',
  handle: 'customer',
  displayName: 'Customer',
  schema: {
    username: {type: 'string', required: true},
    nickname: {type: 'string'},
  },
};

function requests(): RecordedRequest[] {
  return mockHttpRequest.mock.calls.map(([config]) => config);
}

describe('UserAddFormPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockHttpRequest.mockImplementation((config: RecordedRequest) => {
      if (config.method === 'POST' && config.url.endsWith('/users')) {
        return Promise.resolve({data: {id: 'user-1'}});
      }
      if (config.url.endsWith('/user-types/type-1')) {
        return Promise.resolve({data: customerType});
      }
      if (config.url.includes('/user-types')) {
        return Promise.resolve({data: userTypes});
      }
      return Promise.reject(new Error(`unexpected request: ${config.url}`));
    });
  });

  it('posts the chosen type, organization unit and schema attributes to /users', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));

    const picker = await screen.findByTestId('ou-picker');
    expect(picker).toHaveAttribute('data-root', 'ou-root');
    await user.click(picker);

    await user.type(await screen.findByRole('textbox', {name: /username/i}), 'alice');
    await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));

    await waitFor(() => {
      expect(requests().some((request) => request.method === 'POST')).toBe(true);
    });

    const post = requests().find((request) => request.method === 'POST');
    expect(post?.url).toMatch(/\/users$/);
    // The optional field left empty is omitted.
    expect(post?.data).toEqual({ouId: 'ou-child', type: 'customer', attributes: {username: 'alice'}});

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });

    // No flow is involved anywhere along the way.
    expect(requests().some((request) => request.url.includes('/flow/execute'))).toBe(false);
    expect(requests().some((request) => request.url.includes('/server-config/flow'))).toBe(false);
  });

  it('keeps submit disabled until a type and an organization unit are chosen', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    const submit = await screen.findByRole('button', {name: 'users:addForm.submit'});
    expect(submit).toBeDisabled();

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
    await screen.findByRole('textbox', {name: /username/i});
    expect(submit).toBeDisabled();

    await user.click(screen.getByTestId('ou-picker'));
    expect(submit).toBeEnabled();
  });

  it('does not post when a required attribute is missing', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Customer'));
    await user.click(await screen.findByTestId('ou-picker'));
    await screen.findByRole('textbox', {name: /username/i});
    await user.click(screen.getByRole('button', {name: 'users:addForm.submit'}));

    await screen.findByText(/username is required/i);
    expect(requests().some((request) => request.method === 'POST')).toBe(false);
  });

  it('says so when no user types exist', async () => {
    mockHttpRequest.mockResolvedValue({data: {totalResults: 0, startIndex: 1, count: 0, types: []}});
    render(<UserAddFormPage />);

    expect(await screen.findByText('users:addForm.noUserTypes')).toBeInTheDocument();
  });

  it('returns to the user list on cancel', async () => {
    const user = userEvent.setup();
    render(<UserAddFormPage />);

    await user.click(await screen.findByRole('button', {name: 'common:actions.cancel'}));

    expect(mockNavigate).toHaveBeenCalledWith('/users');
  });
});
