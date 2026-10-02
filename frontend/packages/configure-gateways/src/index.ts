// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// API hooks
export {default as useGetGateways} from './api/useGetGateways';
export {default as useGetGateway} from './api/useGetGateway';
export {default as useRegisterGateway} from './api/useRegisterGateway';
export {default as useUpdateGateway} from './api/useUpdateGateway';
export {default as useDeleteGateway} from './api/useDeleteGateway';
export {default as useGetConfigurationVersions} from './api/useGetConfigurationVersions';
export {default as useCaptureConfigurationVersion} from './api/useCaptureConfigurationVersion';
export {default as useGetAppliedVersion} from './api/useGetAppliedVersion';
export {default as useGetGatewayDiff} from './api/useGetGatewayDiff';
export {default as useApplyVersion} from './api/useApplyVersion';
export {default as useRevertGateway} from './api/useRevertGateway';
export {default as useApplyDryRun} from './api/useApplyDryRun';
export {default as useRevertDryRun} from './api/useRevertDryRun';
export {default as useGetGatewayVariables} from './api/useGetGatewayVariables';
export {default as useCreateGatewayVariable} from './api/useCreateGatewayVariable';
export {default as useUpdateGatewayVariable} from './api/useUpdateGatewayVariable';
export {default as useDeleteGatewayVariable} from './api/useDeleteGatewayVariable';
export {default as useGetGatewaySecrets} from './api/useGetGatewaySecrets';
export {default as useCreateGatewaySecret} from './api/useCreateGatewaySecret';
export {default as useUpdateGatewaySecret} from './api/useUpdateGatewaySecret';
export {default as useDeleteGatewaySecret} from './api/useDeleteGatewaySecret';

// Constants
export {default as GatewayQueryKeys} from './constants/gateway-query-keys';

// Models
export type {
  Gateway,
  GatewayRegistration,
  RegisterGatewayRequest,
  UpdateGatewayRequest,
  ConfigurationVersion,
  CaptureConfigurationVersionRequest,
  AppliedVersion,
  ChangeType,
  ResourceChange,
  DiffSummary,
  GatewayDiff,
  ImportResourceResult,
  ImportResult,
  ApplyVersionRequest,
  RevertGatewayRequest,
  ApplyResult,
  MissingValues,
  ListLink,
  GatewayVariable,
  GatewaySecret,
  GatewayVariableList,
  GatewaySecretList,
  GatewayValueListParams,
  CreateGatewayValueRequest,
  UpdateGatewayValueRequest,
  SkippedResource,
} from './models/gateway';

// Pages
export {default as GatewaysListPage} from './pages/GatewaysListPage';
export {default as GatewayDetailPage} from './pages/GatewayDetailPage';

// Routes
export type {GatewayRoutePaths} from './hooks/useGatewayRoutes';
export {defaultGatewayRoutePaths, default as useGatewayRoutes} from './hooks/useGatewayRoutes';
