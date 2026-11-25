// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useProjectScope} from '@thunderid/contexts';
import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {Project} from '../../models/project';
import SelectedProjectScope from '../SelectedProjectScope';

const mockUseGetProjects =
  vi.fn<() => {data?: {projects: Project[]}; isLoading?: boolean; error?: Error | null; refetch?: () => void}>();
const mockNavigate = vi.fn();
let mockParams: Record<string, string> = {};

vi.mock('../../api/useGetProjects', () => ({
  default: () => ({refetch: vi.fn(), error: null, isLoading: false, ...mockUseGetProjects()}),
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
  useParams: () => mockParams,
  Navigate: ({to}: {to: string}) => <div data-testid="redirect">{to}</div>,
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
    mockUseGetProjects.mockReset();
    mockNavigate.mockReset();
    mockParams = {projectHandle: 'shop'};
  });

  it('scopes the page to the project the route names', () => {
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

  it('lets the user leave the project for the projects page when the projects cannot be read', async () => {
    mockUseGetProjects.mockReturnValue({data: undefined, error: new Error('Forbidden')});

    const user = userEvent.setup();
    renderScope();
    await user.click(screen.getByRole('button', {name: 'Clear project selection'}));

    expect(mockNavigate).toHaveBeenCalledWith('/projects');
  });

  it('leads to the projects page when no project has the handle', () => {
    mockParams = {projectHandle: 'removed'};
    mockUseGetProjects.mockReturnValue({data: {projects: [shop]}});

    renderScope();

    expect(screen.queryByTestId('page')).not.toBeInTheDocument();
    expect(screen.getByTestId('redirect')).toHaveTextContent('/projects');
  });

  it('does not scope or wait on a page outside every project', () => {
    mockParams = {};
    mockUseGetProjects.mockReturnValue({data: undefined, isLoading: true});

    renderScope();

    expect(screen.getByTestId('page')).toHaveTextContent('none');
  });
});
