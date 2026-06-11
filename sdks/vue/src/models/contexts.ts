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
  AllOrganizationsApiResponse,
  BrandingPreference,
  CreateOrganizationPayload,
  FlowMetadataResponse,
  HttpRequestConfig,
  HttpResponse,
  IdToken,
  Organization,
  Platform,
  Schema,
  SignInOptions,
  StorageManager,
  Theme,
  TokenExchangeRequestConfig,
  TokenResponse,
  UpdateMeProfileConfig,
  User,
  UserProfile,
  I18nBundle,
} from '@thunderid/browser';
import type {Ref} from 'vue';
import type {ThunderIDVueConfig} from './config';
import type ThunderIDVueClient from '../ThunderIDVueClient';

/**
 * Shape of the core ThunderID context provided via `provide`/`inject`.
 *
 * Reactive refs are exposed as `Readonly<Ref<T>>` so consumers can read
 * them in templates and `watch()` calls but cannot mutate them directly.
 */
export interface ThunderIDContext {
  /** The `afterSignInUrl` from the config. */
  afterSignInUrl: string | undefined;
  /** The ThunderID application ID from the config. */
  applicationId: string | undefined;
  /** The base URL of the ThunderID tenant. */
  baseUrl: string | undefined;
  clearSession: (...args: any[]) => void;
  /** The OAuth2 client ID. */
  clientId: string | undefined;
  /** The OAuth2 scopes requested during flow initialization. */
  scopes: string | string[] | undefined;

  exchangeToken: (config: TokenExchangeRequestConfig) => Promise<TokenResponse | Response>;
  // ── Token ──
  getAccessToken: () => Promise<string>;
  getDecodedIdToken: () => Promise<IdToken>;
  getIdToken: () => Promise<string>;
  getStorageManager: () => StorageManager<any>;
  // ── HTTP ──
  http: {
    request: (requestConfig?: HttpRequestConfig) => Promise<HttpResponse<any>>;
    requestAll: (requestConfigs?: HttpRequestConfig[]) => Promise<HttpResponse<any>[]>;
  };

  /** The instance ID for multi-instance support. */
  instanceId: number;
  // ── Reactive Auth State ──
  /** Whether the SDK has finished initializing. */
  isInitialized: Readonly<Ref<boolean>>;
  /** Whether the SDK is performing a background operation. */
  isLoading: Readonly<Ref<boolean>>;
  /** Whether the user is currently signed in. */
  isSignedIn: Readonly<Ref<boolean>>;

  // ── FlowMeta (injected by useThunderID) ──
  /** Flow metadata from the FlowMeta context, or `null` while loading/unavailable. */
  meta?: Readonly<Ref<FlowMetadataResponse | null>>;
  /** The current organization, or `null`. */
  organization: Readonly<Ref<Organization | null>>;
  organizationHandle: string | undefined;
  platform: Platform | undefined;

  // ── Lifecycle ──
  reInitialize: (config: Partial<ThunderIDVueConfig>) => Promise<boolean>;

  /** Resolve `{{t(...)}}` and `{{meta(...)}}` template literals inside a string. */
  resolveFlowTemplateLiterals?: (text: string | undefined) => string;

  // ── Auth Actions ──
  signIn: (...args: any[]) => Promise<any>;
  // ── Config ──
  signInOptions: SignInOptions | undefined;

  signInSilently: (options?: SignInOptions) => Promise<any>;
  signInUrl: string | undefined;
  signOut: (...args: any[]) => Promise<any>;
  signUp: (...args: any[]) => Promise<any>;
  signUpUrl: string | undefined;
  storage: ThunderIDVueConfig['storage'] | undefined;

  // ── Organization ──
  switchOrganization: ThunderIDVueClient['switchOrganization'];

  /** The current user object, or `null` if not signed in. */
  user: Readonly<Ref<any | null>>;
}

// ─────────────────────────────────────────────────────────────────────────────
// User Context
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Shape of the User context exposed by `useUser()`.
 */
export interface UserContextValue {
  /** The flattened user profile (top-level attribute map). */
  flattenedProfile: Readonly<Ref<User | null>>;
  /** Called after a successful profile update to sync state up to ThunderIDProvider. */
  onUpdateProfile: (payload: User) => void;
  /** The raw nested user profile from the SCIM2/ME endpoint. */
  profile: Readonly<Ref<UserProfile | null>>;
  /** Refetch the user profile from the server. */
  revalidateProfile: () => Promise<void>;
  /** The SCIM2 schemas describing the user profile attributes. */
  schemas: Readonly<Ref<Schema[] | null>>;
  /**
   * Update the user profile. Accepts the standard SCIM2 patch request config.
   */
  updateProfile: (
    requestConfig: UpdateMeProfileConfig,
    sessionId?: string,
  ) => Promise<{data: {user: User}; error: string; success: boolean}>;
}

// ─────────────────────────────────────────────────────────────────────────────
// Organization Context
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Shape of the Organization context exposed by `useOrganization()`.
 */
export interface OrganizationContextValue {
  /** Optional function to create a new sub-organization. */
  createOrganization?: (payload: CreateOrganizationPayload, sessionId: string) => Promise<Organization>;
  /** The organization the user is currently operating in. */
  currentOrganization: Readonly<Ref<Organization | null>>;
  /** Last error message from an organization operation, if any. */
  error: Readonly<Ref<string | null>>;
  /** Fetch all organizations (paginated). */
  getAllOrganizations: () => Promise<AllOrganizationsApiResponse>;
  /** Whether an organization operation is in-flight. */
  isLoading: Readonly<Ref<boolean>>;
  /** The list of organizations the signed-in user is a member of. */
  myOrganizations: Readonly<Ref<Organization[]>>;
  /** Re-fetch the user's organization list from the server. */
  revalidateMyOrganizations: () => Promise<Organization[]>;
  /** Switch to the given organization (performs token exchange). */
  switchOrganization: (organization: Organization) => Promise<void>;
}

