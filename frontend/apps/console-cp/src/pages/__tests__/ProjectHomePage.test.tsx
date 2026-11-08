// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useProjectScope} from '@thunderid/contexts';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ProjectHomePage from '../ProjectHomePage';

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useParams: () => ({projectId: 'p-shop'}),
}));

function Overview({eyebrow = undefined, editPipelineUrl = undefined}: {eyebrow?: string; editPipelineUrl?: string}) {
  return (
    <div data-testid="gateways-overview" data-project={useProjectScope().projectId} data-edit={editPipelineUrl}>
      {eyebrow}
    </div>
  );
}

const selectProject = vi.fn();
vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({selectProject, projects: [{id: 'p-shop', handle: 'shop', displayName: 'Shop'}]}),
}));

vi.mock('@thunderid/configure-gateways', () => ({
  useProjects: () => ({data: [{id: 'p-shop', displayName: 'Shop'}]}),
  GatewaysOverview: ({
    eyebrow = undefined,
    editPipelineUrl = undefined,
  }: {
    eyebrow?: string;
    editPipelineUrl?: string;
  }) => <Overview eyebrow={eyebrow} editPipelineUrl={editPipelineUrl} />,
}));

describe('ProjectHomePage', () => {
  beforeEach(() => {
    selectProject.mockReset();
  });

  it('is the gateways of the project, with every deploy from it scoped to the project', () => {
    render(<ProjectHomePage />);

    expect(screen.getByTestId('gateways-overview')).toHaveAttribute('data-project', 'p-shop');
    expect(screen.getByTestId('gateways-overview')).toHaveTextContent('Shop');
  });

  it("links to where the project's pipeline is edited, naming the project by its handle", () => {
    // A first render has the test providers set their whole runtime configuration, which the link adds to.
    render(<div />).unmount();
    const previous = window.__THUNDERID_RUNTIME_CONFIG__;
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      ...previous!,
      deploy: {edit_pipeline_url: 'https://platform.test/projects/{project}/pipeline'},
    };
    render(<ProjectHomePage />);
    window.__THUNDERID_RUNTIME_CONFIG__ = previous;

    expect(screen.getByTestId('gateways-overview')).toHaveAttribute(
      'data-edit',
      'https://platform.test/projects/shop/pipeline',
    );
  });

  it('selects the project it is opened for, so a link opens the console in the project', async () => {
    render(<ProjectHomePage />);

    await waitFor(() => expect(selectProject).toHaveBeenCalledWith('p-shop'));
  });
});
