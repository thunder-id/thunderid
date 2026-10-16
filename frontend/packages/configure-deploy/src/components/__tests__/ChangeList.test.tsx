// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import type {Change, LineOp} from '../../models/deploy';
import ChangeList from '../ChangeList';

const same = (count: number, from = 0): LineOp[] =>
  Array.from({length: count}, (_value: unknown, index: number) => ({kind: ' ', text: `line ${String(from + index)}`}));

const changes: Change[] = [
  {
    key: 'application/a1',
    resourceType: 'application',
    id: 'a1',
    name: 'Orders',
    change: 'updated',
    lines: [...same(10), {kind: '-', text: 'url: old'}, {kind: '+', text: 'url: new'}, ...same(2, 10)],
  },
  {key: 'role/r1', resourceType: 'role', id: 'r1', name: 'Admin', change: 'added'},
  {key: 'group/g1', resourceType: 'group', id: 'g1', name: 'Ops', change: 'unchanged'},
];

describe('ChangeList', () => {
  it('opens a change to its lines, with the unchanged stretch folded away', async () => {
    render(
      <ChangeList changes={changes} selected={new Set(['application/a1'])} onToggle={vi.fn()} onToggleAll={vi.fn()} />,
    );

    expect(screen.queryByTestId('change-lines')).not.toBeInTheDocument();
    expect(
      within(screen.getByTestId('change-role/r1')).queryByRole('button', {name: /Show the changes/}),
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Show the changes to Orders'}));

    const lines = await screen.findByTestId('change-lines');
    expect(within(lines).getByText('- url: old')).toHaveAttribute('data-kind', '-');
    expect(within(lines).getByText('+ url: new')).toHaveAttribute('data-kind', '+');
    expect(within(lines).getByText('7 unchanged lines')).toBeInTheDocument();
    expect(within(lines).queryByText('  line 0')).not.toBeInTheDocument();
    expect(screen.getByText('1 resources are already as the version has them.')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Hide the changes to Orders'}));
    expect(screen.getByRole('button', {name: 'Show the changes to Orders'})).toHaveAttribute('aria-expanded', 'false');
  });

  it('lists the changes to be read, without checkboxes, when nothing is selectable', () => {
    render(<ChangeList changes={changes} />);

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.getByText('Orders')).toBeInTheDocument();
    expect(screen.getByText('Admin')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Show the changes to Orders'})).toBeInTheDocument();
  });

  it('toggles a change and all of them when selectable', async () => {
    const onToggle = vi.fn();
    const onToggleAll = vi.fn();
    render(<ChangeList changes={changes} selected={new Set()} onToggle={onToggle} onToggleAll={onToggleAll} />);

    const boxes = screen.getAllByRole('checkbox');
    await userEvent.click(boxes[0]);
    await userEvent.click(boxes[2]);
    expect(onToggleAll).toHaveBeenCalledWith(['application/a1', 'role/r1'], true);
    expect(onToggle).toHaveBeenCalledWith('role/r1');
  });

  it('opens a change from anywhere on its row, while only the checkbox selects it', async () => {
    const onToggle = vi.fn();
    render(<ChangeList changes={changes} selected={new Set()} onToggle={onToggle} onToggleAll={vi.fn()} />);

    await userEvent.click(screen.getByText('Orders'));

    expect(await screen.findByTestId('change-lines')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Hide the changes to Orders'})).toHaveAttribute('aria-expanded', 'true');
    expect(onToggle).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole('checkbox', {name: 'Write Orders'}));
    expect(onToggle).toHaveBeenCalledWith('application/a1');
    expect(screen.getByRole('button', {name: 'Hide the changes to Orders'})).toBeInTheDocument();

    await userEvent.click(screen.getByTestId('change-row-application/a1'));
    expect(screen.getByRole('button', {name: 'Show the changes to Orders'})).toHaveAttribute('aria-expanded', 'false');
  });

  it('opens nothing from the row of a change without lines', async () => {
    render(<ChangeList changes={changes} />);

    await userEvent.click(screen.getByText('Admin'));

    expect(screen.queryByTestId('change-lines')).not.toBeInTheDocument();
  });
});
