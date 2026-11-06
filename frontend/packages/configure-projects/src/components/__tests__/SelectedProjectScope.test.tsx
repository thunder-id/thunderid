// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useProjectScope} from '@thunderid/contexts';
import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import {PROJECT_SELECTION_STORAGE_KEY} from '../../hooks/useProjectSelection';
import type {Project} from '../../models/project';
import SelectedProjectScope from '../SelectedProjectScope';

const mockUseGetProjects =
  vi.fn<() => {data?: {projects: Project[]}; isLoading?: boolean; error?: Error | null; refetch?: () => void}>();

vi.mock('../../api/useGetProjects', () => ({
  default: () => ({refetch: vi.fn(), error: null, isLoading: false, ...mockUseGetProjects()}),
}));

const shop: Project = {
  id: 'p1',
  handle: 'shop',
  displayName: 'Shop',
  createdAt: '2026-10-01T00:00:00Z',
};

function ScopedPage() {
  const {projectId} = useProjectScope();
  return <div data-testid="page">{projectId ?? 'none'}</div>;
}

const renderScope = () =>
  render(
    <SelectedProjectScope>
      <ScopedPage />
    </SelectedProjectScope>,
  );

describe('SelectedProjectScope', () => {
  beforeEach(() => {
    localStorage.clear();
    mockUseGetProjects.mockReset();
  });

  it('scopes the page to the selected project', () => {
    mockUseGetProjects.mockReturnValue({data: {projects: [shop]}});

    renderScope();

    expect(screen.getByTestId('page')).toHaveTextContent('p1');
  });

  it('holds the page back while the projects are loading', () => {
    mockUseGetProjects.mockReturnValue({data: undefined, isLoading: true});

    renderScope();

    expect(screen.queryByTestId('page')).not.toBeInTheDocument();
  });

  it('shows the error instead of the page when the projects cannot be read', () => {
    mockUseGetProjects.mockReturnValue({data: undefined, error: new Error('Forbidden')});

    renderScope();

    expect(screen.queryByTestId('page')).not.toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('lets the user clear the selection and work unscoped when the projects cannot be read', async () => {
    mockUseGetProjects.mockReturnValue({data: undefined, error: new Error('Forbidden')});

    const user = userEvent.setup();
    renderScope();
    await user.click(screen.getByRole('button', {name: 'Clear project selection'}));

    expect(localStorage.getItem(PROJECT_SELECTION_STORAGE_KEY)).toBe('');
    expect(screen.getByTestId('page')).toHaveTextContent('none');
  });

  it('does not scope or wait once the selection is cleared', () => {
    localStorage.setItem(PROJECT_SELECTION_STORAGE_KEY, '');
    mockUseGetProjects.mockReturnValue({data: undefined, isLoading: true});

    renderScope();

    expect(screen.getByTestId('page')).toHaveTextContent('none');
  });
});
