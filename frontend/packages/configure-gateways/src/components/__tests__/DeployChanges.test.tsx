// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {renderWithProviders, screen, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import type {Change} from '../../models/deploy';
import DeployChanges from '../DeployChanges';
import MissingValuesFields from '../MissingValuesFields';

const changes: Change[] = [
  {
    key: 'application/app-1',
    resourceType: 'application',
    id: 'app-1',
    name: 'Customer portal',
    change: 'updated',
    lines: [
      {kind: '-', text: 'tokenLifetime: 300'},
      {kind: '+', text: 'tokenLifetime: 600'},
    ],
  },
  {key: 'agent/ag-1', resourceType: 'agent', id: 'ag-1', name: 'Support assistant', change: 'added', failed: true},
  {key: 'flow/f-1', resourceType: 'flow', id: 'f-1', name: 'Sign-in with passkey', change: 'updated'},
  {key: 'flow/f-2', resourceType: 'flow', id: 'f-2', name: 'Unchanged flow', change: 'unchanged'},
];

const sectioned: Change[] = [
  {
    key: 'application/shop',
    resourceType: 'application',
    id: 'shop',
    name: 'Shop web',
    change: 'updated',
    section: 'project',
  },
  {
    key: 'flow/login',
    resourceType: 'flow',
    id: 'login',
    name: 'Shop login',
    change: 'added',
    section: 'needed',
    required: true,
  },
  {
    key: 'organization-unit/common',
    resourceType: 'organization-unit',
    id: 'common',
    name: 'Common',
    change: 'updated',
    section: 'needed',
    usedBy: ['billing'],
  },
  {
    key: 'translation/en-US',
    resourceType: 'translation',
    id: 'en-US',
    name: 'en-US',
    change: 'updated',
    section: 'shared',
  },
  {
    key: 'application/bill',
    resourceType: 'application',
    id: 'bill',
    name: 'Billing portal',
    change: 'added',
    section: 'other',
  },
];

