// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useEffect, type JSX} from 'react';
import {useLocation, useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import {isControlPlane, usePlane} from '../../../lib/plane';
import getWelcomeDismissedStorageKey from '../utils/getWelcomeDismissedStorageKey';

export default function WelcomeRedirect(): JSX.Element | null {
  const {isSignedIn} = useThunderID();
  const {config} = useConfig();
  const plane = usePlane();
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    // The onboarding/welcome flow is a Data Plane runtime concern; the Control Plane authoring
    // console never auto-redirects into it (and PlaneRouteGuard blocks direct navigation there).
    if (!isSignedIn || isControlPlane(plane) || location.pathname.startsWith('/welcome')) return;

    const productName = config.brand.product_name;
    const dismissed = sessionStorage.getItem(getWelcomeDismissedStorageKey(productName)) === 'true';

    if (!dismissed) {
      sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
      void navigate(RouteConfig.welcome.root(), {replace: true});
    }
  }, [isSignedIn, plane, navigate, config.brand.product_name, location.pathname]);

  return null;
}
