// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// API hooks
export {default as useGetGateways} from './api/useGetGateways';
export {default as useGetGateway} from './api/useGetGateway';
export {default as useRegisterGateway} from './api/useRegisterGateway';
export {default as useUpdateGateway} from './api/useUpdateGateway';
export {default as useDeleteGateway} from './api/useDeleteGateway';

// Constants
export {default as GatewayQueryKeys} from './constants/gateway-query-keys';

// Models
export type {Gateway, GatewayRegistration, RegisterGatewayRequest, UpdateGatewayRequest} from './models/gateway';

// Pages
export {default as GatewaysListPage} from './pages/GatewaysListPage';
export {default as GatewayDetailPage} from './pages/GatewayDetailPage';

// Routes
export type {GatewayRoutePaths} from './hooks/useGatewayRoutes';
export {defaultGatewayRoutePaths, default as useGatewayRoutes} from './hooks/useGatewayRoutes';
