// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, userEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import GatewaysStrip from '../GatewaysStrip';

/** Gives the strip's track the sizes a browser would lay it out with. */
function size(track: HTMLElement, {scrollLeft, clientWidth, scrollWidth}: Record<string, number>): void {
  Object.defineProperty(track, 'scrollLeft', {configurable: true, value: scrollLeft});
  Object.defineProperty(track, 'clientWidth', {configurable: true, value: clientWidth});
  Object.defineProperty(track, 'scrollWidth', {configurable: true, value: scrollWidth});
}

describe('GatewaysStrip', () => {
  it('steps through gateways that do not fit, and fades the side there is more to see', async () => {
    render(
      <GatewaysStrip count={4}>
        <div>Dev</div>
      </GatewaysStrip>,
    );
    const track = screen.getByTestId('gateways-strip');
    const scrollBy = vi.fn();
    track.scrollBy = scrollBy;

    size(track, {scrollLeft: 0, clientWidth: 600, scrollWidth: 1200});
    fireEvent.scroll(track);

    const left = screen.getByRole('button', {name: 'Scroll gateways left'});
    const right = screen.getByRole('button', {name: 'Show more gateways'});
    expect(left).toBeDisabled();
    expect(right).toBeEnabled();
    expect(screen.queryByTestId('strip-fade-left')).not.toBeInTheDocument();
    expect(screen.getByTestId('strip-fade-right')).toBeInTheDocument();

    await userEvent.click(right);
    expect(scrollBy).toHaveBeenCalledWith({left: 300, behavior: 'smooth'});

    size(track, {scrollLeft: 600, clientWidth: 600, scrollWidth: 1200});
    fireEvent.scroll(track);
    expect(left).toBeEnabled();
    expect(right).toBeDisabled();
    expect(screen.getByTestId('strip-fade-left')).toBeInTheDocument();
    expect(screen.queryByTestId('strip-fade-right')).not.toBeInTheDocument();
  });

  it('offers no scrolling for one gateway', () => {
    render(
      <GatewaysStrip count={1}>
        <div>Dev</div>
      </GatewaysStrip>,
    );

    expect(screen.getByText('1 gateway')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Show more gateways'})).not.toBeInTheDocument();
  });
});
