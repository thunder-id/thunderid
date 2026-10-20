// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import elements from '../elements.json';

interface PaletteElement {
  category: string;
  display: {label: string; description: string; showOnResourcePanel?: boolean};
  label?: string;
  options?: unknown[];
  placeholder?: string;
  required?: boolean;
  type: string;
}

const find = (type: string): PaletteElement | undefined =>
  (elements as PaletteElement[]).find((element) => element.type === type);

describe('elements.json', () => {
  describe('USER_SELECT', () => {
    it('should be offered on the resource panel as a field next to SELECT', () => {
      const userSelect = find('USER_SELECT');

      expect(userSelect).toBeDefined();
      expect(userSelect?.category).toBe(find('SELECT')?.category);
      expect(userSelect?.display.showOnResourcePanel).not.toBe(false);
    });

    it('should not carry options, since the users are loaded when the flow runs', () => {
      expect(find('USER_SELECT')).not.toHaveProperty('options');
    });

    it('should be named and described so it cannot be mistaken for a SELECT', () => {
      const userSelect = find('USER_SELECT');
      const select = find('SELECT');

      expect(userSelect?.display.label).not.toBe(select?.display.label);
      expect(userSelect?.display.description).toMatch(/users/i);
      expect(userSelect?.label).not.toBe(select?.label);
    });

    it('should start with a label, a placeholder and the required setting like SELECT', () => {
      const userSelect = find('USER_SELECT');

      expect(userSelect?.label).toBeTruthy();
      expect(userSelect?.placeholder).toBeTruthy();
      expect(userSelect?.required).toBe(true);
    });
  });
});
