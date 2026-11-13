// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// APIs
export {default as useGetAccountAccessConfig} from './api/useGetAccountAccessConfig';
export {default as useGetAccountLifecycleFlows} from './api/useGetAccountLifecycleFlows';
export {default as useGetCorsConfig} from './api/useGetCorsConfig';
export {default as useGetServerConfigSection} from './api/useGetServerConfigSection';
export {default as useGetServerConfigSections} from './api/useGetServerConfigSections';
export {default as useUpdateAccountAccessConfig} from './api/useUpdateAccountAccessConfig';
export type {UpdateAccountAccessConfigVariables} from './api/useUpdateAccountAccessConfig';
export {default as useUpdateCorsConfig} from './api/useUpdateCorsConfig';
export type {UpdateCorsConfigVariables} from './api/useUpdateCorsConfig';
export {default as useUpdateServerConfigSection} from './api/useUpdateServerConfigSection';
export type {UpdateServerConfigSectionVariables} from './api/useUpdateServerConfigSection';

// Components
export {default as AccountLockCard} from './components/accountSecurity/AccountLockCard';
export type {AccountLockCardProps} from './components/accountSecurity/AccountLockCard';
export {default as AccountSecuritySection} from './components/accountSecurity/AccountSecuritySection';
export {default as AccountSuspensionSection} from './components/accountSecurity/AccountSuspensionSection';
export {default as LockLadderRows} from './components/accountSecurity/LockLadderRows';
export type {LockLadderRowsProps} from './components/accountSecurity/LockLadderRows';
export {default as AllowedOriginRow} from './components/cors/AllowedOriginRow';
export type {AllowedOriginRowProps} from './components/cors/AllowedOriginRow';
export {default as CorsSection} from './components/cors/CorsSection';
export {default as DurationField} from './components/common/DurationField';
export type {DurationFieldProps} from './components/common/DurationField';
export {default as SectionLockNotice} from './components/common/SectionLockNotice';
export type {SectionLockNoticeProps} from './components/common/SectionLockNotice';
export {default as SettingField} from './components/common/SettingField';
export type {SettingFieldProps} from './components/common/SettingField';
export {default as SessionTimeoutsCard} from './components/sessions/SessionTimeoutsCard';

// Hooks
export {default as useAccountLockDraft} from './hooks/useAccountLockDraft';
export type {AccountLockDraft, AccountLockErrors} from './hooks/useAccountLockDraft';
export {default as useSessionTimeoutsDraft, isSessionDeclarative} from './hooks/useSessionTimeoutsDraft';
export type {SessionTimeoutErrors, SessionTimeoutsDraft} from './hooks/useSessionTimeoutsDraft';
export {default as useSettingsRoutes, defaultSettingsRoutePaths} from './hooks/useSettingsRoutes';
export type {SettingsRoutePaths} from './hooks/useSettingsRoutes';

// Constants
export {default as SettingsQueryKeys} from './constants/settings-query-keys';

// Models
export * from './models/accountAccess';
export * from './models/accountLifecycleFlows';
export * from './models/allowedOriginRow';
export * from './models/responses';
export * from './models/session';

// Pages
export {default as SettingsPage} from './pages/SettingsPage';

// Utils
export {buildAccountLockPayload, isSameAccountLockForm, toAccountLockForm} from './utils/accountLockPayload';
export type {AccountLockForm} from './utils/accountLockPayload';
export {default as describePolicy} from './utils/describePolicy';
export {DurationUnitSeconds, formatDurationShort, toDurationParts, toSeconds} from './utils/duration';
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
export {default as isRegexAnchored} from './utils/isRegexAnchored';
export {isValidOrigin, isValidRegex, normalizeOrigin} from './utils/origin';
export {default as originValueText} from './utils/originValueText';
export {default as validateAllowedOriginRows, AllowedOriginRowIssueFallbacks} from './utils/validateAllowedOriginRows';
export type {
  AllowedOriginRowError,
  AllowedOriginRowIssues,
  AllowedOriginRowWarning,
} from './utils/validateAllowedOriginRows';
