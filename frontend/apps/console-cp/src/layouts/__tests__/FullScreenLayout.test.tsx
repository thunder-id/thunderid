// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable react/require-default-props */

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import FullScreenLayout from '../FullScreenLayout';

// Mock Outlet
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    Outlet: () => <div data-testid="outlet">Outlet Content</div>,
  };
});

interface MockComponentProps {
  children: React.ReactNode;
  sx?: Record<string, unknown>;
}

// Mock @wso2/oxygen-ui Layout and Box
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof import('@wso2/oxygen-ui')>('@wso2/oxygen-ui');

  return {
    ...actual,
    Box: ({children, sx, ...props}: MockComponentProps) => (
      <div data-testid="box" data-sx={JSON.stringify(sx)} {...props}>
        {children}
      </div>
    ),
    Layout: Object.assign(
      ({children, sx, ...props}: MockComponentProps) => (
        <div data-testid="layout-root" data-sx={JSON.stringify(sx)} {...props}>
          {children}
        </div>
      ),
      {
        Content: ({children}: {children: React.ReactNode}) => <div data-testid="layout-content">{children}</div>,
      },
    ),
  };
});

describe('FullScreenLayout', () => {
  it('renders Layout component with minHeight 100vh', () => {
    render(<FullScreenLayout />);

    const layout = screen.getByTestId('layout-root');
    expect(layout).toBeInTheDocument();
    expect(layout).toHaveAttribute('data-sx', JSON.stringify({minHeight: '100vh'}));
  });

  it('renders Layout.Content', () => {
    render(<FullScreenLayout />);

    expect(screen.getByTestId('layout-content')).toBeInTheDocument();
  });

  it('renders Box with minHeight 100vh', () => {
    render(<FullScreenLayout />);

    const box = screen.getByTestId('box');
    expect(box).toBeInTheDocument();
    expect(box).toHaveAttribute('data-sx', JSON.stringify({minHeight: '100vh'}));
  });

  it('renders Outlet for nested routes', () => {
    render(<FullScreenLayout />);

    expect(screen.getByTestId('outlet')).toBeInTheDocument();
    expect(screen.getByTestId('outlet')).toHaveTextContent('Outlet Content');
  });

  it('renders complete layout structure in correct hierarchy', () => {
    render(<FullScreenLayout />);

    // Verify all components are rendered
    const layout = screen.getByTestId('layout-root');
    const content = screen.getByTestId('layout-content');
    const box = screen.getByTestId('box');
    const outlet = screen.getByTestId('outlet');

    expect(layout).toBeInTheDocument();
    expect(content).toBeInTheDocument();
    expect(box).toBeInTheDocument();
    expect(outlet).toBeInTheDocument();

    // Verify hierarchy: Layout > Layout.Content > Box > Outlet
    expect(layout).toContainElement(content);
    expect(content).toContainElement(box);
    expect(box).toContainElement(outlet);
  });
});
