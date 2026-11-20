// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useGetAccountAccessConfig} from './api/useGetAccountAccessConfig';
export {default as useGetAccountLifecycleFlows} from './api/useGetAccountLifecycleFlows';
export {default as useGetCorsConfig} from './api/useGetCorsConfig';
export {default as useGetCspConfig} from './api/useGetCspConfig';
export {default as useUpdateAccountAccessConfig} from './api/useUpdateAccountAccessConfig';
export type {UpdateAccountAccessConfigVariables} from './api/useUpdateAccountAccessConfig';
export {default as useUpdateCorsConfig} from './api/useUpdateCorsConfig';
export type {UpdateCorsConfigVariables} from './api/useUpdateCorsConfig';
export {default as useUpdateCspConfig} from './api/useUpdateCspConfig';
export type {UpdateCspConfigVariables} from './api/useUpdateCspConfig';

// Components
export {default as AccountLockCard} from './components/accountSecurity/AccountLockCard';
export type {AccountLockCardProps} from './components/accountSecurity/AccountLockCard';
export {default as AccountSecuritySection} from './components/accountSecurity/AccountSecuritySection';
export {default as AccountSuspensionSection} from './components/accountSecurity/AccountSuspensionSection';
export {default as DurationField} from './components/accountSecurity/DurationField';
export type {DurationFieldProps} from './components/accountSecurity/DurationField';
export {default as LockLadderRows} from './components/accountSecurity/LockLadderRows';
export type {LockLadderRowsProps} from './components/accountSecurity/LockLadderRows';
export {default as AllowedOriginRow} from './components/cors/AllowedOriginRow';
export type {AllowedOriginRowProps} from './components/cors/AllowedOriginRow';
export {default as CorsSection} from './components/cors/CorsSection';
export {default as CspDirectiveRow} from './components/csp/CspDirectiveRow';
export type {CspDirectiveRowAction, CspDirectiveRowProps} from './components/csp/CspDirectiveRow';
export {default as CspScopeCard} from './components/csp/CspScopeCard';
export type {CspScopeCardProps} from './components/csp/CspScopeCard';
export {default as CspSection} from './components/csp/CspSection';

// Hooks
export {default as useAccountLockDraft} from './hooks/useAccountLockDraft';
export type {AccountLockDraft, AccountLockErrors} from './hooks/useAccountLockDraft';
export {default as useCspDraft} from './hooks/useCspDraft';
export type {CspDraft} from './hooks/useCspDraft';
export {default as useSettingsRoutes, defaultSettingsRoutePaths} from './hooks/useSettingsRoutes';
export type {SettingsRoutePaths} from './hooks/useSettingsRoutes';

// Constants
export {default as SettingsQueryKeys} from './constants/settings-query-keys';

// Models
export * from './models/accountAccess';
export * from './models/accountLifecycleFlows';
export * from './models/allowedOriginRow';
export * from './models/cspDirectiveRow';
export * from './models/responses';

// Pages
export {default as SettingsPage} from './pages/SettingsPage';

// Utils
export {buildAccountLockPayload, isSameAccountLockForm, toAccountLockForm} from './utils/accountLockPayload';
export type {AccountLockForm} from './utils/accountLockPayload';
export {default as describePolicy, formatDurationShort} from './utils/describePolicy';
export {DurationUnitSeconds, toDurationParts, toSeconds} from './utils/duration';
export type {DurationParts, DurationUnit} from './utils/duration';
export {
  createRow,
  createRowId,
  isRowEmpty,
  normalizeRowValue,
  rowKey,
  toAllowedOrigins,
  toRows,
} from './utils/allowedOriginRows';
export {
  buildCspPayload,
  buildScopeRows,
  createCspRow,
  CSP_BASELINE,
  CSP_DIRECTIVES,
  CSP_NONE_SOURCE,
  cspScopes,
  parseSources,
  rowSources,
  scopeDirectives,
  toCspDraft,
} from './utils/cspDirectiveRows';
export {default as buildCspHeader} from './utils/cspHeader';
export {default as isRegexAnchored} from './utils/isRegexAnchored';
export {isValidOrigin, isValidRegex, normalizeOrigin} from './utils/origin';
export {default as originValueText} from './utils/originValueText';
export {default as validateAllowedOriginRows, AllowedOriginRowIssueFallbacks} from './utils/validateAllowedOriginRows';
export type {
  AllowedOriginRowError,
  AllowedOriginRowIssues,
  AllowedOriginRowWarning,
} from './utils/validateAllowedOriginRows';
export {
  default as validateCspDirectiveRows,
  CspDirectiveRowIssueFallbacks,
  isInvalidCspSource,
} from './utils/validateCspDirectiveRows';
export type {CspDirectiveRowError} from './utils/validateCspDirectiveRows';
