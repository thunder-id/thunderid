// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GatewayExplorerPage} from '@thunderid/configure-gateways';
import {ProjectScopeProvider} from '@thunderid/contexts';
import type {JSX} from 'react';

/** The map of what a gateway runs, for the whole organization: a gateway belongs to no project. */
export default function OrganizationExplorerPage(): JSX.Element {
  return (
    <ProjectScopeProvider project={undefined}>
      <GatewayExplorerPage />
    </ProjectScopeProvider>
  );
}
