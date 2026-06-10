/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import type {OAuthResponseMode} from './oauth-response';
import type {OIDCEndpoints} from './oidc-endpoints';
import {TokenEndpointAuthMethod} from './token-endpoint-auth';
import {RecursivePartial} from './utility-types';
import {ComponentsExtensions} from './extensions/components';
import {I18nBundle} from '../i18n/models/i18n';
import {ThemeConfig, ThemeMode} from '../theme/types';

/**
 * Interface representing the additional parameters to be sent in the sign-in request.
 * This can include custom parameters that your authorization server supports.
 * These parameters will be included in the authorization request sent to the server.
 * If not provided, no additional parameters will be sent.
 *
 * @example
 * signInOptions: { prompt: "login", fidp: "OrganizationSSO" }
 */
export type SignInOptions = Record<string, any>;

/**
 * Interface representing the additional parameters to be sent in the sign-out request.
 * This can include custom parameters that your authorization server supports.
 * These parameters will be included in the sign-out request sent to the server.
 * If not provided, no additional parameters will be sent.
 *
 * @example
 * signOutOptions: { idTokenHint: "your-id-token-hint" }
 */
export type SignOutOptions = Record<string, unknown>;

/**
 * Interface representing the additional parameters to be sent in the sign-up request.
 * This can include custom parameters that your authorization server supports.
 * These parameters will be included in the sign-up request sent to the server.
 * If not provided, no additional parameters will be sent.
 *
 * @example
 * signUpOptions: { appId: "your-app-id" }
 */
export type SignUpOptions = Record<string, unknown>;

export interface BaseConfig<T = unknown> extends WithPreferences, WithExtensions {
  /**
   * Whether to enable PKCE (Proof Key for Code Exchange) for the authorization request.
   * Defaults to `true`. Disable only if your authorization server does not support PKCE.
   *
   * @default true
   * @see {@link https://datatracker.ietf.org/doc/html/rfc7636}
   */
  enablePKCE?: boolean;

  /**
   * Optional prompt value to include in the authorization request.
   * Controls the authentication UI behavior (e.g., `"login"`, `"none"`, `"consent"`).
   */
  prompt?: string;

  /**
   * OAuth 2.0 response mode for the authorization request.
   * Controls how the authorization server delivers the response.
   *
   * @default 'query'
   */
  responseMode?: OAuthResponseMode;

  /**
   * Whether to include cookies in access-token, refresh-token, and custom-grant requests.
   * Set to `false` for cross-origin requests where cookies should not be forwarded.
   *
   * @default true
   */
  sendCookiesInRequests?: boolean;

  /**
   * Whether to include the ID token as a hint in the logout request.
   * When `true`, the `id_token_hint` parameter is sent to the end-session endpoint.
   */
  sendIdTokenInLogoutRequest?: boolean;

  /**
   * Optional URL where the authorization server should redirect after authentication.
   * This must match one of the allowed redirect URIs configured in your IdP.
   * If not provided, the framework layer will use the default redirect URL based on the application type.
   *
   * @example
   * For development: "http://localhost:3000/api/auth/callback"
   * For production: "https://your-app.com/api/auth/callback"
   */
  afterSignInUrl?: string | undefined;

  /**
   * Optional URL where the authorization server should redirect after sign out.
   * This must match one of the allowed post logout redirect URIs configured in your IdP
   * and is used to redirect the user after they have signed out.
   * If not provided, the framework layer will use the default sign out URL based on the
   *
   * @example
   * For development: "http://localhost:3000/api/auth/signout"
   * For production: "https://your-app.com/api/auth/signout"
   */
  afterSignOutUrl?: string | undefined;

  /**
   * A list of external API base URLs that the SDK is allowed to attach access tokens to when making HTTP requests.
   *
   * When making authenticated HTTP requests using the SDK's HTTP client, the access token will only be attached
   * to requests whose URLs start with one of these specified base URLs. This provides a security layer by
   * preventing tokens from being sent to unauthorized servers.
   *
   * @remarks
   * - This is only applicable when the storage type is `webWorker`.
   * - Each URL should be a base URL without trailing slashes (e.g., "https://api.example.com").
   * - The SDK will check if the request URL starts with any of these base URLs before attaching the token.
   * - If a request is made to a URL that doesn't match any of these base URLs, an error will be thrown.
   *
   * @example
   * allowedExternalUrls: ["https://api.example.com", "https://api.another-service.com"]
   */
  allowedExternalUrls?: string[];

  /**
   * Optional UUID of the ThunderID application.
   * This is used to identify the application in the ThunderID identity server for Application Branding,
   * obtaining the access URL in the sign-up flow, etc.
   * If not provided, the framework layer will use the default application ID based on the application.
   */
  applicationId?: string | undefined;

