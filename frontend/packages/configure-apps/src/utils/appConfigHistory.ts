// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {
  AppTheme,
  AppContentWidth,
  AppNavigationLayout,
  AppPageConfig,
  AppPageKey,
  AppPageVisibility,
} from '@thunderid/system-apps';
import type {BuilderAppConfig} from '../models/builder';

/**
 * The builder's working copy of a configuration, with undo and redo history.
 *
 * @public
 */
export interface AppConfigHistory {
  future: BuilderAppConfig[];
  past: BuilderAppConfig[];
  present: BuilderAppConfig;
  /**
   * The configuration as last saved, used to tell whether there are unsaved changes.
   */
  saved: BuilderAppConfig;
}

/**
 * The edits the builder can make.
 *
 * @public
 */
export type AppConfigAction =
  | {page: AppPageKey; type: 'togglePage'}
  | {page: AppPageKey; type: 'setVisibility'; visibility: AppPageVisibility}
  | {action: string; page: AppPageKey; type: 'toggleAction'}
  | {layout: AppNavigationLayout; type: 'setLayout'}
  | {type: 'setContentWidth'; width: AppContentWidth}
  | {appTheme: AppTheme | undefined; type: 'setAppTheme'}
  | {themeId: string | undefined; type: 'setGateTheme'}
  | {layoutId: string | undefined; type: 'setGateLayout'}
  | {type: 'toggleSecuredBy'}
  | {type: 'undo'}
  | {type: 'redo'}
  | {config: BuilderAppConfig; type: 'reset'};

const updatePage = (
  config: BuilderAppConfig,
  key: AppPageKey,
  update: (page: AppPageConfig) => AppPageConfig,
): BuilderAppConfig => ({
  ...config,
  pages: config.pages.map((page: AppPageConfig) => (page.key === key ? update(page) : page)),
});

const apply = (config: BuilderAppConfig, action: AppConfigAction): BuilderAppConfig => {
  switch (action.type) {
    case 'togglePage':
      return updatePage(config, action.page, (page: AppPageConfig) => ({...page, enabled: !page.enabled}));
    case 'setVisibility':
      return updatePage(config, action.page, (page: AppPageConfig) => ({...page, visibility: action.visibility}));
    case 'toggleAction':
      return updatePage(config, action.page, (page: AppPageConfig) => ({
        ...page,
        actions: {...page.actions, [action.action]: !page.actions[action.action]},
      }));
    case 'setLayout':
      return {...config, navigation: {...config.navigation, layout: action.layout}};
    case 'setContentWidth':
      return {...config, content: {width: action.width}};
    case 'setAppTheme':
      return {...config, appTheme: action.appTheme};
    case 'setGateTheme':
      return {...config, gate: {...config.gate, themeId: action.themeId}};
    case 'setGateLayout':
      return {...config, gate: {...config.gate, layoutId: action.layoutId}};
    case 'toggleSecuredBy':
      return {...config, branding: {...config.branding, showSecuredBy: !config.branding.showSecuredBy}};
    default:
      return config;
  }
};

/**
 * Starts a history at a saved configuration.
 *
 * @param config - The configuration as the server has it.
 * @returns A history with nothing to undo.
 */
export const createAppConfigHistory = (config: BuilderAppConfig): AppConfigHistory => ({
  future: [],
  past: [],
  present: config,
  saved: config,
});

/**
 * Applies a builder edit, keeping the previous state for undo. An edit that changes nothing is not
 * recorded.
 *
 * @param history - The current history.
 * @param action - The edit.
 * @returns The new history.
 */
export const appConfigHistoryReducer = (history: AppConfigHistory, action: AppConfigAction): AppConfigHistory => {
  if (action.type === 'reset') {
    return createAppConfigHistory(action.config);
  }

  if (action.type === 'undo') {
    const previous: BuilderAppConfig | undefined = history.past[history.past.length - 1];

    return previous
      ? {...history, future: [history.present, ...history.future], past: history.past.slice(0, -1), present: previous}
      : history;
  }

  if (action.type === 'redo') {
    const [next, ...rest] = history.future;

    return next ? {...history, future: rest, past: [...history.past, history.present], present: next} : history;
  }

  const present: BuilderAppConfig = apply(history.present, action);

  if (JSON.stringify(present) === JSON.stringify(history.present)) {
    return history;
  }

  return {...history, future: [], past: [...history.past, history.present], present};
};

/**
 * Whether the working copy differs from the saved configuration.
 *
 * @param history - The current history.
 * @returns `true` when there are unsaved changes.
 */
export const hasUnsavedChanges = (history: AppConfigHistory): boolean =>
  JSON.stringify(history.present) !== JSON.stringify(history.saved);
