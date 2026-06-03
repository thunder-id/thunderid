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

import type {
  BrandingPreference,
  I18nPreferences,
  Organization,
  Platform,
  TokenEndpointAuthMethod,
  User,
  UserProfile,
} from '@thunderid/node';
import type {JWTPayload} from 'jose';

/**
 * Configuration for the ThunderID Nuxt module.
 */
export interface ThunderIDNuxtConfig {
  /** URL to redirect to after sign-in (default: '/') */
  afterSignInUrl?: string;
  /** URL to redirect to after sign-out (default: '/') */
  afterSignOutUrl?: string;
  /**
   * ThunderID application id (`spId`) — appended to the redirect-based sign-up
   * URL when present. Mirrors `applicationId` in the React/Next.js SDKs.
   */
  applicationId?: string;
  /** Base URL of the ThunderID org tenant (e.g. https://localhost:8090) */
  baseUrl?: string;
  /** OAuth2 Client ID */
  clientId?: string;
  /** OAuth2 Client Secret (server-only, use THUNDERID_CLIENT_SECRET env var) */
  clientSecret?: string;
  /**
   * Identity platform variant. Set to `Platform.ThunderID` when connecting to
   * a Thunder (ThunderIDV2) instance. Forwarded to the underlying Node client so
   * platform-specific behaviours (e.g. issuer resolution) apply correctly.
   */
  platform?: keyof typeof Platform;
  /**
   * Feature-gating preferences that control which server-side data fetches
   * the Nitro plugin performs on every SSR request.
   */
  preferences?: {
    /** i18n configuration forwarded to `I18nProvider`. */
    i18n?: I18nPreferences;
    theme?: {
      /**
       * When true (default), the Nitro plugin fetches the branding preference
       * from ThunderID and passes it to `BrandingProvider` / `ThemeProvider`.
       */
      inheritFromBranding?: boolean;
      /**
       * Theme mode forwarded to the Vue SDK's `ThemeProvider`.
       * - `'light'` (default) | `'dark'`: Fixed color scheme. Toggle at runtime with `useTheme().toggleTheme()`.
       * - `'system'`: Follows the OS `prefers-color-scheme`.
       * - `'class'`: Reads a CSS class on `<html>` (works well with Tailwind dark-mode).
       * - `'branding'`: Follows the active theme from the tenant's branding preference.
       */
      mode?: 'light' | 'dark' | 'system' | 'class' | 'branding';
    };
    user?: {
      /** Whether to fetch the user's organisations during SSR (default: true). */
      fetchOrganizations?: boolean;
      /** Whether to fetch the SCIM2 user profile during SSR (default: true). */
      fetchUserProfile?: boolean;
    };
  };
  /** OAuth2 scopes to request */
  scopes?: string | string[];
  /** Secret for signing session JWTs (use THUNDERID_SESSION_SECRET env var) */
  sessionSecret?: string;
  /**
   * Optional override for the redirect-based sign-in URL. Reserved for
   * parity with the React/Next.js SDKs; not currently used by the redirect
   * flow (which goes through `/api/auth/signin`).
   */
  signInUrl?: string;
  /**
   * Optional override for the redirect-based sign-up URL. When set,
   * `<ThunderIDSignUpButton>` and `useThunderID().signUp()` (no-arg) navigate
   * here instead of deriving the URL from `baseUrl`/`clientId`.
   */
  signUpUrl?: string;
  /**
   * Configuration for the token endpoint request.
   */
  tokenRequest?: {
    /**
     * OAuth 2.0 client authentication method used at the token endpoint.
     * Defaults to `client_secret_basic` for ThunderIDV2 and `client_secret_post`
     * for all other platforms when not specified.
     */
    authMethod?: TokenEndpointAuthMethod;
  };
}

/**
 * Payload stored in the session JWT cookie.
 */
export interface ThunderIDSessionPayload extends JWTPayload {
  accessToken: string;
  /** Unix timestamp (seconds) when the access token expires. Used for proactive refresh. */
  accessTokenExpiresAt?: number;
  exp: number;
  iat: number;
  /** Raw ID token string (for userinfo derivation without in-memory store). */
  idToken?: string;
  organizationId?: string;
  /** Refresh token for obtaining new access tokens without re-authentication. */
  refreshToken?: string;
  scopes: string;
  sessionId: string;
  sub: string;
}

/**
 * Payload stored in the temporary session JWT cookie (during OAuth flow).
 */
export interface ThunderIDTempSessionPayload extends JWTPayload {
  /** URL to redirect to after successful sign-in */
  returnTo?: string;
  sessionId: string;
  type: 'temp';
}

/**
 * Full SSR payload resolved by the Nitro plugin on each page request.
 * Written to `event.context.thunderid.ssr` and subsequently seeded into
 * hydrated `useState` keys so the client never re-fetches on first render.
 */
export interface ThunderIDSSRData {
  /** Branding preference fetched from ThunderID (null when `preferences.theme.inheritFromBranding` is false). */
  brandingPreference: BrandingPreference | null;
  /** The organisation the user is currently acting within (null when not in an org). */
  currentOrganization: Organization | null;
  isSignedIn: boolean;
  /** All organisations the user is a member of (empty array when `preferences.user.fetchOrganizations` is false). */
  myOrganizations: Organization[];
  /**
   * The base URL actually used for this request.
   * Equals `${baseUrl}/o` when the user is acting within an organisation
   * (derived from the `user_org` claim in the ID token), otherwise equals
   * the configured `baseUrl`.
   */
  resolvedBaseUrl: string | null;
  session: ThunderIDSessionPayload | null;
  user: User | null;
  /** Flattened SCIM2 profile + raw profile + schemas (null when `preferences.user.fetchUserProfile` is false). */
  userProfile: UserProfile | null;
}

/**
 * Auth state hydrated from server to client via useState.
 */
export interface ThunderIDAuthState {
  isLoading: boolean;
  isSignedIn: boolean;
  user: User | null;
}
