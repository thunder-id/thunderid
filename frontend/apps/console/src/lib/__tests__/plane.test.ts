// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {
  CP_HIDDEN_NAV_IDS,
  CP_ONLY_HIDDEN_NAV_IDS,
  DP_ONLY_ROUTE_SEGMENTS,
  getTopLevelSegment,
  hiddenNavIds,
  isControlPlane,
  isRouteHiddenOnPlane,
  usePlane,
} from '../plane';
import type {Plane} from '../plane';

let mockPlane: Plane | undefined;

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({config: {plane: mockPlane}}),
}));

describe('plane', () => {
  beforeEach(() => {
    mockPlane = undefined;
  });

  describe('usePlane', () => {
    it('returns the configured plane', () => {
      mockPlane = 'cp';
      const {result} = renderHook(() => usePlane());
      expect(result.current).toBe('cp');
    });

    it('defaults to hybrid when the plane is unset', () => {
      mockPlane = undefined;
      const {result} = renderHook(() => usePlane());
      expect(result.current).toBe('hybrid');
    });
  });

  describe('isControlPlane', () => {
    it('is true only for cp', () => {
      expect(isControlPlane('cp')).toBe(true);
      expect(isControlPlane('dp')).toBe(false);
      expect(isControlPlane('hybrid')).toBe(false);
    });
  });

  describe('getTopLevelSegment', () => {
    it.each([
      ['/agents', 'agents'],
      ['/agents/create', 'agents'],
      ['/verifiable-presentations/vp1', 'verifiable-presentations'],
      ['/welcome/tryout/mcp', 'welcome'],
      ['/', ''],
      ['', ''],
    ])('maps %s to "%s"', (pathname, expected) => {
      expect(getTopLevelSegment(pathname)).toBe(expected);
    });
  });

  describe('derived sets', () => {
    it('shows agents and verifiable credentials on the CP', () => {
      // They are configuration like applications and connections: authored on a control plane and
      // promoted to a data plane, so neither plane hides them.
      expect(CP_HIDDEN_NAV_IDS.has('agents')).toBe(false);
      expect(CP_HIDDEN_NAV_IDS.has('verifiable-credentials')).toBe(false);
      expect(CP_HIDDEN_NAV_IDS.has('credentials')).toBe(false);
      expect(CP_HIDDEN_NAV_IDS.has('presentations')).toBe(false);
    });

    it('keeps declarative authoring entries visible on the CP', () => {
      expect(CP_HIDDEN_NAV_IDS.has('applications')).toBe(false);
      expect(CP_HIDDEN_NAV_IDS.has('users')).toBe(false);
      expect(CP_HIDDEN_NAV_IDS.has('flows')).toBe(false);
    });

    it('blocks only the welcome route segment', () => {
      expect(DP_ONLY_ROUTE_SEGMENTS.has('agents')).toBe(false);
      expect(DP_ONLY_ROUTE_SEGMENTS.has('verifiable-credentials')).toBe(false);
      expect(DP_ONLY_ROUTE_SEGMENTS.has('verifiable-presentations')).toBe(false);
      expect(DP_ONLY_ROUTE_SEGMENTS.has('welcome')).toBe(true);
    });

    it('has no dedicated welcome nav entry to hide', () => {
      expect(CP_HIDDEN_NAV_IDS.has('welcome')).toBe(false);
    });
  });

  describe('isRouteHiddenOnPlane', () => {
    it('hides DP-only segments only on the Control Plane', () => {
      expect(isRouteHiddenOnPlane('welcome', 'cp')).toBe(true);
      expect(isRouteHiddenOnPlane('welcome', 'dp')).toBe(false);
      expect(isRouteHiddenOnPlane('welcome', 'hybrid')).toBe(false);
    });

    it('shows agents on every plane', () => {
      expect(isRouteHiddenOnPlane('agents', 'cp')).toBe(false);
      expect(isRouteHiddenOnPlane('agents', 'dp')).toBe(false);
      expect(isRouteHiddenOnPlane('agents', 'hybrid')).toBe(false);
    });

    it('hides promotions everywhere except the Control Plane', () => {
      expect(isRouteHiddenOnPlane('promotions', 'cp')).toBe(false);
      expect(isRouteHiddenOnPlane('promotions', 'dp')).toBe(true);
      expect(isRouteHiddenOnPlane('promotions', 'hybrid')).toBe(true);
    });

    it('never hides authoring segments', () => {
      expect(isRouteHiddenOnPlane('applications', 'cp')).toBe(false);
      expect(isRouteHiddenOnPlane('home', 'cp')).toBe(false);
    });
  });

  describe('CP-only gating', () => {
    it('shows agents on every plane, the way applications and connections are shown', () => {
      expect(hiddenNavIds('cp').has('agents')).toBe(false);
      expect(hiddenNavIds('dp').has('agents')).toBe(false);
      expect(hiddenNavIds('hybrid').has('agents')).toBe(false);
    });

    it('marks environment variables as a Control Plane-only nav entry', () => {
      expect(CP_ONLY_HIDDEN_NAV_IDS.has('environment-variables')).toBe(true);
      expect(isRouteHiddenOnPlane('environment-variables', 'cp')).toBe(false);
      expect(isRouteHiddenOnPlane('environment-variables', 'dp')).toBe(true);
      expect(isRouteHiddenOnPlane('environment-variables', 'hybrid')).toBe(true);
    });

    it('marks promotions as a Control Plane-only nav entry', () => {
      expect(CP_ONLY_HIDDEN_NAV_IDS.has('promotions')).toBe(true);
      expect(CP_HIDDEN_NAV_IDS.has('promotions')).toBe(false);
      expect(hiddenNavIds('cp').has('promotions')).toBe(false);
      expect(hiddenNavIds('dp').has('promotions')).toBe(true);
      expect(hiddenNavIds('hybrid').has('promotions')).toBe(true);
    });
  });
});
