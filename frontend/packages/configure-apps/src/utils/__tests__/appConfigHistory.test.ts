// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DEFAULT_ADMIN_APP_CONFIG} from '@thunderid/system-apps';
import type {AppPageConfig} from '@thunderid/system-apps';
import {describe, expect, it} from 'vitest';
import {
  appConfigHistoryReducer,
  createAppConfigHistory,
  hasUnsavedChanges,
  type AppConfigHistory,
} from '../appConfigHistory';

const page = (history: AppConfigHistory, key: string): AppPageConfig =>
  history.present.pages.find((candidate: AppPageConfig) => candidate.key === key)!;

describe('appConfigHistoryReducer', () => {
  it('records an edit so it can be undone and redone', () => {
    const start: AppConfigHistory = createAppConfigHistory(DEFAULT_ADMIN_APP_CONFIG);

    const edited: AppConfigHistory = appConfigHistoryReducer(start, {page: 'groups', type: 'togglePage'});
    expect(page(edited, 'groups').enabled).toBe(false);
    expect(hasUnsavedChanges(edited)).toBe(true);

    const undone: AppConfigHistory = appConfigHistoryReducer(edited, {type: 'undo'});
    expect(page(undone, 'groups').enabled).toBe(true);
    expect(hasUnsavedChanges(undone)).toBe(false);

    const redone: AppConfigHistory = appConfigHistoryReducer(undone, {type: 'redo'});
    expect(page(redone, 'groups').enabled).toBe(false);
  });

  it('does not record an edit that changes nothing', () => {
    const start: AppConfigHistory = createAppConfigHistory(DEFAULT_ADMIN_APP_CONFIG);

    const same: AppConfigHistory = appConfigHistoryReducer(start, {
      page: 'users',
      type: 'setVisibility',
      visibility: 'ALL',
    });

    expect(same).toBe(start);
  });

  it('drops the redo stack when a new edit is made after undoing', () => {
    let history: AppConfigHistory = createAppConfigHistory(DEFAULT_ADMIN_APP_CONFIG);
    history = appConfigHistoryReducer(history, {layout: 'SIDE_BAR', type: 'setLayout'});
    history = appConfigHistoryReducer(history, {type: 'undo'});
    history = appConfigHistoryReducer(history, {type: 'toggleSecuredBy'});

    expect(history.future).toEqual([]);
    expect(history.present.navigation.layout).toBe('TOP_BAR');
    expect(history.present.branding.showSecuredBy).toBe(false);
  });

  it('toggles a page action and treats a missing action as off', () => {
    let history: AppConfigHistory = createAppConfigHistory(DEFAULT_ADMIN_APP_CONFIG);
    history = appConfigHistoryReducer(history, {action: 'delete', page: 'users', type: 'toggleAction'});

    expect(page(history, 'users').actions['delete']).toBe(true);
  });

  it('starts over from a saved configuration on reset', () => {
    let history: AppConfigHistory = createAppConfigHistory(DEFAULT_ADMIN_APP_CONFIG);
    history = appConfigHistoryReducer(history, {themeId: 'theme-1', type: 'setGateTheme'});

    const saved = history.present;
    history = appConfigHistoryReducer(history, {config: saved, type: 'reset'});

    expect(history.past).toEqual([]);
    expect(hasUnsavedChanges(history)).toBe(false);
  });

  it('keeps the app theme and the Gate design apart, each undoable', () => {
    let history: AppConfigHistory = createAppConfigHistory(DEFAULT_ADMIN_APP_CONFIG);
    history = appConfigHistoryReducer(history, {appTheme: {borderRadius: 12}, type: 'setAppTheme'});
    history = appConfigHistoryReducer(history, {layoutId: 'layout-1', type: 'setGateLayout'});

    expect(history.present.appTheme).toEqual({borderRadius: 12});
    expect(history.present.gate).toEqual({layoutId: 'layout-1'});

    history = appConfigHistoryReducer(history, {type: 'undo'});

    expect(history.present.gate?.layoutId).toBeUndefined();
    expect(history.present.appTheme).toEqual({borderRadius: 12});
  });
});
