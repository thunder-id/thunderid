// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import type {Change} from '../../models/deploy';
import DeployedResources from '../DeployedResources';

const app = (id: string, change: Change['change'], extra: Partial<Change> = {}): Change => ({
  key: `application/${id}`,
  resourceType: 'application',
  id,
  name: id,
  change,
  ...extra,
});

function show(changes: Change[], isShared?: (change: Change) => boolean) {
  return render(
    <DeployedResources
      changes={changes}
      isShared={isShared}
      runningOf={() => 'v1'}
      reasonOf={(change: Change) => (change.failed ? 'Missing executor' : undefined)}
      actionOf={() => null}
    />,
  );
}

describe('DeployedResources', () => {
  it('shows what needs attention first, and the rest on request', async () => {
    show([app('a', 'unchanged'), app('b', 'added'), app('c', 'updated', {failed: true})]);

    expect(screen.getByTestId('gateway-filter-attention')).toHaveTextContent('Needs attention · 2');
    expect(screen.queryByTestId('gateway-resource-application/a')).not.toBeInTheDocument();
    expect(screen.getByTestId('gateway-resource-application/c')).toHaveTextContent('Missing executor');
    await userEvent.click(screen.getByTestId('gateway-filter-inSync'));
    expect(screen.getByTestId('gateway-resource-application/a')).toBeInTheDocument();
    expect(screen.queryByTestId('gateway-resource-application/b')).not.toBeInTheDocument();
    await userEvent.click(screen.getByTestId('gateway-filter-all'));
    expect(screen.getByTestId('gateway-resource-application/b')).toBeInTheDocument();
  });

  it('shows everything when nothing needs attention', () => {
    show([app('a', 'unchanged')]);

    expect(screen.getByTestId('gateway-filter-all')).toHaveAttribute('aria-pressed', 'true');
  });

  it('outlines every filter, and marks the selected one with a check', async () => {
    show([app('a', 'unchanged'), app('b', 'added')]);

    const attention = screen.getByTestId('gateway-filter-attention');
    const all = screen.getByTestId('gateway-filter-all');
    for (const chip of [attention, all]) expect(chip).toHaveClass('MuiChip-outlined');
    expect(attention.querySelector('svg')).toBeInTheDocument();
    expect(all.querySelector('svg')).not.toBeInTheDocument();
    await userEvent.click(all);
    expect(all.querySelector('svg')).toBeInTheDocument();
    expect(attention.querySelector('svg')).not.toBeInTheDocument();
  });

  it('groups by kind with a summary, open only where something needs attention', () => {
    show([
      app('a', 'added'),
      app('b', 'updated', {failed: true}),
      {key: 'flow/f', resourceType: 'flow', id: 'f', name: 'login', change: 'unchanged'},
    ]);
    // Everything is shown, so the in-sync flows group is there, closed.
    const apps = screen.getByTestId('gateway-group-applications');
    expect(apps).toHaveTextContent('Applications · 2');
    expect(apps).toHaveTextContent('1 not deployed, 1 refused');
    expect(within(apps).getByRole('button', {expanded: true})).toBeInTheDocument();
  });

  it('closes a group whose resources are all in sync', async () => {
    show([app('a', 'added'), {key: 'flow/f', resourceType: 'flow', id: 'f', name: 'login', change: 'unchanged'}]);
    await userEvent.click(screen.getByTestId('gateway-filter-all'));

    expect(
      within(screen.getByTestId('gateway-group-flows')).getByRole('button', {expanded: false}),
    ).toBeInTheDocument();
  });

  it('lists shared resources apart', () => {
    show([app('a', 'added'), app('s', 'added')], (change: Change) => change.id === 's');

    const shared = screen.getByTestId('gateway-group-shared');
    expect(within(shared).getByTestId('gateway-resource-application/s')).toBeInTheDocument();
    expect(
      within(screen.getAllByTestId('gateway-group-applications')[0]).getByTestId('gateway-resource-application/a'),
    ).toBeInTheDocument();
  });

  it('searches by name when there are many, and shows long groups a page at a time', async () => {
    const many = Array.from({length: 14}, (_, i) => app(`app-${i}`, 'added'));
    show(many);

    expect(screen.getAllByTestId(/^gateway-resource-/)).toHaveLength(10);
    await userEvent.click(screen.getByRole('button', {name: 'Show 4 more'}));
    expect(screen.getAllByTestId(/^gateway-resource-/)).toHaveLength(14);
    await userEvent.type(screen.getByRole('textbox', {name: 'Search by name'}), 'app-1');
    // app-1, app-10 … app-13
    expect(screen.getAllByTestId(/^gateway-resource-/)).toHaveLength(5);
  });
});
