// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import DraftTextField from '../DraftTextField';
import {clampToInteger} from '../utils';

describe('DraftTextField', () => {
  const mockOnCommit = vi.fn();

  beforeEach(() => {
    mockOnCommit.mockClear();
  });

  it('should render the committed value', () => {
    render(<DraftTextField value="initial" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    expect(screen.getByLabelText('field')).toHaveValue('initial');
  });

  it('should show keystrokes without committing them', () => {
    render(<DraftTextField value="" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    const input = screen.getByLabelText('field');
    fireEvent.change(input, {target: {value: 'a'}});
    fireEvent.change(input, {target: {value: 'ab'}});
    fireEvent.change(input, {target: {value: 'abc'}});

    expect(input).toHaveValue('abc');
    expect(mockOnCommit).not.toHaveBeenCalled();
  });

  it('should commit on blur', () => {
    render(<DraftTextField value="" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    const input = screen.getByLabelText('field');
    fireEvent.change(input, {target: {value: 'abc'}});
    fireEvent.blur(input);

    expect(mockOnCommit).toHaveBeenCalledExactlyOnceWith('abc');
  });

  it('should commit on Enter', () => {
    render(<DraftTextField value="" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    const input = screen.getByLabelText('field');
    fireEvent.change(input, {target: {value: 'abc'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(mockOnCommit).toHaveBeenCalledExactlyOnceWith('abc');
  });

  it('should not commit on Enter for a multiline field', () => {
    render(<DraftTextField value="" onCommit={mockOnCommit} multiline inputProps={{'aria-label': 'field'}} />);

    const input = screen.getByLabelText('field');
    fireEvent.change(input, {target: {value: 'abc'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(mockOnCommit).not.toHaveBeenCalled();
  });

  it('should not commit an unchanged value on blur', () => {
    render(<DraftTextField value="initial" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    fireEvent.blur(screen.getByLabelText('field'));

    expect(mockOnCommit).not.toHaveBeenCalled();
  });

  it('should not commit twice when Enter is followed by blur', () => {
    render(<DraftTextField value="" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    const input = screen.getByLabelText('field');
    fireEvent.change(input, {target: {value: 'abc'}});
    fireEvent.keyDown(input, {key: 'Enter'});
    fireEvent.blur(input);

    expect(mockOnCommit).toHaveBeenCalledExactlyOnceWith('abc');
  });

  it('should commit a pending edit when the field unmounts while focused', () => {
    const {unmount} = render(<DraftTextField value="" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    // No blur: removing a focused input does not reliably fire one.
    fireEvent.change(screen.getByLabelText('field'), {target: {value: 'abc'}});
    unmount();

    expect(mockOnCommit).toHaveBeenCalledExactlyOnceWith('abc');
  });

  it('should not commit on unmount when the draft is unchanged', () => {
    const {unmount} = render(
      <DraftTextField value="initial" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />,
    );

    unmount();

    expect(mockOnCommit).not.toHaveBeenCalled();
  });

  it('should not commit again on unmount after a blur commit', () => {
    const {unmount} = render(<DraftTextField value="" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    const input = screen.getByLabelText('field');
    fireEvent.change(input, {target: {value: 'abc'}});
    fireEvent.blur(input);
    unmount();

    expect(mockOnCommit).toHaveBeenCalledExactlyOnceWith('abc');
  });

  it('should not commit a rejected draft on unmount', () => {
    const {unmount} = render(
      <DraftTextField
        value="10"
        onCommit={mockOnCommit}
        normalize={(raw) => clampToInteger(raw, 1, 20)}
        inputProps={{'aria-label': 'field'}}
      />,
    );

    fireEvent.change(screen.getByLabelText('field'), {target: {value: 'abc'}});
    unmount();

    expect(mockOnCommit).not.toHaveBeenCalled();
  });

  it('should commit the normalized value on unmount', () => {
    const {unmount} = render(
      <DraftTextField
        value="10"
        onCommit={mockOnCommit}
        normalize={(raw) => clampToInteger(raw, 1, 20)}
        inputProps={{'aria-label': 'field'}}
      />,
    );

    fireEvent.change(screen.getByLabelText('field'), {target: {value: '99'}});
    unmount();

    expect(mockOnCommit).toHaveBeenCalledExactlyOnceWith('20');
  });

  it('should re-sync when the committed value changes externally', () => {
    const {rerender} = render(
      <DraftTextField value="first" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />,
    );

    rerender(<DraftTextField value="second" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    expect(screen.getByLabelText('field')).toHaveValue('second');
  });

  it('should discard an uncommitted draft when the committed value changes externally', () => {
    const {rerender} = render(
      <DraftTextField value="first" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />,
    );

    fireEvent.change(screen.getByLabelText('field'), {target: {value: 'typing'}});
    rerender(<DraftTextField value="second" onCommit={mockOnCommit} inputProps={{'aria-label': 'field'}} />);

    expect(screen.getByLabelText('field')).toHaveValue('second');
    expect(mockOnCommit).not.toHaveBeenCalled();
  });

  describe('normalize', () => {
    it('should commit the normalized value', () => {
      render(
        <DraftTextField
          value="10"
          onCommit={mockOnCommit}
          normalize={(raw) => clampToInteger(raw, 1, 20)}
          inputProps={{'aria-label': 'field'}}
        />,
      );

      const input = screen.getByLabelText('field');
      fireEvent.change(input, {target: {value: '3.7'}});
      fireEvent.blur(input);

      expect(mockOnCommit).toHaveBeenCalledExactlyOnceWith('3');
      expect(input).toHaveValue('3');
    });

    it('should not clamp while typing', () => {
      render(
        <DraftTextField
          value="10"
          onCommit={mockOnCommit}
          normalize={(raw) => clampToInteger(raw, 1, 20)}
          inputProps={{'aria-label': 'field'}}
        />,
      );

      // Reaching 15 passes through 1, which the field would otherwise clamp away.
      const input = screen.getByLabelText('field');
      fireEvent.change(input, {target: {value: '1'}});
      fireEvent.change(input, {target: {value: '15'}});

      expect(input).toHaveValue('15');
      expect(mockOnCommit).not.toHaveBeenCalled();
    });

    it('should reset the field when normalization does not change the stored value', () => {
      render(
        <DraftTextField
          value="20"
          onCommit={mockOnCommit}
          normalize={(raw) => clampToInteger(raw, 1, 20)}
          inputProps={{'aria-label': 'field'}}
        />,
      );

      const input = screen.getByLabelText('field');
      fireEvent.change(input, {target: {value: '99'}});
      fireEvent.blur(input);

      // 99 clamps back to the stored 20, so nothing is committed, but the field must
      // not be left showing a value that was never stored.
      expect(mockOnCommit).not.toHaveBeenCalled();
      expect(input).toHaveValue('20');
    });

    it('should not commit the minimum when an empty field unmounts untouched', () => {
      // `clampToInteger('')` resolves to the minimum, so flushing an unedited field would
      // store a value the author never entered — and, before the selection-change fix, store
      // it against whichever component the panel had just switched to.
      const {unmount} = render(
        <DraftTextField
          value=""
          onCommit={mockOnCommit}
          normalize={(raw) => clampToInteger(raw, 1, 20)}
          inputProps={{'aria-label': 'field'}}
        />,
      );

      unmount();

      expect(mockOnCommit).not.toHaveBeenCalled();
    });

    it('should restore the committed value when normalization rejects the draft', () => {
      render(
        <DraftTextField
          value="10"
          onCommit={mockOnCommit}
          normalize={(raw) => clampToInteger(raw, 1, 20)}
          inputProps={{'aria-label': 'field'}}
        />,
      );

      const input = screen.getByLabelText('field');
      fireEvent.change(input, {target: {value: 'abc'}});
      fireEvent.blur(input);

      expect(mockOnCommit).not.toHaveBeenCalled();
      expect(input).toHaveValue('10');
    });
  });
});
