// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import PendingDeployBanner from '../PendingDeployBanner';

vi.mock('../../hooks/useProjectCounts', () => ({default: () => ''}));

const selection = vi.hoisted(() => ({
  selectedProject: undefined as {id: string} | undefined,
  selectProject: vi.fn(),
}));
vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({
    projects: [
      {id: 'p-shop', handle: 'shop', displayName: 'Shop'},
      {id: 'p-bill', handle: 'bill', displayName: 'Billing'},
    ],
    selectedProject: selection.selectedProject,
    selectProject: selection.selectProject,
  }),
}));

const state = vi.hoisted(() => ({auto: false, byProject: {} as Record<string, number>, shared: 0}));
vi.mock('@thunderid/configure-gateways', () => ({
  useAutoApply: () => ({isLoading: false, data: {enabled: state.auto}}),
  useProjectsPending: (_target: string, projects: string[]) => ({
    byProject: Object.fromEntries(projects.map((id) => [id, state.byProject[id] ?? 0])),
    shared: state.shared,
    isLoading: false,
    isError: false,
  }),
  usePipeline: () => ({
    isLoading: false,
    data: {stages: [{name: 'Development', gateway: {id: 'gw-dev', name: 'dev', baseUrl: 'https://dev.test'}}]},
  }),
}));

describe('PendingDeployBanner', () => {
  beforeEach(() => {
    selection.selectProject.mockReset();
    selection.selectedProject = undefined;
    state.auto = false;
    state.byProject = {'p-shop': 2};
    state.shared = 1;
  });

  it("asks which project to deploy, then opens that project's pipeline", async () => {
    render(<PendingDeployBanner />);

    expect(screen.getByText("You have changes that aren't in Development yet")).toBeInTheDocument();
    expect(
      screen.getByText('2 changes in Shop and 1 in shared resources. Deploy them to Development to try them out.'),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('organization-deploy'));
    expect(
      screen.getByText('Choose the project to deploy. Each project deploys through its own pipeline.'),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('project-picker-continue'));

    expect(selection.selectProject).toHaveBeenCalledWith('shop', '/pipeline');
  });

  it("counts only the selected project's changes and the shared ones, and opens its pipeline at once", async () => {
    selection.selectedProject = {id: 'p-bill'};
    state.byProject = {'p-shop': 2, 'p-bill': 3};
    render(<PendingDeployBanner />);

    expect(
      screen.getByText('3 changes in Billing and 1 in shared resources. Deploy them to Development to try them out.'),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('organization-deploy'));

    expect(screen.queryByTestId('project-picker')).not.toBeInTheDocument();
    expect(selection.selectProject).toHaveBeenCalledWith('bill', '/pipeline');
  });

  it('says nothing while auto deploy is on, or when nothing is waiting', () => {
    state.auto = true;
    const {unmount} = render(<PendingDeployBanner />);
    expect(screen.queryByTestId('organization-pending')).not.toBeInTheDocument();
    unmount();

    state.auto = false;
    state.byProject = {};
    state.shared = 0;
    render(<PendingDeployBanner />);
    expect(screen.queryByTestId('organization-pending')).not.toBeInTheDocument();
  });
});
