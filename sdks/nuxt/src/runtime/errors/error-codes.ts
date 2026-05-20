/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * Typed error codes for the ThunderID Nuxt SDK.
 * Every structured error thrown by the SDK carries one of these codes
 * so callers can react to specific failure modes without string matching.
 */
export enum ErrorCode {
  // ── Configuration ──────────────────────────────────────────────────
  ConfigMissingBaseUrl = 'config/missing-base-url',
  ConfigMissingClientId = 'config/missing-client-id',
  ConfigMissingSecret = 'config/missing-session-secret',

  // ── OAuth ──────────────────────────────────────────────────────────
  OAuthCallbackError = 'oauth/callback-error',
  OAuthStateInvalid = 'oauth/state-invalid',
  // ── Security ───────────────────────────────────────────────────────
  OpenRedirectBlocked = 'security/open-redirect-blocked',

  // ── Organization ───────────────────────────────────────────────────
  OrganizationCreateFailed = 'organization/create-failed',
  OrganizationSwitchFailed = 'organization/switch-failed',
  // ── Session ────────────────────────────────────────────────────────
  SessionExpired = 'session/expired',
  SessionInvalid = 'session/invalid',
  SessionMissing = 'session/missing',

  TempSessionInvalid = 'session/temp-invalid',
  TokenExchangeFailed = 'oauth/token-exchange-failed',
  TokenRefreshFailed = 'oauth/token-refresh-failed',
  // ── SCIM2 ──────────────────────────────────────────────────────────
  UserProfileFetchFailed = 'scim2/user-profile-fetch-failed',
  UserProfileUpdateFailed = 'scim2/user-profile-update-failed',
}
