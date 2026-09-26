// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, type JSX} from 'react';
import {useLocation, useNavigate} from 'react-router';
import {getTopLevelSegment, isRouteHiddenOnPlane, usePlane} from '../lib/plane';

/**
 * Redirects Data Plane runtime-only routes to home when the console runs as the Control Plane
 * (authoring) surface. Complements the navigation hiding in DashboardLayout so that deep links and
 * manually typed URLs (e.g. /agents, /verifiable-credentials, /welcome) cannot reach DP-only pages
 * on the CP. On the Data Plane and hybrid consoles this is a no-op.
 */
export default function PlaneRouteGuard(): JSX.Element | null {
  const plane = usePlane();
  const {pathname} = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    if (isRouteHiddenOnPlane(getTopLevelSegment(pathname), plane)) {
      void navigate('/home', {replace: true});
    }
  }, [pathname, plane, navigate]);

  return null;
}
