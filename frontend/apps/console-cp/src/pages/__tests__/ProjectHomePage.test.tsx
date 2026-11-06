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

function Overview({eyebrow = undefined}: {eyebrow?: string}) {
  return (
    <div data-testid="gateways-overview" data-project={useProjectScope().projectId}>
      {eyebrow}
    </div>
  );
}

const selectProject = vi.fn();
vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({selectProject}),
}));

vi.mock('@thunderid/configure-gateways', () => ({
  useProjects: () => ({data: [{id: 'p-shop', displayName: 'Shop'}]}),
  GatewaysOverview: ({eyebrow = undefined}: {eyebrow?: string}) => <Overview eyebrow={eyebrow} />,
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

  it('selects the project it is opened for, so a link opens the console in the project', async () => {
    render(<ProjectHomePage />);

    await waitFor(() => expect(selectProject).toHaveBeenCalledWith('p-shop'));
  });
});
