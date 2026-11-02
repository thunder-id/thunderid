// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/** Where the Deploy page and a gateway's explorer are, and the home page they exit to, as the console routes them. */
export interface DeployRoutePaths {
  home: {
    list: () => string;
  };
  deploy: {
    /** The Deploy page: the captured versions and every gateway. */
    list: () => string;
    /** The explorer of one gateway: what it runs and the values it holds. */
    explore: (gatewayId: string) => string;
    /** The variables and secrets one gateway holds. */
    values: (gatewayId: string) => string;
  };
}

export const defaultDeployRoutePaths: DeployRoutePaths = {
  home: {
    list: () => '/home',
  },
  deploy: {
    list: () => '/deploy',
    explore: (gatewayId: string) => `/deploy/gateways/${encodeURIComponent(gatewayId)}/explore`,
    values: (gatewayId: string) => `/deploy/gateways/${encodeURIComponent(gatewayId)}/values`,
  },
};

export default function useDeployRoutes(): DeployRoutePaths {
  const routes = useRoutes<Partial<DeployRoutePaths>>();
  return {
    home: routes.home ?? defaultDeployRoutePaths.home,
    deploy: routes.deploy ?? defaultDeployRoutePaths.deploy,
  };
}
