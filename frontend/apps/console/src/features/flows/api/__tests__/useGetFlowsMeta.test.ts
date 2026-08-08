// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import actions from '../../data/actions.json';
import elements from '../../data/elements.json';
import steps from '../../data/steps.json';
import rawTemplates from '../../data/templates.json';
import widgets from '../../data/widgets.json';
import type {FlowTemplate} from '../../models/templates';
import useGetFlowsMeta from '../useGetFlowsMeta';

const TEST_PRODUCT_NAME = 'TestProduct';

// Mock useConfig to avoid ConfigProvider requirement.
vi.mock('@thunderid/contexts', async (importOriginal) => {
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-type-assertion
  const actual = (await importOriginal()) as Record<string, unknown>;

  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: TEST_PRODUCT_NAME,
        },
      },
    }),
  };
});

describe('useGetFlowsMeta', () => {
  describe('Return Structure', () => {
    it('should return an object with data, error, and isLoading', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current).toHaveProperty('data');
      expect(result.current).toHaveProperty('error');
      expect(result.current).toHaveProperty('isLoading');
    });

    it('should return error as null', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.error).toBeNull();
    });

    it('should return isLoading as false', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.isLoading).toBe(false);
    });
  });

  describe('Data Content', () => {
    it('should return all templates when no flowType filter is provided', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.templates).toHaveLength((rawTemplates as FlowTemplate[]).length);
    });

    it('should substitute {{productName}} placeholders in templates with the configured product name', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      const serialised = JSON.stringify(result.current.data.templates);

      expect(serialised).not.toContain('{{productName}}');
      expect(serialised).toContain(TEST_PRODUCT_NAME);
    });

    it('should return actions from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.actions).toEqual(actions);
    });

    it('should return elements from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.elements).toEqual(elements);
    });

    it('should return steps from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.steps).toEqual(steps);
    });

    it('should return widgets from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.widgets).toEqual(widgets);
    });

    it('should include the required provisioning executor in registration blank template', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'REGISTRATION'}));

      const blankTemplate = result.current.data.templates.find((template) => template.type === 'BLANK');

      expect(blankTemplate?.config.nodes).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            id: 'start',
            onSuccess: 'user_type_resolver',
          }),
          expect.objectContaining({
            id: 'user_type_resolver',
            type: 'TASK_EXECUTION',
            executor: {name: 'UserTypeResolver'},
            onSuccess: 'view_prompt',
            onIncomplete: 'prompt_usertype',
          }),
          expect.objectContaining({
            id: 'provisioning',
            type: 'TASK_EXECUTION',
            executor: {name: 'ProvisioningExecutor'},
            onSuccess: 'END',
          }),
          expect.objectContaining({
            id: 'prompt_usertype',
            prompts: [
              expect.objectContaining({
                inputs: [
                  expect.objectContaining({
                    identifier: 'userType',
                    type: 'SELECT',
                    required: true,
                  }),
                ],
                action: {
                  ref: 'action_usertype',
                  nextNode: 'user_type_resolver',
                },
              }),
            ],
          }),
          expect.objectContaining({
            id: 'view_prompt',
            prompts: [
              expect.objectContaining({
                action: {
                  ref: 'action_continue',
                  nextNode: 'provisioning',
                },
              }),
            ],
          }),
        ]),
      );
    });

    it('should return an empty executors array', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.executors).toEqual([]);
    });
  });

  describe('Filtering by flowType', () => {
    it('should filter templates by AUTHENTICATION flowType', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'AUTHENTICATION'}));

      const templates = result.current.data.templates;
      expect(templates.length).toBeGreaterThan(0);
      expect(templates.every((t: FlowTemplate) => t.flowType === 'AUTHENTICATION')).toBe(true);
    });

    it('should filter templates by REGISTRATION flowType', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'REGISTRATION'}));

      const templates = result.current.data.templates;
      expect(templates.length).toBeGreaterThan(0);
      expect(templates.every((t: FlowTemplate) => t.flowType === 'REGISTRATION')).toBe(true);
    });

    it('should provide the secure user deletion flow template', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'ADMINISTRATION'}));

      const template = result.current.data.templates.find((item) => item.type === 'USER_DELETION');
      expect(template?.display.label).toBe('User Deletion Flow');
      expect(template?.config.nodes.map((node) => node.executor?.name).filter(Boolean)).toEqual([
        'PermissionValidator',
        'PreDeleteExecutor',
        'CriteriaRevocationExecutor',
        'SessionRevocationExecutor',
        'UserDeleteExecutor',
      ]);
    });

    it('should not affect non-template resources when filtering by flowType', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'AUTHENTICATION'}));

      expect(result.current.data.actions).toEqual(actions);
      expect(result.current.data.elements).toEqual(elements);
      expect(result.current.data.steps).toEqual(steps);
      expect(result.current.data.widgets).toEqual(widgets);
    });
  });

  describe('Memoization', () => {
    it('should return memoized data on subsequent renders', () => {
      const {result, rerender} = renderHook(() => useGetFlowsMeta());

      const initialData = result.current.data;
      rerender();

      expect(result.current.data).toBe(initialData);
    });

    it('should return new templates reference when flowType changes', () => {
      const {result, rerender} = renderHook<
        ReturnType<typeof useGetFlowsMeta>,
        {flowType: 'AUTHENTICATION' | 'REGISTRATION'}
      >(({flowType}) => useGetFlowsMeta({flowType}), {
        initialProps: {flowType: 'AUTHENTICATION'},
      });

      const firstTemplates = result.current.data.templates;
      rerender({flowType: 'REGISTRATION'});

      expect(result.current.data.templates).not.toBe(firstTemplates);
    });
  });
});
