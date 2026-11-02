// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import KeyValueEditor from '../KeyValueEditor';

describe('KeyValueEditor', () => {
  const defaultProps = {
    entries: [] as [string, string][],
    onAdd: vi.fn(),
    onRemove: vi.fn(),
    onKeyChange: vi.fn(),
    onValueChange: vi.fn(),
    keyPlaceholder: 'Key',
    valuePlaceholder: 'Value',
    keyLabel: 'Key Column',
    valueLabel: 'Value Column',
    addLabel: 'Add Entry',
    removeLabel: 'Remove Entry',
  };

  // With no rows there is nothing but this button, so it has to say what it adds.
  it('should render the named add button when entries are empty', () => {
    render(<KeyValueEditor {...defaultProps} />);

    expect(screen.getByRole('button', {name: 'Add Entry'})).toBeInTheDocument();
  });

  it('should call onAdd when add button is clicked', async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn();

    render(<KeyValueEditor {...defaultProps} onAdd={onAdd} />);

    await user.click(screen.getByRole('button', {name: 'Add Entry'}));

    expect(onAdd).toHaveBeenCalledTimes(1);
  });

  it('should render entries with key and value fields', () => {
    const entries: [string, string][] = [
      ['Content-Type', 'application/json'],
      ['Authorization', 'Bearer token'],
    ];

    render(<KeyValueEditor {...defaultProps} entries={entries} />);

    const textboxes = screen.getAllByRole('textbox');
    expect(textboxes).toHaveLength(4);
    expect(textboxes[0]).toHaveValue('Content-Type');
    expect(textboxes[1]).toHaveValue('application/json');
    expect(textboxes[2]).toHaveValue('Authorization');
    expect(textboxes[3]).toHaveValue('Bearer token');
  });

  it('should call onKeyChange when a key field is edited and blurred', () => {
    const onKeyChange = vi.fn();
    const entries: [string, string][] = [['oldKey', 'val']];

    render(<KeyValueEditor {...defaultProps} entries={entries} onKeyChange={onKeyChange} />);

    const textboxes = screen.getAllByRole('textbox');
    fireEvent.change(textboxes[0], {target: {value: 'newKey'}});
    fireEvent.blur(textboxes[0]);

    expect(onKeyChange).toHaveBeenCalledWith(0, 'newKey');
  });

  it('should not call onKeyChange on blur when value is unchanged', () => {
    const onKeyChange = vi.fn();
    const entries: [string, string][] = [['sameKey', 'val']];

    render(<KeyValueEditor {...defaultProps} entries={entries} onKeyChange={onKeyChange} />);

    const textboxes = screen.getAllByRole('textbox');
    fireEvent.blur(textboxes[0]);

    expect(onKeyChange).not.toHaveBeenCalled();
  });

  it('should call onValueChange when a value field is edited and blurred', () => {
    const onValueChange = vi.fn();
    const entries: [string, string][] = [['key', 'oldVal']];

    render(<KeyValueEditor {...defaultProps} entries={entries} onValueChange={onValueChange} />);

    const textboxes = screen.getAllByRole('textbox');
    fireEvent.change(textboxes[1], {target: {value: 'newVal'}});
    fireEvent.blur(textboxes[1]);

    expect(onValueChange).toHaveBeenCalledWith(0, 'newVal');
  });

  it('should call onRemove when remove button is clicked', async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    const entries: [string, string][] = [['key', 'val']];

    render(<KeyValueEditor {...defaultProps} entries={entries} onRemove={onRemove} />);

    await user.click(screen.getByLabelText('Remove Entry'));

    expect(onRemove).toHaveBeenCalledWith(0);
  });

  it('should render one remove button per entry', () => {
    const entries: [string, string][] = [
      ['a', '1'],
      ['b', '2'],
      ['c', '3'],
    ];

    render(<KeyValueEditor {...defaultProps} entries={entries} />);

    expect(screen.getAllByLabelText('Remove Entry')).toHaveLength(3);
  });

  // The fields repeat, so they are headed once and named for a screen reader on every row.
  it('should head the columns only when there are rows to head', () => {
    const {rerender} = render(<KeyValueEditor {...defaultProps} />);

    expect(screen.queryByText('Key Column')).not.toBeInTheDocument();

    rerender(<KeyValueEditor {...defaultProps} entries={[['a', '1']]} />);

    expect(screen.getByText('Key Column')).toBeInTheDocument();
    expect(screen.getByText('Value Column')).toBeInTheDocument();
  });

  it('should name each row field for assistive technology', () => {
    render(<KeyValueEditor {...defaultProps} entries={[['a', '1']]} />);

    expect(screen.getByLabelText('Key Column')).toHaveValue('a');
    expect(screen.getByLabelText('Value Column')).toHaveValue('1');
  });

  it('should use provided placeholders', () => {
    const entries: [string, string][] = [['', '']];

    render(
      <KeyValueEditor
        {...defaultProps}
        entries={entries}
        keyPlaceholder="Header Name"
        valuePlaceholder="Header Value"
      />,
    );

    expect(screen.getByPlaceholderText('Header Name')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Header Value')).toBeInTheDocument();
  });
});