// ─────────────────────────────────────────────────────────────────────────────
// Flow Context
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Types of authentication flow steps that can be displayed.
 */
export type FlowStep = {
  canGoBack?: boolean;
  id: string;
  metadata?: Record<string, any>;
  subtitle?: string;
  title: string;
  type: 'signin' | 'signup' | 'organization-signin' | 'forgot-password' | 'reset-password' | 'verify-email' | 'mfa';
} | null;

/**
 * A message that can be displayed inside an authentication flow UI.
 */
export interface FlowMessage {
  dismissible?: boolean;
  id?: string;
  message: string;
  type: 'success' | 'error' | 'warning' | 'info';
}

/**
 * Shape of the Flow context exposed by `useFlow()`.
 */
export interface FlowContextValue {
  addMessage: (message: FlowMessage) => void;
  clearMessages: () => void;
  currentStep: Readonly<Ref<FlowStep>>;
  error: Readonly<Ref<string | null>>;
  isLoading: Readonly<Ref<boolean>>;
  messages: Readonly<Ref<FlowMessage[]>>;
  navigateToFlow: (
    flowType: NonNullable<FlowStep>['type'],
    options?: {metadata?: Record<string, any>; subtitle?: string; title?: string},
  ) => void;
  onGoBack: Readonly<Ref<(() => void) | undefined>>;
  removeMessage: (messageId: string) => void;
  reset: () => void;
  setCurrentStep: (step: FlowStep) => void;
  setError: (error: string | null) => void;
  setIsLoading: (loading: boolean) => void;
  setOnGoBack: (callback?: () => void) => void;
  setShowBackButton: (show: boolean) => void;
  setSubtitle: (subtitle?: string) => void;
  setTitle: (title: string) => void;
  showBackButton: Readonly<Ref<boolean>>;
  subtitle: Readonly<Ref<string | undefined>>;
  title: Readonly<Ref<string>>;
}

// ─────────────────────────────────────────────────────────────────────────────
// FlowMeta Context
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Shape of the FlowMeta context exposed by `useFlowMeta()`.
 */
export interface FlowMetaContextValue {
  /** Error from the flow metadata fetch, if any. */
  error: Readonly<Ref<Error | null>>;
  /** Manually re-fetch flow metadata from the server. */
  fetchFlowMeta: () => Promise<void>;
  /** Whether the flow metadata is currently being fetched. */
  isLoading: Readonly<Ref<boolean>>;
  /** The fetched `FlowMetadataResponse`, or `null` while loading or on error. */
  meta: Readonly<Ref<FlowMetadataResponse | null>>;
  /**
   * Fetch flow metadata for the given language and activate it in the i18n system.
   * Use this to switch the UI language at runtime.
   */
  switchLanguage: (language: string) => Promise<void>;
}

// ─────────────────────────────────────────────────────────────────────────────
// Theme Context
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Shape of the Theme context exposed by `useTheme()`.
 */
export interface ThemeContextValue {
  /** Error from the branding theme fetch, if any. */
  brandingError: Readonly<Ref<Error | null>>;
  /** The current color scheme ('light' | 'dark'). */
  colorScheme: Readonly<Ref<'light' | 'dark'>>;
  /** The text direction for the UI. */
  direction: Readonly<Ref<'ltr' | 'rtl'>>;
  /** Whether the theme inherits from ThunderID branding preferences. */
  inheritFromBranding: boolean;
  /** Whether the branding theme is currently loading. */
  isBrandingLoading: Readonly<Ref<boolean>>;
  /** The resolved Theme object used by all styled components. */
  theme: Readonly<Ref<Theme>>;
  /** Toggle between light and dark mode. */
  toggleTheme: () => void;
}

// ─────────────────────────────────────────────────────────────────────────────
// Branding Context
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Shape of the Branding context exposed by `useBranding()`.
 */
export interface BrandingContextValue {
  /** The active theme from the branding preference ('light' | 'dark'), or null. */
  activeTheme: Readonly<Ref<'light' | 'dark' | null>>;
  /** The raw branding preference data from the server. */
  brandingPreference: Readonly<Ref<BrandingPreference | null>>;
  /** Error from the branding fetch, if any. */
  error: Readonly<Ref<Error | null>>;
  /** Trigger a branding preference fetch (deduplicated). */
  fetchBranding: () => Promise<void>;
  /** Whether the branding preference is currently loading. */
  isLoading: Readonly<Ref<boolean>>;
  /** Force a fresh branding preference fetch (bypasses dedup). */
  refetch: () => Promise<void>;
  /** The transformed `Theme` object derived from the branding preference. */
  theme: Readonly<Ref<Theme | null>>;
}

// ─────────────────────────────────────────────────────────────────────────────
// I18n Context
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Shape of the I18n context exposed by `useI18n()`.
 */
export interface I18nContextValue {
  /** All available i18n bundles (default + injected + user-provided). */
  bundles: Readonly<Ref<Record<string, I18nBundle>>>;
  /** The current language code (e.g., 'en-US'). */
  currentLanguage: Readonly<Ref<string>>;
  /** The fallback language code. */
  fallbackLanguage: string;
  /**
   * Inject additional bundles into the i18n system (e.g., from flow metadata).
   * Injected bundles take precedence over defaults but are overridden by prop-provided bundles.
   */
  injectBundles: (bundles: Record<string, I18nBundle>) => void;
  /** Change the current language. */
  setLanguage: (language: string) => void;
  /** Translate a key with optional named parameters. */
  t: (key: string, params?: Record<string, string | number>) => string;
}
