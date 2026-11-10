// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoadingAnimation} from '@thunderid/components';
import {useProjectSelection} from '@thunderid/configure-projects';
import type {JSX} from 'react';
import {Navigate} from 'react-router';
import RouteConfig from '../configs/RouteConfig';

/** The handle of the project the console opens in. */
const DEFAULT_PROJECT = 'default';

/**
 * Where opening the console leads: into the default project, so its home is what the administrator sees
 * first; the organization's overview when there is no default project.
 */
export default function DefaultProjectRedirect(): JSX.Element {
  const {projects, isLoading} = useProjectSelection();
  if (isLoading) return <PageLoadingAnimation />;
  const hasDefault = projects.some((project) => project.handle === DEFAULT_PROJECT);
  return <Navigate to={hasDefault ? RouteConfig.projects.home(DEFAULT_PROJECT) : RouteConfig.home.list()} replace />;
}
