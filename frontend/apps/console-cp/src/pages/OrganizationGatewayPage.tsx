// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GatewayPage, type GatewaySection} from '@thunderid/configure-gateways';
import {ProjectScopeProvider} from '@thunderid/contexts';
import type {JSX} from 'react';

/**
 * A section of a gateway's own pages. A gateway belongs to the organization, not to a project, so what it
 * shows is the organization's whichever project is selected.
 */
export default function OrganizationGatewayPage({section = 'overview'}: {section?: GatewaySection}): JSX.Element {
  return (
    <ProjectScopeProvider project={undefined}>
      <GatewayPage section={section} />
    </ProjectScopeProvider>
  );
}
