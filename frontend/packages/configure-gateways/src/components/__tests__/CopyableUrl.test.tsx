// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import CopyableUrl from '../CopyableUrl';

describe('CopyableUrl', () => {
  it('copies the URL and says so, then offers to copy again', async () => {
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
    render(<CopyableUrl url="https://dp.test" />);

    await userEvent.click(screen.getByRole('button', {name: 'Copy the URL'}));

    expect(writeText).toHaveBeenCalledWith('https://dp.test');
    await userEvent.hover(screen.getByRole('button', {name: 'Copy the URL'}));
    expect(await screen.findByText('Copied')).toBeInTheDocument();
    writeText.mockRestore();
  });

  it('keeps offering to copy when the clipboard refuses', async () => {
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'));
    render(<CopyableUrl url="https://dp.test" />);

    await userEvent.click(screen.getByRole('button', {name: 'Copy the URL'}));
    await userEvent.hover(screen.getByRole('button', {name: 'Copy the URL'}));

    expect(await screen.findByText('Copy the URL')).toBeInTheDocument();
    expect(screen.queryByText('Copied')).not.toBeInTheDocument();
    writeText.mockRestore();
  });
});
