// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GatewayExplorerPage} from '@thunderid/configure-gateways';
import type {JSX} from 'react';
import RouteProjectScope from '../components/RouteProjectScope';

/** The map of what a gateway runs within a project: the project's own resources and what they need. */
export default function ProjectExplorerPage(): JSX.Element {
  return (
    <RouteProjectScope>
      <GatewayExplorerPage />
    </RouteProjectScope>
  );
}
