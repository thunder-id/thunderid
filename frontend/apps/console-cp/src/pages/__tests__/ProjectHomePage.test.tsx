// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {RoutesProvider} from '@thunderid/contexts';
import {render, screen, userEvent} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, expect, it, vi} from 'vitest';
import {projectRouteConfig} from '../../configs/RouteConfig';
import ProjectHomePage from '../ProjectHomePage';

const navigate = vi.fn();
vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigate,
}));

/** The home as the project's route renders it: every link it makes stays in the project. */
function renderInShop(): void {
  render(
    <RoutesProvider paths={projectRouteConfig('shop')}>
      <ProjectHomePage />
    </RoutesProvider>,
  );
}

vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({selectedProject: {id: 'p-shop', handle: 'shop', displayName: 'Shop'}}),
}));

vi.mock('@thunderid/configure-gateways', async (importOriginal) => ({
  gatewayColor: (await importOriginal<typeof import('@thunderid/configure-gateways')>()).gatewayColor,
  GatewaysOverview: ({
    eyebrow = undefined,
    beforePipeline = undefined,
    showPipeline = true,
  }: {
    eyebrow?: string;
    beforePipeline?: ReactNode;
    showPipeline?: boolean;
  }) => (
    <div data-testid="gateways-overview" data-pipeline={String(showPipeline)}>
      {eyebrow}
      {beforePipeline}
    </div>
  ),
}));

describe('ProjectHomePage', () => {
  it('is the overview of the project the route names, leaving the pipeline to its own page', () => {
    renderInShop();

    expect(screen.getByTestId('gateways-overview')).toHaveTextContent('Shop');
    expect(screen.getByTestId('gateways-overview')).toHaveAttribute('data-pipeline', 'false');
  });

  it('offers four kinds of resource at once, in one row, by their names alone', () => {
    renderInShop();

    const onboard = screen.getByTestId('project-onboard');
    expect(onboard.querySelectorAll('[data-testid^="project-onboard-"]')).toHaveLength(4);
    expect(screen.getByText('Web application')).toBeInTheDocument();
    expect(screen.queryByText('Sign users in with OpenID Connect')).not.toBeInTheDocument();
  });

  it("opens a kind's setup inside the project", async () => {
    renderInShop();

    await userEvent.click(screen.getByText('Web application'));

    expect(navigate).toHaveBeenCalledWith('/project/shop/applications/types?type=FULL_STACK');
  });
});
