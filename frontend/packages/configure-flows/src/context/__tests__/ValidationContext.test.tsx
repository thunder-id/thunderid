// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {useContext} from 'react';
import {describe, it, expect} from 'vitest';
import {ValidationContext} from '../ValidationContext';

describe('ValidationContext', () => {
  describe('default context values', () => {
    it('should have isValid set to true by default', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.isValid).toBe(true);
    });

    it('should have empty notifications array by default', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.notifications).toEqual([]);
    });

    it('should have currentActiveTab set to 0 by default', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.currentActiveTab).toBe(0);
    });

    it('should have openValidationPanel set to false by default', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.openValidationPanel).toBe(false);
    });

    it('should have undefined optional functions by default', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.addNotification).toBeUndefined();
      expect(result.current.removeNotification).toBeUndefined();
      expect(result.current.setCurrentActiveTab).toBeUndefined();
      expect(result.current.setOpenValidationPanel).toBeUndefined();
    });

    it('should have undefined selectedNotification by default', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.selectedNotification).toBeUndefined();
    });

    it('should have default validationConfig with all options disabled', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.validationConfig).toEqual({
        isOTPValidationEnabled: false,
        isPasswordExecutorValidationEnabled: false,
        isRecoveryFactorValidationEnabled: false,
      });
    });

    it('should have getNotification function that returns undefined', () => {
      const {result} = renderHook(() => useContext(ValidationContext));

      expect(result.current.getNotification('any-id')).toBeUndefined();
    });
  });
});
