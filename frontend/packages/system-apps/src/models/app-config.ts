// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The apps the Console builds. Data planes serve them; the Console only stores their configuration.
 *
 * @public
 */
export type AppType = 'ADMIN' | 'ACCOUNT';

/**
 * A page of the Admin App.
 *
 * @public
 */
export type AdminAppPageKey = 'users' | 'groups' | 'roles' | 'agents';

/**
 * A page of the Account App.
 *
 * @public
 */
export type AccountAppPageKey = 'profile' | 'security';

/**
 * A page of any app the Console builds.
 *
 * @public
 */
export type AppPageKey = AdminAppPageKey | AccountAppPageKey;

/**
 * Who sees a page: every admin of the organization unit, or only admins whose token carries the
 * page's view permission.
 *
 * @public
 */
export type AppPageVisibility = 'ALL' | 'BY_PERMISSION';

/**
 * How the app's pages are navigated.
 *
 * @public
 */
export type AppNavigationLayout = 'TOP_BAR' | 'SIDE_BAR';

/**
 * How wide the app's pages are: centered in a column, or the full width of the window.
 *
 * @public
 */
export type AppContentWidth = 'CONTAINED' | 'FULL';

/**
 * The configuration of one page.
 *
 * @public
 */
export interface AppPageConfig {
  /**
   * The actions the page offers, keyed by action name. An action missing from the map is off.
   */
  actions: Record<string, boolean>;
  enabled: boolean;
  key: AppPageKey;
  visibility: AppPageVisibility;
}

/**
 * The colors of one color scheme of an app theme. A color left out keeps the app's own.
 *
 * @public
 */
export interface AppThemeColors {
  background?: string;
  border?: string;
  primary?: string;
  surface?: string;
  textPrimary?: string;
  textSecondary?: string;
}

/**
 * How an app and the SDK components it is built from look, as read from the Design theme the app
 * takes.
 *
 * @public
 */
export interface AppTheme {
  /** The corner radius of the app's controls, in pixels. */
  borderRadius?: number;
  colorSchemes?: {dark?: AppThemeColors; light?: AppThemeColors};
  fontFamily?: string;
}

/**
 * A branding image for each color scheme, as http or https URLs. The dark scheme takes the light
 * image when it has none of its own.
 *
 * @public
 */
export interface AppImage {
  dark?: string;
  light?: string;
}

/**
 * A link at the foot of an app's pages.
 *
 * @public
 */
export interface AppLink {
  label: string;
  /** An http or https URL. */
  url: string;
}

/**
 * The language menu of an app.
 *
 * @public
 */
export interface AppLanguages {
  /** The language the app opens in until its user picks another. One of `offered`. */
  default?: string;
  /** Whether the app's header shows a menu to switch its language. */
  menu: boolean;
  /** The language tags the menu lists, such as `en-US`. */
  offered?: string[];
}

/**
 * The configuration of an app: the pages it shows, who sees them, the actions they allow, its
 * layout, theme and branding.
 *
 * @public
 */
export interface AppConfig {
  branding: {
    /**
     * The icon the browser shows for the app. When not set, the app shows its own.
     */
    favicon?: AppImage;
    /**
     * The logo the app shows in its header. When not set, the app shows its application's logo.
     */
    logo?: AppImage;
    /**
     * The pages the app links to at the foot of every page, such as privacy and terms.
     */
    links?: AppLink[];
    showSecuredBy: boolean;
    title?: string;
  };
  content: {
    width: AppContentWidth;
  };
  /**
   * The app's language menu. When not set, the app has none.
   */
  languages?: AppLanguages;
  navigation: {
    layout: AppNavigationLayout;
  };
  pages: AppPageConfig[];
  /**
   * The Design theme the app takes. When not set, the app follows the Gate theme of the
   * application it signs in with.
   */
  themeId?: string;
  type: AppType;
}
