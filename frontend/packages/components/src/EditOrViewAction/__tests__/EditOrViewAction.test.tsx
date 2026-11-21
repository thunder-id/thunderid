// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import EditOrViewAction from '../EditOrViewAction';

describe('EditOrViewAction', () => {
  it('offers Edit to an admin who may update', () => {
    render(<EditOrViewAction canEdit onClick={vi.fn()} />);

    expect(screen.getByRole('button', {name: 'Edit'})).toBeInTheDocument();
  });

  it('offers View to an admin who may only view', () => {
    render(<EditOrViewAction canEdit={false} onClick={vi.fn()} editLabel="Edit unit" />);

    expect(screen.getByRole('button', {name: 'View'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Edit unit'})).not.toBeInTheDocument();
  });

  it('opens the row without the click reaching the row', () => {
    const onClick = vi.fn();
    const onRowClick = vi.fn();
    render(
      // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
      <div onClick={onRowClick}>
        <EditOrViewAction canEdit onClick={onClick} editLabel="Edit unit" />
      </div>,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Edit unit'}));

    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onRowClick).not.toHaveBeenCalled();
  });
});
