// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import TimerAdapter from '../TimerAdapter';

describe('TimerAdapter', () => {
  const createResource = (label?: string): Resource =>
    ({
      id: 'timer-1',
      ...(label !== undefined ? {label} : {}),
    }) as unknown as Resource;

  it('should replace the dynamic time placeholder with a canvas value', () => {
    render(<TimerAdapter resource={createResource('Try again in {time}')} />);

    expect(screen.getByText('Try again in 05:00')).toBeInTheDocument();
  });

  it('should render default text when resource label is missing', () => {
    render(<TimerAdapter resource={createResource()} />);

    expect(screen.getByText('Time remaining: 05:00')).toBeInTheDocument();
  });

  it('should render default text when resource is undefined', () => {
    render(<TimerAdapter />);

    expect(screen.getByText('Time remaining: 05:00')).toBeInTheDocument();
  });

  it('should render default text when resource label property is missing', () => {
    const resourceWithoutLabel = {
      id: 'timer-no-label',
    } as unknown as Resource;

    render(<TimerAdapter resource={resourceWithoutLabel} />);

    expect(screen.getByText('Time remaining: 05:00')).toBeInTheDocument();
  });
});
