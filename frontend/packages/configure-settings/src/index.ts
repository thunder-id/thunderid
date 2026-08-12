// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useGetCorsConfig} from './api/useGetCorsConfig';
export {default as useUpdateCorsConfig} from './api/useUpdateCorsConfig';
export type {UpdateCorsConfigVariables} from './api/useUpdateCorsConfig';

// Components
export {default as AllowedOriginRow} from './components/cors/AllowedOriginRow';
export type {AllowedOriginRowProps} from './components/cors/AllowedOriginRow';
export {default as CorsSection} from './components/cors/CorsSection';

// Constants
export {default as SettingsQueryKeys} from './constants/settings-query-keys';

// Models
export * from './models/allowedOriginRow';
export * from './models/responses';

// Pages
export {default as SettingsPage} from './pages/SettingsPage';

// Utils
export {
  createRow,
  createRowId,
  isRowEmpty,
  normalizeRowValue,
  rowKey,
  toAllowedOrigins,
  toRows,
} from './utils/allowedOriginRows';
export {default as isRegexAnchored} from './utils/isRegexAnchored';
export {isValidOrigin, isValidRegex, normalizeOrigin} from './utils/origin';
export {default as originValueText} from './utils/originValueText';
export {default as validateAllowedOriginRows, AllowedOriginRowIssueFallbacks} from './utils/validateAllowedOriginRows';
export type {
  AllowedOriginRowError,
  AllowedOriginRowIssues,
  AllowedOriginRowWarning,
} from './utils/validateAllowedOriginRows';
