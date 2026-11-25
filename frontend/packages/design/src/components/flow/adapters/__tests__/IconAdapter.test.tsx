// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {render} from '@testing-library/react';
import {beforeAll, describe, expect, it} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import loadOxygenIcons from '../../../../utils/loadOxygenIcons';
import IconAdapter from '../IconAdapter';

const createComponent = (overrides: Partial<FlowComponent> = {}): FlowComponent =>
  ({id: 'icon-1', type: 'ICON', ...overrides}) as FlowComponent;

describe('IconAdapter', () => {
  // IconAdapter draws icons from the icon set it loads on first use. Load it up front, so the tests
  // render with it in place.
  beforeAll(async () => {
    await loadOxygenIcons();
  }, 30000);

  it('draws the configured icon at the configured size and color', () => {
    const {container} = render(<IconAdapter component={createComponent({name: 'User', size: 32, color: 'red'})} />);
    const icon = container.querySelector('svg.lucide-user');

    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('width', '32');
    expect(icon).toHaveAttribute('height', '32');
    expect(icon).toHaveAttribute('stroke', 'red');
  });

  it('draws a 24px icon in the current color by default', () => {
    const {container} = render(<IconAdapter component={createComponent({name: 'User'})} />);
    const icon = container.querySelector('svg.lucide-user');

    expect(icon).toHaveAttribute('width', '24');
    expect(icon).toHaveAttribute('stroke', 'currentColor');
  });

  it('renders nothing for a name the icon set does not have', () => {
    const {container} = render(<IconAdapter component={createComponent({name: 'NotAnIcon'})} />);

    expect(container).toBeEmptyDOMElement();
  });
});
