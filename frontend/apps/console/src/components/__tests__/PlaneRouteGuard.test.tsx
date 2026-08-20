// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Plane} from '../../lib/plane';
import PlaneRouteGuard from '../PlaneRouteGuard';

let mockPlane: Plane;
let mockPathname: string;
const mockNavigate = vi.fn();

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({config: {plane: mockPlane}}),
}));

vi.mock('react-router', () => ({
  useLocation: () => ({pathname: mockPathname}),
  useNavigate: () => mockNavigate,
}));

describe('PlaneRouteGuard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPlane = 'cp';
    mockPathname = '/home';
  });

  it('redirects DP-only routes to home on the Control Plane', () => {
    mockPathname = '/welcome/tryout/mcp';
    render(<PlaneRouteGuard />);
    expect(mockNavigate).toHaveBeenCalledWith('/home', {replace: true});
  });

  it('leaves agents and verifiable credentials reachable on the Control Plane', () => {
    // They are authored on a control plane and promoted, the same as applications and connections.
    mockPathname = '/agents/create';
    const {rerender} = render(<PlaneRouteGuard />);
    expect(mockNavigate).not.toHaveBeenCalled();

    mockPathname = '/verifiable-credentials';
    rerender(<PlaneRouteGuard />);
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('leaves authoring routes untouched on the Control Plane', () => {
    mockPathname = '/applications';
    render(<PlaneRouteGuard />);
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('is a no-op on the Data Plane', () => {
    mockPlane = 'dp';
    mockPathname = '/agents';
    render(<PlaneRouteGuard />);
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('is a no-op on the hybrid console', () => {
    mockPlane = 'hybrid';
    mockPathname = '/verifiable-presentations';
    render(<PlaneRouteGuard />);
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