  /**
   * The base URL of the ThunderID identity server.
   * Example: "https://localhost:8090"
   */
  baseUrl: string | undefined;

  /**
   * The client ID obtained from the ThunderID application registration.
   * This is used to identify your application during authentication.
   */
  clientId?: string | undefined;

  /**
   * Optional client secret for the application.
   * Only required when using confidential client flows.
   * Not recommended for public clients like browser applications.
   */
  clientSecret?: string | undefined;

  /**
   * OpenID Connect discovery configuration.
   * Controls how the SDK resolves endpoint URLs from the authorization server.
   * Each discovery mechanism is independently configurable.
   *
   * @example
   * // Use a custom well-known discovery document URL
   * discovery: { wellKnown: { endpoint: "https://custom.example.com/.well-known/openid-configuration" } }
   *
   * @example
   * // Disable well-known discovery entirely
   * discovery: { wellKnown: { enabled: false } }
   */
  discovery?: {
    /**
     * Configuration for OpenID Connect Discovery via the well-known endpoint (RFC 8414).
     * The SDK fetches `{baseUrl}/oauth2/token/.well-known/openid-configuration` by default.
     */
    wellKnown?: {
      /**
       * Whether to fetch and use the well-known discovery document to resolve endpoint URLs.
       * @default true
       */
      enabled?: boolean;
    };
  };

  /**
   * Optional overrides for the OIDC protocol endpoints.
   * By default, the SDK derives all endpoint URLs from the well-known discovery document
   * located at `{baseUrl}/oauth2/token/.well-known/openid-configuration`.
   * Use this when your authorization server exposes endpoints at non-standard paths,
   * or when a custom domain differs from `baseUrl`.
   *
   * Individual overrides take precedence over values resolved from the discovery document.
   *
   * @example
   * endpoints: {
   *   wellKnown: "https://custom-domain.example.com/.well-known/openid-configuration",
   *   authorization: "https://custom-domain.example.com/oauth2/authorize",
   * }
   */
  endpoints?: {
    /**
     * The authorization endpoint URL.
     * If not provided, resolved from the well-known discovery document.
     */
    authorization?: string;
    /**
     * The end-session (logout) endpoint URL.
     * If not provided, resolved from the well-known discovery document.
     */
    endSession?: string;
    /**
     * The introspection endpoint URL.
     * If not provided, resolved from the well-known discovery document.
     */
    introspection?: string;
    /**
     * The JSON Web Key Set (JWKS) endpoint URL used to fetch public keys for token verification.
     * If not provided, resolved from the well-known discovery document.
     */
    jwks?: string;
    /**
     * The token endpoint URL.
     * If not provided, resolved from the well-known discovery document.
     */
    token?: string;
    /**
     * The userinfo endpoint URL.
     * If not provided, resolved from the well-known discovery document.
     */
    userInfo?: string;
    /**
     * The OpenID Connect discovery document URL.
     * Defaults to `{baseUrl}/oauth2/token/.well-known/openid-configuration`.
     */
    wellKnown?: string;
  };

  /**
   * Optional instance ID for multi-auth context support.
   * Use this when you need multiple authentication contexts in the same application.
   */
  instanceId?: number;

  /**
   * Authentication interaction mode.
   *
   * - `'redirect'` (default) — standard OAuth 2.0 authorization-code redirect flow.
   * - `'embedded'` — app-native embedded flow; the server SDK drives the step-by-step
   *   authentication without a browser redirect to the identity provider.
   *
   * @default 'redirect'
   */
  mode?: 'redirect' | 'embedded';

  /**
   * Configuration for chaining authentication across multiple organization contexts.
   * Used when you need to authenticate a user in one organization using credentials
   * from another organization context.
   */
  organizationChain?: {
    /**
     * Instance ID of the source organization context to retrieve access token from for organization token exchange.
     * Used in linked organization scenarios to automatically fetch the source organization's access token.
     */
    sourceInstanceId?: number;
    /**
     * Organization ID for the target organization.
     * When provided with sourceInstanceId, triggers automatic organization token exchange.
     */
    targetOrganizationId?: string;
  };

  /**
   * Optional organization handle for the Organization in ThunderID.
   * This is used to identify the organization in the ThunderID identity server in cases like Branding, etc.
   * If not provided, the framework layer will try to use the `baseUrl` to determine the organization handle.
   * @remarks This is mandatory if a custom domain is configured for the ThunderID organization.
   */
  organizationHandle?: string | undefined;

  /**
   * The scopes to request during authentication.
   * Accepts either a space-separated string or an array of strings.
   *
   * These define what access the token should grant (e.g., openid, profile, email).
   * If not provided, defaults to `["openid"]`.
   *
   * @example
   * scopes: "openid profile email"
   * @example
   * scopes: ["openid", "profile", "email"]
   */
  scopes?: string | string[] | undefined;