describe('DeployChanges', () => {
  it("groups a project's changes by where they stand with it, locking only what the ticked ones need", async () => {
    const onToggle = vi.fn();
    const onToggleAll = vi.fn();
    renderWithProviders(
      <DeployChanges
        changes={sectioned}
        selected={new Set(['application/shop', 'flow/login'])}
        onToggle={onToggle}
        onToggleAll={onToggleAll}
        target="Stage"
        required={new Map([['flow/login', 'application/shop']])}
      />,
    );

    const own = screen.getByTestId('deploy-changes-project');
    expect(within(own).getByText('Project changes · 1')).toBeInTheDocument();
    expect(
      within(own).getByText('Tick the project changes to bring. What they need goes with them.'),
    ).toBeInTheDocument();
    const shop = within(own).getByRole('checkbox', {name: 'Write Shop web'});
    expect(shop).toBeChecked();
    expect(shop).toBeEnabled();
    await userEvent.click(shop);
    expect(onToggle).toHaveBeenCalledWith('application/shop');

    const needed = screen.getByTestId('deploy-changes-needed');
    const login = within(needed).getByTestId('change-flow/login');
    expect(within(login).getByRole('checkbox')).toBeChecked();
    expect(within(login).getByRole('checkbox')).toBeDisabled();
    expect(within(login).getByText('Stage lacks it')).toBeInTheDocument();
    const common = within(needed).getByTestId('change-organization-unit/common');
    expect(within(common).getByRole('checkbox')).not.toBeChecked();
    expect(within(common).getByRole('checkbox')).toBeEnabled();
    expect(within(common).getByTestId('change-used-by')).toHaveTextContent('Also used by billing');

    const shared = screen.getByTestId('deploy-changes-shared');
    await userEvent.click(within(shared).getByRole('checkbox', {name: 'Write en-US'}));
    expect(onToggle).toHaveBeenCalledWith('translation/en-US');
    await userEvent.click(within(shared).getByRole('checkbox', {name: 'Write every one of Shared resources'}));
    expect(onToggleAll).toHaveBeenCalledWith(['translation/en-US'], true);

    const other = screen.getByTestId('deploy-changes-other');
    const bill = within(other).getByRole('checkbox', {name: 'Write Billing portal'});
    expect(bill).not.toBeChecked();
    expect(bill).toBeDisabled();
    expect(within(other).queryByRole('checkbox', {name: /Write every one/})).not.toBeInTheDocument();
    expect(within(own).getByRole('checkbox', {name: 'Write every one of Project changes'})).toBeInTheDocument();
  });

  it('says the target is up to date with the project, and hides other projects, when the project has no changes', () => {
    renderWithProviders(
      <DeployChanges
        changes={sectioned.filter((change) => change.section === 'shared' || change.section === 'other')}
        selected={new Set()}
        onToggle={vi.fn()}
        onToggleAll={vi.fn()}
        target="Stage"
      />,
    );

    const own = screen.getByTestId('deploy-changes-project');
    expect(within(own).getByTestId('deploy-changes-project-none')).toHaveTextContent(
      'Stage is up to date with this project',
    );
    expect(within(own).queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.getByTestId('deploy-changes-shared')).toBeInTheDocument();
    expect(screen.queryByTestId('deploy-changes-other')).not.toBeInTheDocument();
  });

  it("lists the project's changes, then the shared ones, each with its kind and what changed", () => {
    renderWithProviders(
      <DeployChanges
        changes={changes}
        selected={new Set(['application/app-1', 'agent/ag-1', 'flow/f-1'])}
        onToggle={vi.fn()}
        onToggleAll={vi.fn()}
        target="Dev"
      />,
    );

    const project = screen.getByTestId('deploy-changes-project');
    expect(within(project).getByText('Changes · 2')).toBeInTheDocument();
    const app = within(project).getByTestId('change-application/app-1');
    expect(within(app).getByText('Modified')).toBeInTheDocument();
    expect(within(app).getByText(/· Token lifetime$/)).toBeInTheDocument();
    const agent = within(project).getByTestId('change-agent/ag-1');
    expect(within(agent).getByText('Added')).toBeInTheDocument();
    expect(within(agent).getByText('Refused by Dev last time')).toBeInTheDocument();

    const shared = screen.getByTestId('deploy-changes-shared');
    expect(within(shared).getByText('Shared resources · 1')).toBeInTheDocument();
    expect(within(shared).getByText(/ship with the deploy/)).toBeInTheDocument();
    expect(within(shared).getByText('Sign-in with passkey')).toBeInTheDocument();
    expect(screen.getByText('1 resource is already as the version has it.')).toBeInTheDocument();
  });

  it('opens a diff in place and ticks a change or a whole section', async () => {
    const onToggle = vi.fn();
    const onToggleAll = vi.fn();
    renderWithProviders(
      <DeployChanges
        changes={changes}
        selected={new Set(['application/app-1'])}
        onToggle={onToggle}
        onToggleAll={onToggleAll}
        target="Dev"
      />,
    );

    await userEvent.click(screen.getByRole('button', {name: 'Show the changes to Customer portal'}));
    expect(await screen.findByText(/tokenLifetime: 600/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole('checkbox', {name: 'Write Support assistant'}));
    expect(onToggle).toHaveBeenCalledWith('agent/ag-1');
    await userEvent.click(screen.getByRole('checkbox', {name: 'Write every one of Shared resources'}));
    expect(onToggleAll).toHaveBeenCalledWith(['flow/f-1'], true);
  });

  it('says when the target already runs everything', () => {
    renderWithProviders(
      <DeployChanges
        changes={[changes[3]]}
        selected={new Set()}
        onToggle={vi.fn()}
        onToggleAll={vi.fn()}
        target="Dev"
      />,
    );

    expect(screen.getByText('Dev already runs this version as it is.')).toBeInTheDocument();
  });
});

describe('MissingValuesFields for a gateway', () => {
  it('groups the values by resource, names each for a person and folds away', async () => {
    renderWithProviders(
      <MissingValuesFields
        target="Prod"
        fields={[
          {
            kind: 'secret',
            name: 'APP_ONE_CLIENT_SECRET',
            list: false,
            references: [
              {
                name: 'APP_ONE_CLIENT_SECRET',
                kind: 'secret',
                resourceType: 'application',
                resourceId: 'app-1',
                resourceName: 'Customer portal',
                field: 'clientSecret',
              },
            ],
          },
        ]}
        values={{}}
        onChange={vi.fn()}
      />,
    );

    const panel = screen.getByTestId('missing-values');
    expect(within(panel).getByText('Values Prod needs · 1 to set')).toBeInTheDocument();
    const group = within(panel).getByTestId('missing-group-application/app-1');
    expect(within(group).getByText(/^Customer portal · /)).toBeInTheDocument();
    expect(within(group).getByLabelText(/Client secret/)).toBeInTheDocument();
    expect(within(group).getByText('APP_ONE_CLIENT_SECRET')).toBeInTheDocument();

    await userEvent.click(within(panel).getByRole('button', {name: /Values Prod needs/}));
    expect(within(panel).getByRole('button', {name: /Values Prod needs/})).toHaveAttribute('aria-expanded', 'false');
  });
});
