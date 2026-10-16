// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import Timestamp from '../Timestamp';
import VersionName from '../VersionName';

describe('VersionName', () => {
  it('names a version by its number with the hash a hover away, or by its short hash alone', async () => {
    render(<VersionName number={14} hash="abcdef0123456789" />);

    expect(screen.getByText('v14')).toBeInTheDocument();
    await userEvent.hover(screen.getByText('v14'));
    expect(await screen.findByText(/abcdef0123456789/)).toBeInTheDocument();

    render(<VersionName hash="fedcba9876543210" />);
    expect(screen.getByText('fedcba9')).toBeInTheDocument();
  });
});

describe('Timestamp', () => {
  it('shows nothing for no moment and the text itself for one it cannot read', () => {
    const {container} = render(<Timestamp />);
    expect(container).toBeEmptyDOMElement();

    render(<Timestamp value="not a date" />);
    expect(screen.getByText('not a date')).toBeInTheDocument();
  });
});
