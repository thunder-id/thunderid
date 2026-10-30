// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ProjectsListPage from '../ProjectsListPage';
import type {Project} from '../useGetProjects';
import {PROJECT_SELECTION_STORAGE_KEY} from '../useProjectSelection';

const mockNavigate = vi.fn();
const mockUseGetProjects = vi.fn<() => unknown>();

vi.mock('../useGetProjects', () => ({
  default: () => mockUseGetProjects(),
}));

vi.mock('../useDeleteProject', () => ({
  default: () => ({mutate: vi.fn(), isPending: false, reset: vi.fn()}),
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: {language: 'en-US'},
  }),
}));

function project(id: string, displayName: string): Project {
  return {id, handle: id, displayName, rootOuId: `ou-${id}`, createdAt: new Date().toISOString()};
}

describe('ProjectsListPage', () => {
  beforeEach(() => {
    localStorage.clear();
    mockNavigate.mockReset();
    mockUseGetProjects.mockReturnValue({
      data: {projects: [project('shop', 'Shop'), project('support', 'Support')]},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it('lists every project as a card', () => {
    render(<ProjectsListPage />);

    expect(screen.getByRole('heading', {name: 'common:projects.title'})).toBeInTheDocument();
    expect(screen.getByTestId('project-card-shop')).toHaveTextContent('Shop');
    expect(screen.getByTestId('project-card-support')).toHaveTextContent('Support');
  });

  it('filters the projects by the search text', async () => {
    const user = userEvent.setup();
    render(<ProjectsListPage />);

    await user.type(screen.getByRole('textbox', {name: 'common:projects.search'}), 'sho');

    expect(screen.getByTestId('project-card-shop')).toBeInTheDocument();
    expect(screen.queryByTestId('project-card-support')).not.toBeInTheDocument();
  });

  it('says so when no project matches the search', async () => {
    const user = userEvent.setup();
    render(<ProjectsListPage />);

    await user.type(screen.getByRole('textbox', {name: 'common:projects.search'}), 'nothing');

    expect(screen.getByText('common:projects.noMatches')).toBeInTheDocument();
  });

  it('enters a project when its card is opened', async () => {
    const user = userEvent.setup();
    render(<ProjectsListPage />);

    await user.click(screen.getByRole('button', {name: /Support/}));

    expect(localStorage.getItem(PROJECT_SELECTION_STORAGE_KEY)).toBe('support');
    expect(mockNavigate).toHaveBeenCalledWith('/applications');
  });

  it('opens the create project page from the add button', async () => {
    const user = userEvent.setup();
    render(<ProjectsListPage />);

    await user.click(screen.getByRole('button', {name: 'common:projects.add'}));

    expect(mockNavigate).toHaveBeenCalledWith('/projects/create');
  });

  it('asks to confirm before deleting a project from its card', async () => {
    const user = userEvent.setup();
    render(<ProjectsListPage />);

    await user.click(
      within(screen.getByTestId('project-card-shop')).getByRole('button', {name: 'common:projects.delete.action'}),
    );

    expect(screen.getByRole('dialog')).toHaveTextContent('common:projects.delete.title');
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('says so when there are no projects', () => {
    mockUseGetProjects.mockReturnValue({data: {projects: []}, isLoading: false, error: null, refetch: vi.fn()});

    render(<ProjectsListPage />);

    expect(screen.getByText('common:projects.empty')).toBeInTheDocument();
  });

  it('shows the loading state while the projects load', () => {
    mockUseGetProjects.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: vi.fn()});

    render(<ProjectsListPage />);

    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows the error when the projects cannot be read', () => {
    mockUseGetProjects.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Forbidden'),
      refetch: vi.fn(),
    });

    render(<ProjectsListPage />);

    expect(screen.queryByTestId('project-card-shop')).not.toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });
});
