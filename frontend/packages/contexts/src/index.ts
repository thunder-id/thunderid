// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Export types
export type {ProductConfig, ServerConfig, TrustedIssuerConfig, BrandConfig, SdkConfig, Plane} from './Config/types';
export type {ToastContextType, ToastSeverity} from './Toast/ToastContext';
export type {RoutePaths} from './Routes/RoutesContext';

// Export React components and hooks
export {default as ConfigContext, type ConfigContextType} from './Config/ConfigContext';
export {default as ConfigProvider, type ConfigProviderProps} from './Config/ConfigProvider';
export {default as useConfig} from './Config/useConfig';
export {default as ToastContext} from './Toast/ToastContext';
export {default as ToastProvider, type ToastProviderProps} from './Toast/ToastProvider';
export {default as useToast} from './Toast/useToast';
export {default as RoutesContext} from './Routes/RoutesContext';
export {default as RoutesProvider, type RoutesProviderProps} from './Routes/RoutesProvider';
export {default as useRoutes} from './Routes/useRoutes';

// Administration actions: how this console carries out the operations that depend on a runtime.
// A console that installs none takes the plain management path for every one of them.
export {
  AdministrationActionsContext,
  AdministrationActionsProvider,
  useAdministrationActions,
} from './AdministrationActions';
export type {
  AdministrationActions,
  AdministrationActionsProviderProps,
  AdministrationHttpLike,
} from './AdministrationActions';
