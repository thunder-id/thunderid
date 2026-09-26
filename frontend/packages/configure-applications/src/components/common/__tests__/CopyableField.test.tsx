// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import CopyableField from '../CopyableField';

const mockCopy = vi.fn().mockResolvedValue(undefined);

vi.mock('@thunderid/hooks', () => ({
  useCopyToClipboard: vi.fn(() => ({copied: false, copy: mockCopy})),
}));

describe('CopyableField', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCopy.mockResolvedValue(undefined);
  });

  it('renders a read-only field with the given label and value', () => {
    render(<CopyableField id="field-id" label="Client ID" value="my-client-id" copyAriaLabel="Copy Client ID" />);

    expect(screen.getByText('Client ID')).toBeInTheDocument();
    const input = screen.getByDisplayValue('my-client-id');
    expect(input).toHaveAttribute('readonly');
  });

  it('copies the value when the copy button is clicked', async () => {
    const user = userEvent.setup();
    render(<CopyableField id="field-id" label="Client ID" value="my-client-id" copyAriaLabel="Copy Client ID" />);

    await user.click(screen.getByRole('button', {name: 'Copy Client ID'}));

    expect(mockCopy).toHaveBeenCalledWith('my-client-id');
  });
});
