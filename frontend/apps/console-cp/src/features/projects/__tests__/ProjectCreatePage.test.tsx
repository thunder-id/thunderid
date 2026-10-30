// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ProjectCreatePage from '../ProjectCreatePage';
import type {CreateProjectRequest} from '../useCreateProject';
import type {Project} from '../useGetProjects';

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

vi.mock('../useCreateProject', () => ({
  default: () => ({mutate: mockMutate, isPending: false}),
}));

vi.mock('../useProjectSelection', () => ({
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
    expect(screen.getByRole('textbox', {name: /Handle/})).toHaveAttribute('readonly');
  });

  it('keeps an edited handle when the name changes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.click(screen.getByRole('button', {name: 'Edit handle'}));
    const handle = screen.getByRole('textbox', {name: /Handle/});
    expect(handle).not.toHaveAttribute('readonly');
    expect(handle).toHaveFocus();

    await user.type(handle, 'shop');
    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Online Shop');

    expect(handle).toHaveValue('shop');
  });

  it('returns to the projects page on cancel', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(mockNavigate).toHaveBeenCalledWith('/projects');
  });

  it('creates the project, selects it and opens its applications', async () => {
    mockMutate.mockImplementation((_request, options) =>
      options.onSuccess?.({
        id: 'p1',
        handle: 'online-shop',
        displayName: 'Online Shop',
        rootOuId: 'ou1',
        createdAt: '2026-10-01T00:00:00Z',
      }),
    );
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Online Shop');
    await user.type(screen.getByRole('textbox', {name: /Description/}), 'Storefront');
    await waitFor(() => expect(screen.getByRole('button', {name: 'Create'})).toBeEnabled());
    await user.click(screen.getByRole('button', {name: 'Create'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {handle: 'online-shop', displayName: 'Online Shop', description: 'Storefront'},
      expect.anything(),
    );
    expect(mockSelectProject).toHaveBeenCalledWith('p1');
    expect(mockNavigate).toHaveBeenCalledWith('/applications');
  });

  it('rejects a handle with characters other than lowercase letters, digits and hyphens', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ProjectCreatePage />);

    await user.type(screen.getByRole('textbox', {name: /Name/}), 'Shop');
    await user.click(screen.getByRole('button', {name: 'Edit handle'}));
    await user.clear(screen.getByRole('textbox', {name: /Handle/}));
    await user.type(screen.getByRole('textbox', {name: /Handle/}), 'Shop/1');

    expect(await screen.findByText('Handle must be lowercase alphanumeric with hyphens only')).toBeInTheDocument();
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
