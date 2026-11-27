// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderWithProviders, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {Project} from '../../models/project';
import type {CreateProjectRequest} from '../../models/requests';
import ProjectCreatePage from '../ProjectCreatePage';

const mockNavigate = vi.fn(() => Promise.resolve());
const mockSelectProject = vi.fn();
const mockMutate =
  vi.fn<
    (
      request: CreateProjectRequest,
      options: {onSuccess?: (project: Project) => void; onError?: (error: Error) => void},
    ) => void
  >();

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('@thunderid/utils', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/utils')>()),
  generateRandomHumanReadableIdentifiers: () => ['Fresh Towns Hide'],
}));

vi.mock('../../api/useCreateProject', () => ({
  default: () => ({mutate: mockMutate, isPending: false}),
}));

vi.mock('../../hooks/useProjectSelection', () => ({
  default: () => ({selectProject: mockSelectProject}),
}));

describe('ProjectCreatePage', () => {
  beforeEach(() => {
    mockNavigate.mockClear();
    mockSelectProject.mockReset();
    mockMutate.mockReset();
  });

  it('derives the handle from the name', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Online Shop');

    expect(screen.getByRole('textbox', {name: /Handle/})).toHaveValue('online-shop');
  });

  it('fills the name and handle from the name suggestion', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.click(screen.getByRole('button', {name: 'Fresh Towns Hide'}));

    expect(screen.getByRole('textbox', {name: /Name/})).toHaveValue('Fresh Towns Hide');
    expect(screen.getByRole('textbox', {name: /Handle/})).toHaveValue('fresh-towns-hide');
  });

  it('keeps an edited handle when the name changes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    const handle = screen.getByRole('textbox', {name: /Handle/});
    await user.type(handle, 'shop');
    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Online Shop');

    expect(handle).toHaveValue('shop');
  });

  it('returns to the projects page on close', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.click(screen.getByRole('button', {name: 'Close'}));

    expect(mockNavigate).toHaveBeenCalledWith('/projects');
  });

  it('creates the project, selects it and opens its applications', async () => {
    mockMutate.mockImplementation((_request, options) =>
      options.onSuccess?.({
        id: 'p1',
        handle: 'online-shop',
        displayName: 'Online Shop',
        createdAt: '2026-10-01T00:00:00Z',
      }),
    );
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Online Shop');
    await waitFor(() => expect(screen.getByRole('button', {name: 'Create'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Create'}));

    expect(mockMutate).toHaveBeenCalledWith({handle: 'online-shop', displayName: 'Online Shop'}, expect.anything());
    expect(mockSelectProject).toHaveBeenCalledWith('online-shop', '/applications');
  });

  it('sends the description when one is given', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Shop');
    await user.type(screen.getByRole('textbox', {name: /Description/}), '  Storefront apps  ');
    await waitFor(() => expect(screen.getByRole('button', {name: 'Create'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Create'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {handle: 'shop', displayName: 'Shop', description: 'Storefront apps'},
      expect.anything(),
    );
  });

  it('accepts underscores in the handle', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Shop');
    await user.clear(screen.getByRole('textbox', {name: /Handle/}));
    await user.type(screen.getByRole('textbox', {name: /Handle/}), 'my_shop');

    await waitFor(() => expect(screen.getByRole('button', {name: 'Create'})).toBeEnabled());
  });

  it('rejects a handle with characters other than lowercase letters, digits, hyphens and underscores', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Shop');
    await user.clear(screen.getByRole('textbox', {name: /Handle/}));
    await user.type(screen.getByRole('textbox', {name: /Handle/}), 'Shop/1');

    expect(
      await screen.findByText(
        'Handle may contain only lowercase letters, numbers, hyphens and underscores, and must start and end with a letter or a number',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Create'})).toBeDisabled();
  });

  it('shows the error when the project cannot be created', async () => {
    mockMutate.mockImplementation((_request, options) => options.onError?.(new Error('Conflict')));
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Shop');
    await waitFor(() => expect(screen.getByRole('button', {name: 'Create'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Create'}));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  const failWith = (status: number, data: unknown) =>
    mockMutate.mockImplementation((_request, options) =>
      options.onError?.(
        Object.assign(new Error(`Request failed with status code ${status}`), {response: {status, data}}),
      ),
    );

  const submit = async (): Promise<void> => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Shop');
    await waitFor(() => expect(screen.getByRole('button', {name: 'Create'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Create'}));
  };

  const TAKEN = 'A project with the handle "shop" already exists. Choose a different handle.';

  it('shows a handle the project API reports as taken in the page alert', async () => {
    failWith(409, {
      code: 'PRJ-1012',
      message: {key: 'error.projectservice.handle_conflict', defaultValue: 'Project handle already exists'},
      description: {
        key: 'error.projectservice.handle_conflict_description',
        defaultValue: 'Supply a handle that no other project uses',
      },
    });
    await submit();

    expect(await screen.findByRole('alert')).toHaveTextContent(TAKEN);
    expect(screen.queryByText(/Supply a handle/)).not.toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('takes a conflict without an error code as a taken handle', async () => {
    failWith(409, {
      error: 'conflict: creating project shop: {"code":"CONFLICT","error":"Project already exists"}',
    });
    await submit();

    expect(await screen.findByRole('alert')).toHaveTextContent(TAKEN);
    expect(screen.queryByText(/creating project/)).not.toBeInTheDocument();
  });

  it('shows the generic message for a conflict with another error code', async () => {
    failWith(409, {
      code: 'PRJ-1006',
      message: {key: 'error.projectservice.project_id_conflict', defaultValue: 'Project ID already exists'},
      description: {
        key: 'error.projectservice.project_id_conflict_description',
        defaultValue: 'Omit the ID to have one generated, or supply an ID that no other project uses',
      },
    });
    await submit();

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to create the project. Please try again.');
    expect(screen.queryByText(/Omit the ID/)).not.toBeInTheDocument();
  });

  it('names the submitted handle when the handle was edited while the create was pending', async () => {
    let fail: ((error: Error) => void) | undefined;
    mockMutate.mockImplementation((_request, options) => {
      fail = options.onError;
    });
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Shop');
    await waitFor(() => expect(screen.getByRole('button', {name: 'Create'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Create'}));
    await user.type(screen.getByRole('textbox', {name: /Handle/}), '-new');
    act(() => fail?.(Object.assign(new Error('Conflict'), {response: {status: 409, data: {code: 'PRJ-1012'}}})));

    expect(await screen.findByRole('alert')).toHaveTextContent(TAKEN);
  });

  it('clears the taken handle message when the form changes', async () => {
    failWith(409, {code: 'PRJ-1012'});
    await submit();
    expect(await screen.findByRole('alert')).toHaveTextContent(TAKEN);

    await userEvent.setup().type(screen.getByRole('textbox', {name: /Handle/}), '-new');

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
