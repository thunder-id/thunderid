// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import ReadOnlyFieldset from '../ReadOnlyFieldset';

function renderSelect(readOnly: boolean, onKeyDown: () => void): HTMLElement {
  render(
    <ReadOnlyFieldset readOnly={readOnly}>
      <div
        className="MuiSelect-select"
        role="combobox"
        aria-controls="menu"
        aria-expanded="false"
        tabIndex={0}
        onKeyDown={onKeyDown}
      >
        Google
      </div>
      <input aria-label="Name" />
    </ReadOnlyFieldset>,
  );
  return screen.getByRole('combobox');
}

describe('ReadOnlyFieldset', () => {
  it('disables the inputs of a locked section', () => {
    renderSelect(true, vi.fn());

    expect(screen.getByRole('textbox', {name: 'Name'})).toBeDisabled();
  });

  it('keeps the keyboard from opening a Select in a locked section', () => {
    const onKeyDown = vi.fn();
    fireEvent.keyDown(renderSelect(true, onKeyDown), {key: 'ArrowDown'});

    expect(onKeyDown).not.toHaveBeenCalled();
  });

  it('still lets Tab move focus past a locked Select', () => {
    const onKeyDown = vi.fn();
    fireEvent.keyDown(renderSelect(true, onKeyDown), {key: 'Tab'});

    expect(onKeyDown).toHaveBeenCalled();
  });

  it('leaves an unlocked section alone', () => {
    const onKeyDown = vi.fn();
    fireEvent.keyDown(renderSelect(false, onKeyDown), {key: 'ArrowDown'});

    expect(onKeyDown).toHaveBeenCalled();
    expect(screen.getByRole('textbox', {name: 'Name'})).toBeEnabled();
  });
});
