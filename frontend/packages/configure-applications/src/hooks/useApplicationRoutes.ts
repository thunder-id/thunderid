// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';
import {useMemo} from 'react';

/**
 * Route paths this package needs from the host application, including the flow and onboarding
 * destinations the application pages link to.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useApplicationRoutes` falls back to
 * `defaultApplicationRoutePaths` below.
 *
 * @public
 */
export interface ApplicationRoutePaths {
  applications: {
    list: () => string;
    detail: (id: string) => string;
    types: () => string;
    create: () => string;
  };
  flows: {
    list: () => string;
    detail: (flowId: string) => string;
  };
  welcome: {
    root: () => string;
    createProject: () => string;
    getStarted: () => string;
  };
}

/**
 * Default application paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultApplicationRoutePaths: ApplicationRoutePaths = {
  applications: {
    list: () => '/applications',
    detail: (id) => `/applications/${id}`,
    types: () => '/applications/types',
    create: () => '/applications/create',
  },
  flows: {
    list: () => '/flows',
    detail: (flowId) => `/flows/${flowId}`,
  },
  welcome: {
    root: () => '/welcome',
    createProject: () => '/welcome/create-project',
    getStarted: () => '/welcome/get-started',
  },
};

/**
 * Resolves the application route paths, preferring the host application's configuration
 * (supplied via `RoutesProvider`) and falling back to this package's own defaults.
 *
 * @public
 */
export default function useApplicationRoutes(): ApplicationRoutePaths {
  const {applications, flows, welcome} = useRoutes<Partial<ApplicationRoutePaths>>();
  return useMemo(
    () => ({
      applications: applications ?? defaultApplicationRoutePaths.applications,
      flows: flows ?? defaultApplicationRoutePaths.flows,
      welcome: welcome ?? defaultApplicationRoutePaths.welcome,
    }),
    [applications, flows, welcome],
  );
}
