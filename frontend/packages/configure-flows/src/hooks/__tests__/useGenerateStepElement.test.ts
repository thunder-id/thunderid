// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-member-access */

import {renderHook} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Element} from '../../models/elements';
import useGenerateStepElement from '../useGenerateStepElement';

// Mock generateResourceId
vi.mock('../../utils/generateResourceId', () => ({
  default: (prefix: string) => `${prefix}-generated-id`,
}));

describe('useGenerateStepElement', () => {
  describe('Hook Interface', () => {
    it('should return generateStepElement function', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      expect(typeof result.current.generateStepElement).toBe('function');
    });
  });

  describe('generateStepElement', () => {
    it('should generate element with unique ID based on category', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'ACTION',
        category: 'ACTION',
        display: {label: 'Test Button', image: '', showOnResourcePanel: true},
      } as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect(generatedElement.id).toBe('action-generated-id');
    });

    it('should preserve original element properties', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'TEXT_INPUT',
        category: 'INPUT',
        display: {label: 'Email Input', image: '', showOnResourcePanel: true},
        config: {placeholder: 'Enter email'},
      } as unknown as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect(generatedElement.type).toBe('TEXT_INPUT');
      expect(generatedElement.category).toBe('INPUT');
      expect(generatedElement.display?.label).toBe('Email Input');
      expect((generatedElement as any).config?.placeholder).toBe('Enter email');
    });

    it('should convert category to lowercase for ID generation', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'BUTTON',
        category: 'ACTION',
        display: {label: 'Button', image: '', showOnResourcePanel: true},
      } as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect(generatedElement.id).toBe('action-generated-id');
    });

    it('should apply default variant when variants exist', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'BUTTON',
        category: 'ACTION',
        display: {label: 'Button', image: '', showOnResourcePanel: true, defaultVariant: 'primary'},
        variants: [
          {variant: 'primary', style: 'contained', color: 'primary'},
          {variant: 'secondary', style: 'outlined', color: 'secondary'},
        ] as unknown as Element[],
      } as unknown as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect((generatedElement as any).variant).toBe('primary');
      expect((generatedElement as any).style).toBe('contained');
      expect((generatedElement as any).color).toBe('primary');
    });

    it('should use first variant when no defaultVariant is specified', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'BUTTON',
        category: 'ACTION',
        display: {label: 'Button', image: '', showOnResourcePanel: true},
        variants: [
          {variant: 'outlined', style: 'outlined'},
          {variant: 'contained', style: 'contained'},
        ] as unknown as Element[],
      } as unknown as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect((generatedElement as any).variant).toBe('outlined');
      expect((generatedElement as any).style).toBe('outlined');
    });

    it('should not modify element without variants', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'DIVIDER',
        category: 'LAYOUT',
        display: {label: 'Divider', image: '', showOnResourcePanel: true},
      } as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect(generatedElement.type).toBe('DIVIDER');
      expect((generatedElement as any).variants).toBeUndefined();
    });

    it('should handle empty variants array', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'TEXT',
        category: 'TYPOGRAPHY',
        display: {label: 'Text', image: '', showOnResourcePanel: true},
        variants: [] as unknown as Element[],
      } as unknown as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect(generatedElement.id).toBe('typography-generated-id');
      expect(generatedElement.type).toBe('TEXT');
    });

    it('should handle element with undefined variants property', () => {
      const {result} = renderHook(() => useGenerateStepElement());

      const element: Element = {
        id: 'original-id',
        type: 'IMAGE',
        category: 'MEDIA',
        display: {label: 'Image', image: '', showOnResourcePanel: true},
      } as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect(generatedElement.id).toBe('media-generated-id');
    });
  });

  describe('Function Behavior', () => {
    it('should work correctly after rerender', () => {
      const {result, rerender} = renderHook(() => useGenerateStepElement());

      rerender();

      const element: Element = {
        id: 'original-id',
        type: 'BUTTON',
        category: 'ACTION',
        display: {label: 'Button', image: '', showOnResourcePanel: true},
      } as Element;

      const generatedElement = result.current.generateStepElement(element);

      expect(generatedElement.id).toBe('action-generated-id');
    });
  });
});
