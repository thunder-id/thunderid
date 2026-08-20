// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import DataPlaneStatusChip from '../DataPlaneStatusChip';

describe('DataPlaneStatusChip', () => {
  it('reports a connected data plane', () => {
    render(<DataPlaneStatusChip status={{connected: true, lastSeen: '2026-08-01T10:00:00Z'}} />);

    expect(screen.getByText('Data Plane connected')).toBeInTheDocument();
  });

  it('reports one that is offline', () => {
    render(<DataPlaneStatusChip status={{connected: false}} />);

    expect(screen.getByText('Data Plane offline')).toBeInTheDocument();
  });

  // An environment the service could not report on is offline as far as an operator is concerned:
  // nothing can be applied to it either way, and showing nothing would read as connected.
  it('treats an unknown status as offline', () => {
    render(<DataPlaneStatusChip />);

    expect(screen.getByText('Data Plane offline')).toBeInTheDocument();
  });
});
