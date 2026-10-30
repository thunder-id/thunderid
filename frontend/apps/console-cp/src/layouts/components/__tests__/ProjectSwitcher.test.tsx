// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {Header} from '@wso2/oxygen-ui';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {Project} from '../../../features/projects/useGetProjects';
import {PROJECT_SELECTION_STORAGE_KEY} from '../../../features/projects/useProjectSelection';
import ProjectSwitcher from '../ProjectSwitcher';

const mockNavigate = vi.fn();
const mockUseGetProjects = vi.fn<() => {data?: {projects: Project[]}}>();

vi.mock('../../../features/projects/useGetProjects', () => ({
  default: () => mockUseGetProjects(),
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

function project(id: string, displayName: string): Project {
  return {id, handle: id, displayName, rootOuId: `ou-${id}`, createdAt: '2026-10-01T00:00:00Z'};
}

function renderSwitcher(): ReturnType<typeof render> {
  return render(
    <Header>
      <ProjectSwitcher />
    </Header>,
  );
}

describe('ProjectSwitcher', () => {
  beforeEach(() => {
    localStorage.clear();
    mockNavigate.mockReset();
    mockUseGetProjects.mockReturnValue({data: {projects: [project('shop', 'Shop'), project('support', 'Support')]}});
  });

  it('shows the first project as the selected one', () => {
    renderSwitcher();

    expect(screen.getByLabelText('common:header.project')).toHaveTextContent('Shop');
    expect(screen.queryByText('Support')).not.toBeInTheDocument();
  });

  it('switches to another project without leaving the page', async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByLabelText('common:header.project'));
    await user.click(within(screen.getByRole('listbox')).getByText('Support'));

    expect(screen.getByLabelText('common:header.project')).toHaveTextContent('Support');
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('clears the selection and opens the projects page', async () => {
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole('button', {name: 'common:header.clearProject', hidden: true}));

    expect(mockNavigate).toHaveBeenCalledWith('/projects');
    expect(screen.queryByLabelText('common:header.project')).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'common:header.selectProject', hidden: true})).toBeInTheDocument();
  });

  it('enters a project chosen from the quick selector', async () => {
    localStorage.setItem(PROJECT_SELECTION_STORAGE_KEY, '');
    const user = userEvent.setup();
    renderSwitcher();

    await user.click(screen.getByRole('button', {name: 'common:header.selectProject', hidden: true}));
    await user.type(screen.getByRole('textbox', {name: 'common:projects.search'}), 'supp');
    const menu = screen.getByRole('menu');
    expect(within(menu).queryByText('Shop')).not.toBeInTheDocument();
    await user.click(within(menu).getByText('Support'));

    expect(mockNavigate).toHaveBeenCalledWith('/applications');
    expect(screen.getByLabelText('common:header.project')).toHaveTextContent('Support');
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
