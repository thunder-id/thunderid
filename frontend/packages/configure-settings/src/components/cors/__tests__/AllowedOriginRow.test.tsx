// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import {AllowedOriginTypes} from '../../../models/allowedOriginRow';
import AllowedOriginRow, {type AllowedOriginRowProps} from '../AllowedOriginRow';

const LABELS = {
  originPlaceholder: 'https://app.example.com',
  regexPlaceholder: '^https://example\\.com$',
  typeLabel: 'Entry type',
  originOptionLabel: 'Origin',
  regexOptionLabel: 'Regex',
  removeLabel: 'Remove origin',
};

function renderRow(props: Partial<AllowedOriginRowProps> = {}) {
  return renderWithProviders(<AllowedOriginRow type={AllowedOriginTypes.ORIGIN} value="" {...LABELS} {...props} />);
}

describe('AllowedOriginRow', () => {
  it('shows the origin placeholder and no delimiters for an origin row', () => {
    renderRow({type: AllowedOriginTypes.ORIGIN});
    expect(screen.getByPlaceholderText('https://app.example.com')).toBeInTheDocument();
    expect(screen.queryByText('/')).toBeNull();
  });

  it('wraps a regex row in delimiters and keeps them out of the field value', () => {
    renderRow({type: AllowedOriginTypes.REGEX, value: '^https://x\\.io$'});
    expect(screen.getByPlaceholderText('^https://example\\.com$')).toBeInTheDocument();
    expect(screen.getAllByText('/')).toHaveLength(2);
    // The delimiters are decoration, so the field still reports the raw pattern.
    expect(screen.getByDisplayValue('^https://x\\.io$')).toBeInTheDocument();
  });

  // The type selector renders its own hidden native input ahead of the value field, so anything that
  // reaches for "the input in this row" by position gets the selector instead. Callers that address
  // the value field by role, as the end-to-end page object does, must keep finding exactly one.
  it('exposes the value field as the only textbox in the row', () => {
    renderRow({value: 'https://app.example.com'});
    expect(screen.getAllByRole('textbox')).toHaveLength(1);
    expect(screen.getByRole('textbox')).toHaveValue('https://app.example.com');
  });

  it('reports a type change', async () => {
    const user = userEvent.setup();
    const onTypeChange = vi.fn();
    renderRow({onTypeChange});

    await user.click(screen.getByRole('combobox', {name: 'Entry type'}));
    await user.click(screen.getByRole('option', {name: 'Regex'}));

    expect(onTypeChange).toHaveBeenCalledWith(AllowedOriginTypes.REGEX);
  });

  it('renders an error as helper text and puts the field in the error state', () => {
    renderRow({value: 'bad', error: 'Enter a valid origin.'});
    expect(screen.getByText('Enter a valid origin.')).toBeInTheDocument();
    expect(screen.getByDisplayValue('bad')).toHaveAttribute('aria-invalid', 'true');
  });

  it('renders a warning as helper text without the error state', () => {
    renderRow({type: AllowedOriginTypes.REGEX, value: 'acme\\.io', warning: 'Not anchored.'});
    expect(screen.getByText('Not anchored.')).toBeInTheDocument();
    expect(screen.getByDisplayValue('acme\\.io')).toHaveAttribute('aria-invalid', 'false');
  });

  it('prefers the error over the warning when both apply', () => {
    renderRow({value: 'x', error: 'Blocking.', warning: 'Advisory.'});
    expect(screen.getByText('Blocking.')).toBeInTheDocument();
    expect(screen.queryByText('Advisory.')).toBeNull();
  });

  it('locks the row: read-only field, disabled type, and a lock in place of the remove action', () => {
    renderRow({value: 'https://console.example.com', locked: true, lockedLabel: 'Managed declaratively.'});
    expect(screen.getByDisplayValue('https://console.example.com')).toHaveAttribute('readonly');
    // No placeholder, so a locked row is never mistaken for an empty editable one.
    expect(screen.queryByPlaceholderText('https://app.example.com')).toBeNull();
    expect(screen.getByRole('combobox', {name: 'Entry type'})).toHaveAttribute('aria-disabled', 'true');
    expect(screen.queryByRole('button', {name: 'Remove origin'})).toBeNull();
  });

  it('locks the row even when no lock explanation was supplied', () => {
    renderRow({value: 'https://console.example.com', locked: true});
    expect(screen.getByDisplayValue('https://console.example.com')).toHaveAttribute('readonly');
    expect(screen.queryByRole('button', {name: 'Remove origin'})).toBeNull();
  });

  it('reports a remove request', async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    renderRow({onRemove});

    await user.click(screen.getByRole('button', {name: 'Remove origin'}));

    expect(onRemove).toHaveBeenCalled();
  });
});
