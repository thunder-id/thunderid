// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useEffect, type JSX} from 'react';
import {useLocation, useNavigate} from 'react-router';
import RouteConfig from '../../configs/RouteConfig';
import getWelcomeDismissedStorageKey from '../../utils/getWelcomeDismissedStorageKey';

export default function WelcomeRedirect(): JSX.Element | null {
  const {isSignedIn} = useThunderID();
  const {config} = useConfig();
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    if (!isSignedIn || location.pathname.startsWith('/welcome')) return;

    const productName = config.brand.product_name;
    const dismissed = sessionStorage.getItem(getWelcomeDismissedStorageKey(productName)) === 'true';

    if (!dismissed) {
      sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
      void navigate(RouteConfig.welcome.root(), {replace: true});
    }
  }, [isSignedIn, navigate, config.brand.product_name, location.pathname]);

  return null;
}
