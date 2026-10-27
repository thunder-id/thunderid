// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useGetAppConfig} from './api/useGetAppConfig';

// Constants
export * from './constants/app-config-api';
export * from './constants/app-catalog';

// Models
export * from './models/app-config';

// Themes
export {
  APP_THEME_COLOR,
  createAppSdkThemes,
  createAppTheme,
  DEFAULT_APP_THEME,
  resolveAppTheme,
  type AppColorScheme,
  type ResolvedAppTheme,
} from './utils/appTheme';

// Components
export {default as AppBrand, type AppBrandProps} from './components/AppBrand';
