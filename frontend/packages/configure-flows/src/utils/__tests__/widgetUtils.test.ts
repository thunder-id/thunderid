// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {Widget} from '../../models/widget';
import {widgetNeedsViewContainer} from '../widgetUtils';

const createMockWidget = (steps?: {__generationMeta__?: {strategy?: string}}[]): Widget =>
  ({
    id: 'widget-1',
    category: 'COMPOSITE',
    type: 'IDENTIFIER_PASSWORD',
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    resourceType: 'WIDGET',
    display: {
      label: 'Widget',
      image: '',
      showOnResourcePanel: true,
    },
    config: {
      field: {name: '', type: {}},
      styles: {},
      data: {
        steps,
      },
    },
  }) as Widget;

describe('widgetNeedsViewContainer', () => {
  it('returns true when a widget has no generated steps', () => {
    expect(widgetNeedsViewContainer(createMockWidget([]))).toBe(true);
  });

  it('returns true when any step uses the merge-with-drop-point strategy', () => {
    expect(
      widgetNeedsViewContainer(
        createMockWidget([
          {__generationMeta__: {strategy: 'MERGE_WITH_DROP_POINT'}},
          {__generationMeta__: {strategy: 'APPEND'}},
        ]),
      ),
    ).toBe(true);
  });

  it('returns false when all steps omit the merge-with-drop-point strategy', () => {
    expect(
      widgetNeedsViewContainer(
        createMockWidget([{__generationMeta__: {strategy: 'APPEND'}}, {__generationMeta__: {strategy: 'INLINE'}}]),
      ),
    ).toBe(false);
  });

  it('returns false when all steps are missing generation metadata', () => {
    expect(widgetNeedsViewContainer(createMockWidget([{}, {}]))).toBe(false);
  });
});
