// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {useCallback} from 'react';
import {useNavigate} from 'react-router';
import useGatewayRoutes from './useGatewayRoutes';
import {GATEWAY_DETAIL_TAB_PARAM, type GatewayValuesTab} from '../constants/gateway-values';

/**
 * Returns a function that opens a gateway's variables tab or its secrets tab.
 */
export default function useOpenGatewayTab(): (gatewayId: string, tab: GatewayValuesTab) => void {
  const navigate = useNavigate();
  const routes = useGatewayRoutes();
  const logger = useLogger('useOpenGatewayTab');

  return useCallback(
    (gatewayId: string, tab: GatewayValuesTab): void => {
      const query = new URLSearchParams({[GATEWAY_DETAIL_TAB_PARAM]: tab});
      (async (): Promise<void> => {
        await navigate(`${routes.detail(gatewayId)}?${query.toString()}`);
      })().catch((err: unknown) => {
        logger.error('Failed to open the gateway tab', {error: err});
      });
    },
    [navigate, routes, logger],
  );
}
