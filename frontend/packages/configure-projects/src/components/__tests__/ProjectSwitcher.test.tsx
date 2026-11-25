// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {Header} from '@wso2/oxygen-ui';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {Project} from '../../models/project';
import ProjectSwitcher from '../ProjectSwitcher';

const mockNavigate = vi.fn();
const mockUseGetProjects = vi.fn<() => {data?: {projects: Project[]}}>();
let mockParams: Record<string, string> = {};
let mockPathname = '/';

vi.mock('../../api/useGetProjects', () => ({
  default: () => mockUseGetProjects(),
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
  useParams: () => mockParams,
  useLocation: () => ({pathname: mockPathname, search: '', hash: '', state: null, key: 'test'}),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

function project(handle: string, displayName: string): Project {
  return {id: `p-${handle}`, handle, displayName, createdAt: '2026-10-01T00:00:00Z'};
}

function renderSwitcher(): ReturnType<typeof render> {
  return render(
    <Header>
      <Header.Switchers showDivider={false}>
        <ProjectSwitcher />
      </Header.Switchers>
    </Header>,
  );
}

describe('ProjectSwitcher', () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockParams = {projectHandle: 'shop'};
    mockPathname = '/project/shop/applications/app-1';
    mockUseGetProjects.mockReturnValue({data: {projects: [project('shop', 'Shop'), project('support', 'Support')]}});
  });

  it('shows the project the route names as the selected one', () => {
    renderSwitcher();

    expect(screen.getByLabelText('projects:switcher.label')).toHaveTextContent('Shop');
    expect(screen.queryByText('Support')).not.toBeInTheDocument();
  });

  it('switches to the same section of another project', async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByLabelText('projects:switcher.label'));
    await user.click(within(screen.getByRole('listbox')).getByText('Support'));

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/project/support/applications'));
  });

  it("switches to another project's home from the project's home", async () => {
    mockPathname = '/project/shop';
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByLabelText('projects:switcher.label'));
    await user.click(within(screen.getByRole('listbox')).getByText('Support'));

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/project/support'));
  });

  it('leaves the project for the projects page', async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole('button', {name: 'projects:switcher.clear', hidden: true}));

    expect(mockNavigate).toHaveBeenCalledWith('/projects');
  });

  it('offers the quick selector outside every project', () => {
    mockParams = {};
    renderSwitcher();

    expect(screen.queryByLabelText('projects:switcher.label')).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'projects:switcher.select', hidden: true})).toBeInTheDocument();
  });

  it('enters a project chosen from the quick selector', async () => {
    mockParams = {};
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole('button', {name: 'projects:switcher.select', hidden: true}));
    const menu = screen.getByRole('menu');
    expect(within(menu).getByText('Shop')).toBeInTheDocument();
    await user.click(within(menu).getByText('Support'));

    expect(mockNavigate).toHaveBeenCalledWith('/project/support/applications');
  });

  it('offers to create a project first in the quick selector, and opens the creation', async () => {
    mockParams = {};
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole('button', {name: 'projects:switcher.select', hidden: true}));
    const items = within(screen.getByRole('menu')).getAllByRole('menuitem');
    expect(items[0]).toHaveTextContent('projects:switcher.create');
    await user.click(items[0]);

    expect(mockNavigate).toHaveBeenCalledWith('/projects/create');
  });

  it('renders nothing when there are no projects', () => {
    mockUseGetProjects.mockReturnValue({data: {projects: []}});

    const {container} = render(<ProjectSwitcher />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing while the projects are not loaded', () => {
    mockUseGetProjects.mockReturnValue({data: undefined});

    const {container} = render(<ProjectSwitcher />);

    expect(container).toBeEmptyDOMElement();
  });
});
