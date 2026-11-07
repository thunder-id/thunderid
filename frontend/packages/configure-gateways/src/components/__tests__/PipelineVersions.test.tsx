// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import PipelineVersions from '../PipelineVersions';

const versions = Array.from({length: 7}, (_, index: number) => ({
  version: `hash-${7 - index}`,
  number: 7 - index,
  createdAt: '2026-11-01T10:00:00Z',
}));

describe('PipelineVersions', () => {
  it('offers every version to deploy, the latest first, and marks what the gateways run', async () => {
    const onDeploy = vi.fn();
    render(
      <PipelineVersions
        versions={versions}
        runners={{'hash-5': [{name: 'Dev', index: 0}], 'hash-3': [{name: 'Prod', index: 1}]}}
        firstRuns="hash-5"
        firstName="Dev"
        onDeploy={onDeploy}
      />,
    );

    const latest = screen.getByTestId('pipeline-version-hash-7');
    expect(within(latest).getByText(/^Latest/)).toBeInTheDocument();
    const deployLatest = within(latest).getByRole('button', {name: 'Deploy'});
    expect(deployLatest).toHaveClass('MuiButton-contained');
    expect(within(screen.getByTestId('pipeline-version-hash-6')).getByRole('button', {name: 'Deploy'})).toHaveClass(
      'MuiButton-outlined',
    );
    const running = screen.getByTestId('pipeline-version-hash-5');
    expect(within(running).getByText('Dev')).toBeInTheDocument();
    // What the first gateway runs can be deployed again, and an older version to go back to it.
    expect(within(running).getByRole('button', {name: 'Deploy'})).toHaveClass('MuiButton-outlined');
    expect(
      within(screen.getByTestId('pipeline-version-hash-3')).getByRole('button', {name: 'Deploy'}),
    ).toBeInTheDocument();

    await userEvent.click(deployLatest);
    expect(onDeploy).toHaveBeenCalledWith(versions[0]);
  });

  it('shows five versions, and every version in a dialog when asked', async () => {
    const onDeploy = vi.fn();
    render(<PipelineVersions versions={versions} runners={{}} firstName="Dev" onDeploy={onDeploy} />);

    expect(screen.queryByTestId('pipeline-version-hash-2')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'View all versions'}));
    const dialog = await screen.findByRole('dialog', {name: 'All versions'});
    const oldest = within(dialog).getByTestId('pipeline-version-hash-1');
    await userEvent.click(within(oldest).getByRole('button', {name: 'Deploy'}));
    expect(onDeploy).toHaveBeenCalledWith(expect.objectContaining({version: 'hash-1'}));
  });

  it('counts the changes the latest version brings beside it', () => {
    render(<PipelineVersions versions={versions} runners={{}} firstName="Dev" latestChanges={3} onDeploy={vi.fn()} />);

    expect(
      within(screen.getByTestId('pipeline-version-hash-7')).getByText(/^Latest · .* · 3 changes$/),
    ).toBeInTheDocument();
  });

  it('says when nothing is captured yet', () => {
    render(<PipelineVersions versions={[]} runners={{}} firstName="Dev" onDeploy={vi.fn()} />);

    expect(screen.getByText(/Nothing is captured yet/)).toBeInTheDocument();
  });

  it("keeps the latest's Deploy the main one while the first gateway runs it, the others outlined", () => {
    render(<PipelineVersions versions={versions} runners={{}} firstRuns="hash-7" firstName="Dev" onDeploy={vi.fn()} />);

    expect(within(screen.getByTestId('pipeline-version-hash-7')).getByRole('button', {name: 'Deploy'})).toHaveClass(
      'MuiButton-contained',
    );
    expect(within(screen.getByTestId('pipeline-version-hash-6')).getByRole('button', {name: 'Deploy'})).toHaveClass(
      'MuiButton-outlined',
    );
  });
});
