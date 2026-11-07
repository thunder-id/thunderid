// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GatewayPage, type GatewaySection} from '@thunderid/configure-gateways';
import type {JSX} from 'react';
import RouteProjectScope from '../components/RouteProjectScope';

/** A section of a gateway's own pages within a project: what it shows is the project's. */
export default function ProjectGatewayPage({section = 'overview'}: {section?: GatewaySection}): JSX.Element {
  return (
    <RouteProjectScope>
      <GatewayPage section={section} />
    </RouteProjectScope>
  );
}
