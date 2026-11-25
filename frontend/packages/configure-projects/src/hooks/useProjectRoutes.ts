// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';
import {useMemo} from 'react';

/**
 * Route paths this package needs from the host application, including the applications page a
 * project opens on.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useProjectRoutes` falls back to
 * `defaultProjectRoutePaths` below.
 *
 * @public
 */
export interface ProjectRoutePaths {
  projects: {
    list: () => string;
    create: () => string;
    /** A page in a project, named by its handle: the project's home, or the given path within it. */
    home: (handle: string, path?: string) => string;
  };
  applications: {
    list: () => string;
  };
}

/**
 * Default project paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultProjectRoutePaths: ProjectRoutePaths = {
  projects: {
    list: () => '/projects',
    create: () => '/projects/create',
    home: (handle: string, path = '') => `/project/${encodeURIComponent(handle)}${path}`,
  },
  applications: {
    list: () => '/applications',
  },
};

/**
 * Resolves the project route paths, preferring the host application's configuration (supplied via
 * `RoutesProvider`) and falling back to this package's own defaults.
 *
 * @public
 */
export default function useProjectRoutes(): ProjectRoutePaths {
  const {projects, applications} = useRoutes<Partial<ProjectRoutePaths>>();
  return useMemo(
    () => ({
      projects: projects ?? defaultProjectRoutePaths.projects,
      applications: applications ?? defaultProjectRoutePaths.applications,
    }),
    [projects, applications],
  );
}
