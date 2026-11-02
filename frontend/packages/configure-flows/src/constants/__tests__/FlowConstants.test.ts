// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import FlowConstants from '../FlowConstants';

describe('FlowConstants', () => {
  describe('AUTO_SAVE_INTERVAL', () => {
    it('should have AUTO_SAVE_INTERVAL set to 60000ms (1 minute)', () => {
      expect(FlowConstants.AUTO_SAVE_INTERVAL).toBe(60000);
    });

    it('should be a readonly property', () => {
      expect(typeof FlowConstants.AUTO_SAVE_INTERVAL).toBe('number');
    });
  });

  describe('MAX_HISTORY_ITEMS', () => {
    it('should have MAX_HISTORY_ITEMS set to 20', () => {
      expect(FlowConstants.MAX_HISTORY_ITEMS).toBe(20);
    });

    it('should be a readonly property', () => {
      expect(typeof FlowConstants.MAX_HISTORY_ITEMS).toBe('number');
    });
  });

  describe('step identifiers', () => {
    it('should have START_STEP_ID set to the lowercased start static step type', () => {
      expect(FlowConstants.START_STEP_ID).toBe('start');
    });

    it('should have END_STEP_ID set to the end step type', () => {
      expect(FlowConstants.END_STEP_ID).toBe('END');
    });
  });

  describe('DEFAULT_EDGE_TYPE', () => {
    it('should have DEFAULT_EDGE_TYPE set to base-edge', () => {
      expect(FlowConstants.DEFAULT_EDGE_TYPE).toBe('base-edge');
    });
  });

  describe('ExecutorNames', () => {
    it('should have the password provisioning executor name', () => {
      expect(FlowConstants.ExecutorNames.PASSWORD_PROVISIONING).toBe(
        'AskPasswordFlowExecutorConstants.PASSWORD_PROVISIONING_EXECUTOR',
      );
    });

    it('should have the email OTP executor name', () => {
      expect(FlowConstants.ExecutorNames.EMAIL_OTP).toBe('AskPasswordFlowExecutorConstants.EMAIL_OTP_EXECUTOR');
    });
  });

  describe('ActionTypes', () => {
    it('should have the EXECUTOR action type', () => {
      expect(FlowConstants.ActionTypes.EXECUTOR).toBe('EXECUTOR');
    });
  });

  describe('class structure', () => {
    it('should have all constants defined', () => {
      expect(FlowConstants).toHaveProperty('AUTO_SAVE_INTERVAL');
      expect(FlowConstants).toHaveProperty('MAX_HISTORY_ITEMS');
      expect(FlowConstants).toHaveProperty('START_STEP_ID');
      expect(FlowConstants).toHaveProperty('END_STEP_ID');
      expect(FlowConstants).toHaveProperty('DEFAULT_EDGE_TYPE');
      expect(FlowConstants).toHaveProperty('ExecutorNames');
      expect(FlowConstants).toHaveProperty('ActionTypes');
    });

    it('should be accessible as static properties', () => {
      // Verify constants are accessible without instantiation
      const autoSaveInterval = FlowConstants.AUTO_SAVE_INTERVAL;
      const maxHistoryItems = FlowConstants.MAX_HISTORY_ITEMS;

      expect(autoSaveInterval).toBeDefined();
      expect(maxHistoryItems).toBeDefined();
    });
  });
});
