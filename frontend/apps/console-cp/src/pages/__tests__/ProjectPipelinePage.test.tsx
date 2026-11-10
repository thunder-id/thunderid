// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, expect, it, vi} from 'vitest';
import ProjectPipelinePage from '../ProjectPipelinePage';

vi.mock('../../components/AutoDeploySwitch', () => ({default: () => <span data-testid="auto-deploy" />}));

vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({selectedProject: {id: 'p-shop', handle: 'shop', displayName: 'Shop'}}),
}));

vi.mock('@thunderid/configure-gateways', () => ({
  GatewaysOverview: ({
    editPipelineUrl = undefined,
    pipelineTitle = undefined,
    showIntro = true,
    pipelineActions = undefined,
  }: {
    editPipelineUrl?: string;
    pipelineTitle?: string;
    showIntro?: boolean;
    pipelineActions?: ReactNode;
  }) => (
    <div data-testid="gateways-overview" data-edit={editPipelineUrl} data-intro={String(showIntro)}>
      <h2>{pipelineTitle}</h2>
      {pipelineActions}
    </div>
  ),
}));

describe('ProjectPipelinePage', () => {
  it("is the project's pipeline alone, with the auto deploy switch over its gateways", () => {
    render(<ProjectPipelinePage />);

    expect(screen.getByTestId('gateways-overview')).toHaveAttribute('data-intro', 'false');
    expect(screen.getByRole('heading', {name: 'Project deployment pipeline'})).toBeInTheDocument();
    expect(screen.getByTestId('auto-deploy')).toBeInTheDocument();
  });

  it("links to where the project's pipeline is edited, naming the project by its handle", () => {
    // A first render has the test providers set their whole runtime configuration, which the link adds to.
    render(<div />).unmount();
    const previous = window.__THUNDERID_RUNTIME_CONFIG__;
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      ...previous!,
      deploy: {edit_pipeline_url: 'https://platform.test/projects/{project}/pipeline'},
    };
    render(<ProjectPipelinePage />);
    window.__THUNDERID_RUNTIME_CONFIG__ = previous;

    expect(screen.getByTestId('gateways-overview')).toHaveAttribute(
      'data-edit',
      'https://platform.test/projects/shop/pipeline',
    );
  });
});
