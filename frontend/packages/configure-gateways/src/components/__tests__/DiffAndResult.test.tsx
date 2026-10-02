// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import type {ApplyResult, GatewayDiff} from '../../models/gateway';
import ApplyResultSummary from '../ApplyResultSummary';
import DiffSummaryChips from '../DiffSummaryChips';
import MissingValuesNotice from '../MissingValuesNotice';
import ResourceDiffList from '../ResourceDiffList';

const diff: GatewayDiff = {
  fromVersion: 1,
  toVersion: 2,
  summary: {added: 1, updated: 1, deleted: 1, unchanged: 3},
  changes: [
    {resourceType: 'application', id: 'app-1', name: 'Payments', change: 'added'},
    {resourceType: 'application', id: 'app-2', name: 'Legacy', change: 'deleted'},
    {resourceType: 'flow', id: 'flow-1', change: 'updated'},
    {resourceType: 'flow', id: 'flow-2', name: 'Same', change: 'unchanged'},
  ],
};

describe('DiffSummaryChips', () => {
  it('shows a count for each kind of change', () => {
    renderWithProviders(<DiffSummaryChips summary={diff.summary} />);

    expect(screen.getByText('1 added')).toBeInTheDocument();
    expect(screen.getByText('1 updated')).toBeInTheDocument();
    expect(screen.getByText('1 deleted')).toBeInTheDocument();
    expect(screen.getByText('3 unchanged')).toBeInTheDocument();
  });
});

describe('ResourceDiffList', () => {
  it('groups changes by resource type and leaves unchanged resources out', () => {
    renderWithProviders(<ResourceDiffList diff={diff} />);

    expect(screen.getByText('application')).toBeInTheDocument();
    expect(screen.getByText('flow')).toBeInTheDocument();
    expect(screen.getByText('Payments')).toBeInTheDocument();
    expect(screen.getByText('flow-1')).toBeInTheDocument();
    expect(screen.queryByText('Same')).not.toBeInTheDocument();
  });

  it('marks a deletion so it stands out', () => {
    const {container} = renderWithProviders(<ResourceDiffList diff={diff} />);

    const deleted = container.querySelector('[data-change="deleted"]');
    expect(deleted).not.toBeNull();
    expect(deleted).toHaveTextContent('Deleted');
    expect(deleted).toHaveTextContent('Legacy');
  });

  it('says nothing would change when every resource matches', () => {
    renderWithProviders(
      <ResourceDiffList
        diff={{toVersion: 1, summary: {added: 0, updated: 0, deleted: 0, unchanged: 1}, changes: []}}
      />,
    );

    expect(screen.getByText(/Nothing would change/)).toBeInTheDocument();
  });
});

describe('ApplyResultSummary', () => {
  const base: ApplyResult = {
    gatewayId: 'gw-1',
    dryRun: false,
    recorded: true,
    import: {summary: {totalDocuments: 4, imported: 4, deleted: 1, failed: 0}, results: []},
  };

  it('confirms an apply the gateway took whole', () => {
    renderWithProviders(<ApplyResultSummary result={base} />);

    expect(screen.getByText('The gateway now holds this configuration.')).toBeInTheDocument();
    expect(screen.getByText('4 documents')).toBeInTheDocument();
    expect(screen.getByText('4 imported')).toBeInTheDocument();
    expect(screen.getByText('1 deleted')).toBeInTheDocument();
    expect(screen.getByText('0 failed')).toBeInTheDocument();
  });

  it('says a dry run changed nothing', () => {
    renderWithProviders(<ApplyResultSummary result={{...base, dryRun: true, recorded: false}} />);

    expect(screen.getByText(/Dry run finished. Nothing was changed/)).toBeInTheDocument();
  });

  it('lists each resource the gateway refused', () => {
    renderWithProviders(
      <ApplyResultSummary
        result={{
          ...base,
          import: {
            summary: {totalDocuments: 2, imported: 1, failed: 1},
            results: [
              {resourceType: 'application', resourceName: 'Payments', status: 'success'},
              {
                resourceType: 'flow',
                resourceId: 'flow-1',
                status: 'failed',
                code: 'IMP-1001',
                message: 'Flow is invalid',
              },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText('Applied, but the gateway refused some resources.')).toBeInTheDocument();
    expect(screen.getByText('Resources the gateway refused')).toBeInTheDocument();
    expect(screen.getByText('flow-1')).toBeInTheDocument();
    expect(screen.getByText('flow (IMP-1001)')).toBeInTheDocument();
    expect(screen.getByText('Flow is invalid')).toBeInTheDocument();
    expect(screen.queryByText('Payments')).not.toBeInTheDocument();
  });

  it('warns that a dry run would be refused in part', () => {
    renderWithProviders(
      <ApplyResultSummary
        result={{
          ...base,
          dryRun: true,
          recorded: false,
          import: {summary: {totalDocuments: 1, imported: 0, failed: 1}},
        }}
      />,
    );

    expect(screen.getByText(/The gateway would refuse some resources/)).toBeInTheDocument();
  });
});

describe('ApplyResultSummary missing values', () => {
  it('names what a dry run found missing and leads to where it is set', async () => {
    const user = userEvent.setup();
    const onManage = vi.fn();
    renderWithProviders(
      <ApplyResultSummary
        result={{gatewayId: 'gw-1', dryRun: true, recorded: false, missing: {variables: ['API_URL'], secrets: []}}}
        onManage={onManage}
      />,
    );

    expect(screen.getByText('1 variable has no value')).toBeInTheDocument();
    expect(screen.queryByText(/secret is not configured/)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Manage variables'}));
    expect(onManage).toHaveBeenCalledWith('variables');
  });
});

describe('MissingValuesNotice', () => {
  it('counts the missing secrets and variables', () => {
    renderWithProviders(
      <MissingValuesNotice missing={{variables: ['A', 'B'], secrets: ['S1', 'S2', 'S3']}} onManageSecrets={vi.fn()} />,
    );

    expect(screen.getByText('3 secrets are not configured')).toBeInTheDocument();
    expect(screen.getByText('2 variables have no value')).toBeInTheDocument();
    expect(screen.getByText(/Applying now would create resources whose credentials fail/)).toBeInTheDocument();
    expect(screen.getByText(/Applying now would leave these fields empty/)).toBeInTheDocument();
    ['A', 'B', 'S1', 'S2', 'S3'].forEach((name: string) => expect(screen.getByText(name)).toBeInTheDocument());
    expect(screen.getByRole('button', {name: 'Manage secrets'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Manage variables'})).not.toBeInTheDocument();
  });

  it('shows nothing when nothing is missing', () => {
    renderWithProviders(<MissingValuesNotice missing={{variables: [], secrets: []}} />);

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
