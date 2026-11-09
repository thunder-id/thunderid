// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import ProjectHomePage from '../ProjectHomePage';

vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => ({selectedProject: {id: 'p-shop', handle: 'shop', displayName: 'Shop'}}),
}));

vi.mock('@thunderid/configure-gateways', () => ({
  GatewaysOverview: ({
    eyebrow = undefined,
    editPipelineUrl = undefined,
  }: {
    eyebrow?: string;
    editPipelineUrl?: string;
  }) => (
    <div data-testid="gateways-overview" data-edit={editPipelineUrl}>
      {eyebrow}
    </div>
  ),
}));

describe('ProjectHomePage', () => {
  it('is the gateways of the project the route names', () => {
    render(<ProjectHomePage />);

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
});
