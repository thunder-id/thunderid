// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';
import {useMemo} from 'react';

/**
 * Route paths this package needs from the host application: where the home page lives, and the
 * welcome area with its project, getting-started, tryout and import steps. The tryouts run flows, so
 * a host that executes none leaves their paths out, and the welcome page then offers no tryouts.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useOnboardingRoutes` falls back to
 * `defaultOnboardingRoutePaths` below.
 *
 * @public
 */
export interface OnboardingRoutePaths {
  home: {
    list: () => string;
  };
  welcome: {
    root: () => string;
    createProject: () => string;
    getStarted: () => string;
    getStartedApplicationsTypes: () => string;
    getStartedApplicationsCreate: () => string;
    getStartedAgentsCreate: () => string;
    tryoutSecuringApplication?: () => string;
    tryoutAiAgents?: () => string;
    tryoutMcp?: () => string;
    importConfigurationUpload: () => string;
    importConfigurationValidate: () => string;
    importConfigurationSummary: () => string;
  };
}

/**
 * Default onboarding paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultOnboardingRoutePaths: OnboardingRoutePaths = {
  home: {
    list: () => '/home',
  },
  welcome: {
    root: () => '/welcome',
    createProject: () => '/welcome/create-project',
    getStarted: () => '/welcome/get-started',
    getStartedApplicationsTypes: () => '/welcome/get-started/applications/types',
    getStartedApplicationsCreate: () => '/welcome/get-started/applications/create',
    getStartedAgentsCreate: () => '/welcome/get-started/agents/create',
    tryoutSecuringApplication: () => '/welcome/tryout/securing-application',
    tryoutAiAgents: () => '/welcome/tryout/ai-agents',
    tryoutMcp: () => '/welcome/tryout/mcp',
    importConfigurationUpload: () => '/welcome/import-configuration',
    importConfigurationValidate: () => '/welcome/import-configuration/validate',
    importConfigurationSummary: () => '/welcome/import-configuration/summary',
  },
};

/**
 * Resolves the onboarding route paths, preferring the host application's configuration and
 * falling back to this package's own defaults.
 *
 * @public
 */
export default function useOnboardingRoutes(): OnboardingRoutePaths {
  const routes = useRoutes<Partial<OnboardingRoutePaths>>();
  const home = routes.home ?? defaultOnboardingRoutePaths.home;
  const welcome = routes.welcome ?? defaultOnboardingRoutePaths.welcome;
  return useMemo(() => ({home, welcome}), [home, welcome]);
}
