// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import Device from '../Device';

const mockUseDesign = vi.fn();

vi.mock('../DeviceBox', () => ({
  default: () => <div data-testid="device-box">DeviceBox</div>,
}));

vi.mock('@thunderid/design', () => ({
  AuthPageLayout: ({children, isLoading, variant}: {children: ReactNode; isLoading: boolean; variant: string}) => (
    <div data-testid="auth-page-layout" data-loading={String(isLoading)} data-variant={variant}>
      {children}
    </div>
  ),
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  useDesign: () => mockUseDesign(),
}));

describe('Device', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseDesign.mockReturnValue({isLoading: false});
  });

  it('renders DeviceBox inside the sign-in layout', () => {
    render(<Device />);

    expect(screen.getByTestId('device-box')).toBeInTheDocument();
    expect(screen.getByTestId('auth-page-layout')).toHaveAttribute('data-variant', 'SignIn');
    expect(screen.getByTestId('auth-page-layout')).toHaveAttribute('data-loading', 'false');
  });

  it('shows the layout loading state while design configuration is loading', () => {
    mockUseDesign.mockReturnValue({isLoading: true});

    render(<Device />);

    expect(screen.getByTestId('auth-page-layout')).toHaveAttribute('data-loading', 'true');
  });
});