  /**
   * Optional additional parameters to be sent in the authorize request.
   * @see {@link SignInOptions} for more details.
   */
  signInOptions?: SignInOptions;

  /**
   * Optional URL to redirect the user to sign-in.
   * By default, this will be the sign-in page of ThunderID.
   * If you want to use a custom sign-in page, you can provide the URL here and use the `SignIn` component to render it.
   */
  signInUrl?: string | undefined;

  /**
   * Optional additional parameters to be sent in the sign-out request.
   * @see {@link SignOutOptions} for more details.
   */
  signOutOptions?: SignOutOptions;

  /**
   * Optional additional parameters to be sent in the sign-up request.
   * @see {@link SignUpOptions} for more details.
   */
  signUpOptions?: SignUpOptions;

  /**
   * Optional URL to redirect the user to sign-up.
   * By default, this will be the sign-up page of ThunderID.
   * If you want to use a custom sign-up page, you can provide the URL here
   * and use the `SignUp` component to render it.
   */
  signUpUrl?: string | undefined;

  /**
   * Storage mechanism to use for storing tokens and session data.
   * The values should be defined at the framework layer.
   */
  storage?: T;

  /**
   * Flag to indicate whether the Application session should be synchronized with the IdP session.
   * @remarks This uses the OIDC iframe base session management feature to keep the application session in sync with the IdP session.
   * WARNING: This may not work in all browsers due to 3rd party cookie restrictions.
   * It is recommended to use this feature only if you are aware of the implications and have tested it in your target browsers.
   * If you are not sure, it is safer to leave this option as `false`.
   * @example
   * syncSession: true
   * @see {@link https://openid.net/specs/openid-connect-session-management-1_0.html#IframeBasedSessionManagement}
   */
  syncSession?: boolean;

  /**
   * Configuration for token lifecycle management.
   */
  tokenLifecycle?: {
    /**
     * Configuration for refresh token behavior.
     */
    refreshToken?: {
      /**
       * Whether to automatically refresh the access token periodically before it expires.
       */
      autoRefresh?: boolean;
    };
  };

  /**
   * Configuration for the token endpoint request.
   */
  tokenRequest?: {
    /**
     * OAuth 2.0 client authentication method used at the token endpoint.
     * Maps to `token_endpoint_auth_method` in OIDC Discovery.
     *
     * - `client_secret_basic` — Credentials in the `Authorization: Basic` header.
     * - `client_secret_post` — Credentials in the POST body.
     * - `none` — No client authentication (public clients).
     *
     * When omitted the SDK applies its platform-based default:
     * ThunderIDV2 → `client_secret_basic`; all others → `client_secret_post`.
     */
    authMethod?: TokenEndpointAuthMethod;
    /**
     * Optional additional parameters to be sent in the token request body.
     * Appended to the token endpoint POST body alongside the standard OAuth parameters.
     *
     * @example
     * params: { resource: "https://api.example.com", audience: "my-api" }
     */
    params?: Record<string, unknown>;
  };

  /**
   * Token validation configuration.
   * This allows you to configure how the SDK validates tokens received from the authorization server.
   * It includes options for ID token validation, such as whether to validate the token,
   * whether to validate the issuer, and the allowed clock tolerance for token validation.
   * If not provided, the SDK will use default validation settings.
   */
  tokenValidation?: {
    /**
     * ID token validation config.
     */
    idToken?: {
      /**
       * Allowed leeway for ID tokens (in seconds).
       */
      clockTolerance?: number;
      /**
       * Whether to validate ID tokens.
       */
      validate?: boolean;
      /**
       * Whether to validate the issuer of ID tokens.
       */
      validateIssuer?: boolean;
    };
  };
}

export interface WithPreferences {
  /**
   * Preferences for customizing the ThunderID UI components
   */
  preferences?: Preferences;
}

export interface Extensions {
  /**
   * Extension configuration for flow component rendering.
   */
  components?: ComponentsExtensions;
}

export interface WithExtensions {
  /**
   * Extensions for customizing SDK behavior at defined integration points.
   */
  extensions?: Extensions;
}

export type Config<T = unknown> = BaseConfig<T>;

export interface ThemePreferences {
  /**
   * The text direction for the UI.
   * @default 'ltr'
   */
  direction?: 'ltr' | 'rtl';
  /**
   * Inherit branding from WSO2 Identity Server or ThunderID.
   * When set to `true`, the SDK will fetch and apply branding preferences from the server.
   * Defaults to `false` — branding is not fetched unless explicitly enabled.
   * @default false
   */
  inheritFromBranding?: boolean;
  /**
   * The theme mode to use. Defaults to 'system'.
   */
  mode?: ThemeMode;
  /**
   * Theme overrides to customize the default theme
   */
  overrides?: RecursivePartial<ThemeConfig>;
}

