// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {DEFAULT_AGENT_TYPE_NAME} from '../agent';

describe('agent model', () => {
  describe('DEFAULT_AGENT_TYPE_NAME', () => {
    it('should be "default"', () => {
      expect(DEFAULT_AGENT_TYPE_NAME).toBe('default');
    });

    it('should be a non-empty string', () => {
      expect(typeof DEFAULT_AGENT_TYPE_NAME).toBe('string');
      expect(DEFAULT_AGENT_TYPE_NAME.length).toBeGreaterThan(0);
    });
  });
});
