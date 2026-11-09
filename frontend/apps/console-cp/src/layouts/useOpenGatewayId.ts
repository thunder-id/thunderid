// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PROJECT_HANDLE_PARAM} from '@thunderid/configure-projects';
import {useMatch} from 'react-router';
import {ROUTE_SEGMENTS} from '../configs/RouteConfig';

const GATEWAY_PAGES = `${ROUTE_SEGMENTS.deployTargets}/:gatewayId/*`;
const ORGANIZATION_GATEWAY_PAGES = `/${GATEWAY_PAGES}`;
const PROJECT_GATEWAY_PAGES = `/${ROUTE_SEGMENTS.project}/:${PROJECT_HANDLE_PARAM}/${GATEWAY_PAGES}`;

/** The gateway whose pages are open, in the organization or in a project, or undefined on any other page. */
export default function useOpenGatewayId(): string | undefined {
  const inOrganization = useMatch<string>(ORGANIZATION_GATEWAY_PAGES);
  const inProject = useMatch<string>(PROJECT_GATEWAY_PAGES);
  return inOrganization?.params.gatewayId ?? inProject?.params.gatewayId;
}