/**
 * The storage strategy to use for persisting the user's language selection.
 *
 * - `'cookie'`       — persists in `document.cookie` as a domain cookie (default).
 *                      Useful for cross-subdomain scenarios where the auth portal and
 *                      the application share a root domain.
 * - `'localStorage'` — persists in `window.localStorage`.
 * - `'none'`         — no persistence; the resolved language is held in React state only.
 */
export type I18nStorageStrategy = 'cookie' | 'localStorage' | 'none';

export interface I18nPreferences {
  /**
   * Custom translations to override default ones.
   */
  bundles?: Record<string, I18nBundle>;
  /**
   * The domain to use when setting the language cookie.
   * Only applies when `storageStrategy` is `'cookie'`.
   * Defaults to the root domain derived from `window.location.hostname`
   * (e.g. `'app.example.com'` → `'example.com'`).
   * Override this for eTLD+1 domains like `.co.uk` or custom cookie scoping.
   */
  cookieDomain?: string;
  /**
   * The fallback language to use if translations are not available in the specified language.
   * Defaults to 'en-US'.
   */
  fallbackLanguage?: string;
  /**
   * The language to use for translations.
   * When set, acts as a hard override and bypasses all other detection sources
   * (URL param, stored preference, browser language).
   */
  language?: string;
  /**
   * The key used when reading/writing the language to the chosen storage.
   * For `localStorage` this is the key name; for `cookie` this is the cookie name.
   * @default 'thunderid-i18n-language'
   */
  storageKey?: string;
  /**
   * The storage strategy to use for persisting the user's language selection.
   * @default 'cookie'
   */
  storageStrategy?: I18nStorageStrategy;
  /**
   * The URL query-parameter name to inspect for a language override.
   * Set to `false` to disable URL-parameter detection entirely.
   * When a URL param is detected its value is immediately persisted to storage.
   * @default 'lang'
   * @example
   * // With urlParam: 'locale', the URL ?locale=fr-FR will select French.
   * // With urlParam: false, URL parameters are ignored.
   */
  urlParam?: string | false;
}

export interface UserPreferences {
  /**
   * Whether to automatically fetch the user's associated organizations after sign-in.
   * When set to false, the SDK will not make API calls to `/api/users/v1/me/organizations`.
   * @default true
   * @remarks Disabling this will improve performance if you don't need organization information.
   * You can manually call `getMyOrganizations()` when needed if this is disabled.
   */
  fetchOrganizations?: boolean;
  /**
   * Whether to automatically fetch the user profile from SCIM2 endpoints after sign-in.
   * When set to false, the SDK will not make API calls to `/scim2/Me` and `/scim2/Schemas`.
   * Instead, it will extract basic user claims from the ID token.
   * @default true
   * @remarks Disabling this will improve performance but provide limited user profile information.
   * Only the claims present in the ID token will be available (e.g., sub, email, name).
   * For full user profile attributes (custom claims, enterprise attributes, etc.),
   * keep this enabled or manually call `getUserProfile()` when needed.
   */
  fetchUserProfile?: boolean;
}

export interface Preferences {
  /**
   * Internationalization preferences for the ThunderID UI components
   */
  i18n?: I18nPreferences;
  /**
   * Whether to resolve the theme from the Flow Meta API (GET /flow/meta).
   * @remarks This is only applicable when using platform `ThunderID V2` (Thunder).
   */
  resolveFromMeta?: boolean;
  /**
   * Theme preferences for the ThunderID UI components
   */
  theme?: ThemePreferences;
  /**
   * User profile preferences for controlling user data fetching behavior.
   * TEMPORARY CONFIG
   */
  user?: UserPreferences;
}

/**
 * Full client configuration type, combining all base options with an optional
 * framework-specific extension type `T`.
 *
 * @typeParam T - Optional extension type for framework-specific config fields.
 */
export type AuthClientConfig<T = unknown> = Config<T>;

/**
 * Alias for the strict (non-extended) client configuration.
 * Equivalent to `Config` with no extension type.
 */
export type StrictAuthClientConfig = Config;

/**
 * Alias for the default client configuration fields.
 * Equivalent to `Config` with no extension type.
 */
export type DefaultAuthClientConfig = Config;

/**
 * Config variant for clients that discover endpoints via the well-known document.
 * The `endpoints.wellKnown` field specifies the discovery URL.
 */
export type WellKnownAuthClientConfig = Config & {endpoints?: {wellKnown?: string}};

/**
 * Config variant for clients that derive endpoints from a `baseUrl`.
 */
export type BaseURLAuthClientConfig = Config & {baseUrl: string};

/**
 * Config variant for clients that specify OIDC endpoints explicitly.
 */
export type ExplicitAuthClientConfig = Config & {endpoints: Partial<OIDCEndpoints>};
