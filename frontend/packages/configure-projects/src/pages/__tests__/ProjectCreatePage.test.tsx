// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen, userEvent, waitFor} from '@thunderid/test-utils';
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
});
